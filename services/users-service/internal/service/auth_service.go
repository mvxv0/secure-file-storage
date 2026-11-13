package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mvxv0/secure-file-storage/common/auth"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/config"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/mailer"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/model"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserAlreadyVerified = errors.New("nalog je vec potvrdjen")
	ErrUserNotVerified     = errors.New("nalog nije potvrdjen. Molimo vas da prvo potvrdite email")
	ErrInvalidCredentials  = errors.New("neispravna email adresa ili lozinka")
	ErrAccountLocked       = errors.New("nalog je privremeno zakljucan zbog vise uzastopnih neuspelih pokusaja prijave")
	ErrPasswordExpired     = errors.New("vasa lozinka je istekla (starija od 60 dana). Prijava je onemogucena dok ne promenite lozinku")
	ErrInvalidToken        = errors.New("neispravan ili istekao kod")
)

type AuthService struct {
	repo   *repository.UserRepository
	mailer *mailer.Mailer
	cfg    *config.Config
}

func NewAuthService(repo *repository.UserRepository, mailer *mailer.Mailer, cfg *config.Config) *AuthService {
	return &AuthService{
		repo:   repo,
		mailer: mailer,
		cfg:    cfg,
	}
}

type RegisterInput struct {
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// Register vrsi kompletnu registraciju novog korisnika:
// 1. Provera ulaznih podataka (ime, prezime, email, jaka lozinka)
// 2. Provera jedinstvenosti email adrese
// 3. Bcrypt hesiranje sa solju
// 4. Dodeljivanje podrazumevane kvote i uloge
// 5. Kreiranje verifikacionog koda i slanje na email preko MailHog-a
func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*model.User, error) {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	// validacija unetih podataka
	if err := ValidateRegistration(input.FirstName, input.LastName, input.Email, input.Password, input.ConfirmPassword); err != nil {
		return nil, err
	}

	// provera da li email vec postoji u bazi
	existing, _ := s.repo.FindByEmail(ctx, input.Email)
	if existing != nil {
		return nil, repository.ErrUserAlreadyExists
	}

	// hesiranje lozinke koristeci bcrypt (automatski dodaje nasumicnu so / salt)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("greska pri hesiranju lozinke: %w", err)
	}

	now := time.Now().UTC()
	newUser := &model.User{
		FirstName:         input.FirstName,
		LastName:          input.LastName,
		Email:             input.Email,
		PasswordHash:      string(hashedPassword),
		Role:              model.RoleRegularUser,
		IsVerified:        false,                     // nalog je neaktivan dok se ne potvrdi mejl
		QuotaLimit:        s.cfg.DefaultStorageQuota, // podrazumevana kvota dodeljena korisniku
		QuotaUsed:         0,
		PasswordChangedAt: now, // pocetni datum za pracenje periodicne promene lozinke
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.repo.CreateUser(ctx, newUser); err != nil {
		return nil, err
	}

	// generisanje 6-cifrenog verifikacionog koda za potvrdu registracije
	verificationCode, err := generateNumericCode(6)
	if err != nil {
		return nil, fmt.Errorf("greska pri generisanju koda: %w", err)
	}

	// cuvamo hes koda u bazi radi bezbednosti (traje 15 minuta)
	tokenHash, _ := bcrypt.GenerateFromPassword([]byte(verificationCode), bcrypt.DefaultCost)
	authToken := &model.AuthToken{
		Email:     newUser.Email,
		CodeHash:  string(tokenHash),
		Type:      model.TokenVerification,
		ExpiresAt: now.Add(15 * time.Minute),
	}

	if err := s.repo.SaveAuthToken(ctx, authToken); err != nil {
		return nil, fmt.Errorf("greska pri cuvanju verifikacionog tokena: %w", err)
	}

	// slanje mejla sa kodom korisniku
	subject := "Potvrda registracije naloga"
	body := fmt.Sprintf(
		"Pozdrav %s,\n\nUspesno ste zapoceli registraciju na platformi za skladistenje fajlova.\n"+
			"Vas jednokratni kod za potvrdu naloga je: %s\n\n"+
			"Kod je vazeci 15 minuta.\n",
		newUser.FirstName, verificationCode,
	)

	_ = s.mailer.SendEmail(newUser.Email, subject, body)

	return newUser, nil
}

// VerifyEmail potvrdjuje registraciju korisnika na osnovu unetog koda
func (s *AuthService) VerifyEmail(ctx context.Context, email, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)

	if email == "" || code == "" {
		return errors.New("email i verifikacioni kod su obavezni")
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}

	if user.IsVerified {
		return ErrUserAlreadyVerified
	}

	token, err := s.repo.FindValidToken(ctx, email, model.TokenVerification)
	if err != nil {
		return ErrInvalidToken
	}

	// provera da li uneti kod odgovara sacuvanom hesu
	if err := bcrypt.CompareHashAndPassword([]byte(token.CodeHash), []byte(code)); err != nil {
		return ErrInvalidToken
	}

	// oznacavamo token kao iskoriscen i aktiviramo korisnika
	_ = s.repo.MarkTokenUsed(ctx, token.ID)
	return s.repo.SetUserVerified(ctx, email)
}

