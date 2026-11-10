package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mvxv0/secure-file-storage/services/users-service/internal/config"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/mailer"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/model"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserAlreadyVerified = errors.New("nalog je vec potvrdjen")
	ErrInvalidToken        = errors.New("neispravan ili istekao verifikacioni kod")
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

