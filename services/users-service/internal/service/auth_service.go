package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	ErrPasswordTooYoung    = errors.New("lozinka mora biti stara minimalno 24h (1 dan) pre nego sto se moze ponovo promeniti")
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




// Proverava postojanje korisnika sa unetom email adresom
// Proverava da li je lozinka naloga minimalno 24h (1 dan) stara (s.cfg.MinPasswordAge)
// Generise kratkotrajni sigurnosni token (s.cfg.ResetTokenExpiration = 15 min) i salje link na email
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email adresa je obavezna")
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}

	// Kriterijum prihvatanja 
	// Da bi se lozinka promenila, mora biti bar 1 dan (24h) stara.
	if time.Since(user.PasswordChangedAt) < s.cfg.MinPasswordAge {
		preostalo := (s.cfg.MinPasswordAge - time.Since(user.PasswordChangedAt)).Round(time.Second)
		return fmt.Errorf("%w (preostalo jos %s)", ErrPasswordTooYoung, preostalo)
	}

	resetToken, err := generateSecureHexToken(16) 
	if err != nil {
		return fmt.Errorf("greska pri generisanju tokena za reset lozinke: %w", err)
	}

	// Ponistavamo prethodne neiskoriscene tokene za reset lozinke
	_ = s.repo.InvalidateExistingTokens(ctx, user.Email, model.TokenPasswordReset)

	tokenHash, err := bcrypt.GenerateFromPassword([]byte(resetToken), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("greska pri hesiranju reset tokena: %w", err)
	}

	now := time.Now().UTC()
	authToken := &model.AuthToken{
		Email:     user.Email,
		CodeHash:  string(tokenHash),
		Type:      model.TokenPasswordReset,
		ExpiresAt: now.Add(s.cfg.ResetTokenExpiration),
	}

	if err := s.repo.SaveAuthToken(ctx, authToken); err != nil {
		return fmt.Errorf("greska pri cuvanju tokena za reset lozinke: %w", err)
	}

	resetLink := fmt.Sprintf("%s/reset-password?email=%s&token=%s", strings.TrimRight(s.cfg.FrontendURL, "/"), user.Email, resetToken)

	subject := "Resetovanje zaboravljene lozinke"
	body := fmt.Sprintf(
		"Pozdrav %s,\n\nPrimili smo zahtev za promenu zaboravljene lozinke vaseg naloga.\n\n"+
			"Kratkotrajni link za resetovanje lozinke:\n%s\n\n"+
			"Kod / token za resetovanje: %s\n\n"+
			"Link i token vaze %d minuta (%s).\n",
		user.FirstName, resetLink, resetToken, int(s.cfg.ResetTokenExpiration.Minutes()), s.cfg.ResetTokenExpiration,
	)

	_ = s.mailer.SendEmail(user.Email, subject, body)
	return nil
}