// Login vrsi 1. KORAK prijave (lozinka + provere + slanje OTP-a):
// 1. Provera postojanja naloga
// 2. Provera da li je nalog privremeno zakljucan (brute-force zastita)
// 3. Provera lozinke (bcrypt)
// 4. Provera da li je nalog verifikovan (is_verified)
// 5. Provera auditabilnosti: da li je lozinka starija od maksimalnog roka (60 dana)
// 6. Generisanje i slanje 6-cifrenog OTP koda na mejl
func (s *AuthService) Login(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		return errors.New("email i lozinka su obavezni")
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		// radi bezbednosti (user enumeration protection), vracamo genericnu poruku
		return ErrInvalidCredentials
	}

	// 1. Provera da li je nalog zakljucan zbog neuspelih pokusaja
	now := time.Now().UTC()
	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		preostalo := time.Until(*user.LockedUntil).Round(time.Second)
		return fmt.Errorf("%w. Pokusajte ponovo za %s (do %s)", ErrAccountLocked, preostalo, user.LockedUntil.Format("15:04:05"))
	}

	// 2. Provera lozinke
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// lozinka nije tacna -> povecavamo brojac neuspeha i potencijalno zakljucavamo
		lockedUntil, _ := s.repo.IncrementFailedLogins(ctx, email, user.FailedLoginAttempts, s.cfg.AccountLockDuration)
		if lockedUntil != nil {
			return fmt.Errorf("%w. Nalog je zakljucan na %s", ErrAccountLocked, s.cfg.AccountLockDuration)
		}
		return ErrInvalidCredentials
	}

	// lozinka je tacna -> resetujemo brojac neuspelih pokusaja
	_ = s.repo.ResetFailedLogins(ctx, email)

	// 3. Provera da li je potvrdjen nalog
	if !user.IsVerified {
		return ErrUserNotVerified
	}

	// 4. Provera auditabilnosti: istek roka vazenja lozinke (maksimalno 60 dana)
	if time.Since(user.PasswordChangedAt) > s.cfg.MaxPasswordAge {
		return ErrPasswordExpired
	}

	// 5. Generisanje 6-cifrenog jednokratnog koda (OTP) za 2FA
	otpCode, err := generateNumericCode(6)
	if err != nil {
		return fmt.Errorf("greska pri generisanju OTP koda: %w", err)
	}

	// ponistavamo sve prethodne neiskoriscene OTP kodove za ovog korisnika
	_ = s.repo.InvalidateExistingTokens(ctx, user.Email, model.TokenOTP)

	// cuvamo hesovan OTP kod u bazi
	codeHash, _ := bcrypt.GenerateFromPassword([]byte(otpCode), bcrypt.DefaultCost)
	authToken := &model.AuthToken{
		Email:     user.Email,
		CodeHash:  string(codeHash),
		Type:      model.TokenOTP,
		ExpiresAt: now.Add(s.cfg.OTPExpiration), // npr. 5 minuta (300 sekundi)
	}

	if err := s.repo.SaveAuthToken(ctx, authToken); err != nil {
		return fmt.Errorf("greska pri cuvanju OTP koda: %w", err)
	}

	// slanje OTP koda preko MailHog-a
	subject := "Vas jednokratni kod za prijavu (OTP)"
	body := fmt.Sprintf(
		"Pozdrav %s,\n\nVas jednokratni kod za prijavu na platformu je: %s\n\n"+
			"Kod je vazeci %d sekundi (%s).\n"+
			"Ukoliko niste vi zatrazili prijavu, preporucujemo da promenite lozinku.",
		user.FirstName, otpCode, int(s.cfg.OTPExpiration.Seconds()), s.cfg.OTPExpiration,
	)

	_ = s.mailer.SendEmail(user.Email, subject, body)

	return nil
}

type AuthResponseData struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// VerifyLoginOTP vrsi 2. KORAK prijave (provera OTP koda i izdavanje JWT tokena)
func (s *AuthService) VerifyLoginOTP(ctx context.Context, email, code string) (*AuthResponseData, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)

	if email == "" || code == "" {
		return nil, errors.New("email i OTP kod su obavezni")
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// trazenje vazeceg OTP koda
	token, err := s.repo.FindValidToken(ctx, email, model.TokenOTP)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// poređenje unetog koda sa hešom u bazi
	if err := bcrypt.CompareHashAndPassword([]byte(token.CodeHash), []byte(code)); err != nil {
		return nil, ErrInvalidToken
	}

	// oznacavamo OTP token kao iskoriscen da se ne moze ponovo iskoristiti
	_ = s.repo.MarkTokenUsed(ctx, token.ID)

	// generisanje JWT tokena (potpisan tajnim kljucem iz konfiguracije)
	jwtToken, err := auth.GenerateToken(
		s.cfg.JWTSecret,
		user.ID,
		user.Email,
		string(user.Role),
		s.cfg.JWTExpiration,
	)
	if err != nil {
		return nil, fmt.Errorf("greska pri generisanju JWT tokena: %w", err)
	}

	return &AuthResponseData{
		Token: jwtToken,
		User:  user,
	}, nil
}

// Logout stavlja JWT token na crnu listu tako da vise ne moze da se koristi
func (s *AuthService) Logout(ctx context.Context, tokenStr string) error {
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return errors.New("token je obavezan za odjavu")
	}

	// validiramo token da bismo izvukli vreme isteka (exp)
	claims, err := auth.ValidateToken(tokenStr, s.cfg.JWTSecret)
	if err != nil {
		// ako je token vec neispravan ili istekao, smatramo da je odjavljen
		return nil
	}

	expiresAt := time.Unix(claims.Expire, 0).UTC()
	return s.repo.RevokeToken(ctx, tokenStr, expiresAt)
}

// IsTokenBlacklisted proverava da li je token na crnoj listi (za middleware)
func (s *AuthService) IsTokenBlacklisted(ctx context.Context, tokenStr string) bool {
	return s.repo.IsTokenRevoked(ctx, tokenStr)
}

// generateNumericCode generise siguran nasumicni niz cifara date duzine
func generateNumericCode(length int) (string, error) {
	code := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code[i] = byte('0' + num.Int64())
	}
	return string(code), nil
}