type ResetPasswordInput struct {
	Email           string `json:"email"`
	Token           string `json:"token"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

// ResetPassword vrsi promenu lozinke koriscenjem kratkotrajnog tokena sa email-a:
func (s *AuthService) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	tokenStr := strings.TrimSpace(input.Token)

	if email == "" || tokenStr == "" || input.NewPassword == "" || input.ConfirmPassword == "" {
		return errors.New("email, token, nova lozinka i potvrda lozinke su obavezni")
	}

	if input.NewPassword != input.ConfirmPassword {
		return ErrPasswordMismatch
	}

	if err := ValidateStrongPassword(input.NewPassword); err != nil {
		return err
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}

	// Provera da li je lozinka minimalno 24h (1 dan) stara
	if time.Since(user.PasswordChangedAt) < s.cfg.MinPasswordAge {
		preostalo := (s.cfg.MinPasswordAge - time.Since(user.PasswordChangedAt)).Round(time.Second)
		return fmt.Errorf("%w (preostalo jos %s)", ErrPasswordTooYoung, preostalo)
	}

	tokenDoc, err := s.repo.FindValidToken(ctx, email, model.TokenPasswordReset)
	if err != nil {
		return ErrInvalidToken
	}

	if err := bcrypt.CompareHashAndPassword([]byte(tokenDoc.CodeHash), []byte(tokenStr)); err != nil {
		return ErrInvalidToken
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("greska pri hesiranju nove lozinke: %w", err)
	}

	_ = s.repo.MarkTokenUsed(ctx, tokenDoc.ID)

	if err := s.repo.UpdatePassword(ctx, email, string(hashedPassword)); err != nil {
		return err
	}

	_ = s.repo.ResetFailedLogins(ctx, email)
	return nil
}



// GetProfile vraca podatke o trenutno ulogovanom korisniku
func (s *AuthService) GetProfile(ctx context.Context, email string) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	return s.repo.FindByEmail(ctx, email)
}

// RequestAccountUpdate generise i salje verifikacioni kod na email ulogovanog korisnika.
func (s *AuthService) RequestAccountUpdate(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email je obavezan")
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}

	code, err := generateNumericCode(6)
	if err != nil {
		return fmt.Errorf("greska pri generisanju koda za izmenu naloga: %w", err)
	}

	_ = s.repo.InvalidateExistingTokens(ctx, user.Email, model.TokenAccountUpdate)

	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("greska pri hesiranju koda: %w", err)
	}

	now := time.Now().UTC()
	authToken := &model.AuthToken{
		Email:     user.Email,
		CodeHash:  string(codeHash),
		Type:      model.TokenAccountUpdate,
		ExpiresAt: now.Add(s.cfg.OTPExpiration),
	}

	if err := s.repo.SaveAuthToken(ctx, authToken); err != nil {
		return fmt.Errorf("greska pri cuvanju koda za izmenu naloga: %w", err)
	}

	subject := "Potvrda izmene podataka korisnickog naloga"
	body := fmt.Sprintf(
		"Pozdrav %s,\n\nZatrazili ste izmenu podataka vaseg korisnickog naloga.\n"+
			"Vas jednokratni kod za potvrdu izmene podataka je: %s\n\n"+
			"Kod je vazeci %d sekundi (%s).\n",
		user.FirstName, code, int(s.cfg.OTPExpiration.Seconds()), s.cfg.OTPExpiration,
	)

	_ = s.mailer.SendEmail(user.Email, subject, body)
	return nil
}

type UpdateAccountInput struct {
	Code            string `json:"code"`
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	NewPassword     string `json:"newPassword,omitempty"`
	ConfirmPassword string `json:"confirmPassword,omitempty"`
}

// UpdateAccount vrsi izmenu podataka ulogovanog korisnika (ime, prezime i opciono lozinku)
// uz obaveznu proveru koda poslatog na email naloga 
func (s *AuthService) UpdateAccount(ctx context.Context, email string, input UpdateAccountInput) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	code := strings.TrimSpace(input.Code)
	firstName := strings.TrimSpace(input.FirstName)
	lastName := strings.TrimSpace(input.LastName)

	if code == "" {
		return nil, errors.New("verifikacioni kod sa emaila je obavezan za izmenu podataka naloga")
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	// Ako ime ili prezime nisu prosledjeni u zahtevu, zadrzavamo postojece vrednosti
	if firstName == "" {
		firstName = user.FirstName
	}
	if lastName == "" {
		lastName = user.LastName
	}

	if len(firstName) < 2 || len(firstName) > 50 || len(lastName) < 2 || len(lastName) > 50 {
		return nil, ErrInvalidNameLength
	}

	// Ako korisnik menja i lozinku kroz izmenu naloga, proveravamo starost (min 24h) i jacinu lozinke
	wantsPasswordChange := strings.TrimSpace(input.NewPassword) != "" || strings.TrimSpace(input.ConfirmPassword) != ""
	var newPasswordHash string
	if wantsPasswordChange {
		if time.Since(user.PasswordChangedAt) < s.cfg.MinPasswordAge {
			preostalo := (s.cfg.MinPasswordAge - time.Since(user.PasswordChangedAt)).Round(time.Second)
			return nil, fmt.Errorf("%w (preostalo jos %s)", ErrPasswordTooYoung, preostalo)
		}
		if input.NewPassword != input.ConfirmPassword {
			return nil, ErrPasswordMismatch
		}
		if err := ValidateStrongPassword(input.NewPassword); err != nil {
			return nil, err
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("greska pri hesiranju nove lozinke: %w", err)
		}
		newPasswordHash = string(hashed)
	}

	// Provera koda sa emaila (dokaz da korisnik poseduje pristup emailu naloga)
	tokenDoc, err := s.repo.FindValidToken(ctx, email, model.TokenAccountUpdate)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if err := bcrypt.CompareHashAndPassword([]byte(tokenDoc.CodeHash), []byte(code)); err != nil {
		return nil, ErrInvalidToken
	}

	_ = s.repo.MarkTokenUsed(ctx, tokenDoc.ID)

	if wantsPasswordChange {
		if err := s.repo.UpdatePassword(ctx, email, newPasswordHash); err != nil {
			return nil, err
		}
	}

	return s.repo.UpdateUserProfile(ctx, email, firstName, lastName)
}

// ============================================================================
// 1.3 Povracaj naloga upotrebom magicnog linka (Magic Link)
// ============================================================================

// RequestMagicLink generise kratkotrajni magicni link za povracaj naloga i salje ga na email korisnika (zahtev 1.3)
func (s *AuthService) RequestMagicLink(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email adresa je obavezna")
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}

	if !user.IsVerified {
		return ErrUserNotVerified
	}

	magicToken, err := generateSecureHexToken(16) // 32 hex karaktera
	if err != nil {
		return fmt.Errorf("greska pri generisanju magicnog linka: %w", err)
	}

	_ = s.repo.InvalidateExistingTokens(ctx, user.Email, model.TokenMagicLink)

	tokenHash, err := bcrypt.GenerateFromPassword([]byte(magicToken), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("greska pri hesiranju magicnog tokena: %w", err)
	}

	now := time.Now().UTC()
	authToken := &model.AuthToken{
		Email:     user.Email,
		CodeHash:  string(tokenHash),
		Type:      model.TokenMagicLink,
		ExpiresAt: now.Add(s.cfg.ResetTokenExpiration),
	}

	if err := s.repo.SaveAuthToken(ctx, authToken); err != nil {
		return fmt.Errorf("greska pri cuvanju magicnog tokena: %w", err)
	}

	magicLink := fmt.Sprintf("%s/auth?magicEmail=%s&magicToken=%s", strings.TrimRight(s.cfg.FrontendURL, "/"), user.Email, magicToken)

	subject := "Magicni link za povracaj naloga (1.3)"
	body := fmt.Sprintf(
		"Pozdrav %s,\n\nZatrazili ste povracaj naloga i autentifikaciju putem magicnog linka.\n\n"+
			"Vas magicni link za prijavu:\n%s\n\n"+
			"Kod / token magicnog linka: %s\n\n"+
			"Link je jednokratan i vazi %d minuta (%s).\n",
		user.FirstName, magicLink, magicToken, int(s.cfg.ResetTokenExpiration.Minutes()), s.cfg.ResetTokenExpiration,
	)

	_ = s.mailer.SendEmail(user.Email, subject, body)
	return nil
}

// VerifyMagicLink proverava token magicnog linka, otkljucava nalog ako je bio zakljucan i izdaje JWT token (zahtev 1.3)
func (s *AuthService) VerifyMagicLink(ctx context.Context, email, tokenStr string) (*AuthResponseData, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	tokenStr = strings.TrimSpace(tokenStr)

	if email == "" || tokenStr == "" {
		return nil, errors.New("email i token magicnog linka su obavezni")
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !user.IsVerified {
		return nil, ErrUserNotVerified
	}

	tokenDoc, err := s.repo.FindValidToken(ctx, email, model.TokenMagicLink)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if err := bcrypt.CompareHashAndPassword([]byte(tokenDoc.CodeHash), []byte(tokenStr)); err != nil {
		return nil, ErrInvalidToken
	}

	_ = s.repo.MarkTokenUsed(ctx, tokenDoc.ID)
	_ = s.repo.ResetFailedLogins(ctx, email)

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

// generateSecureHexToken generise kriptografski siguran hex token za kratkotrajni link
func generateSecureHexToken(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
