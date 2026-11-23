package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mvxv0/secure-file-storage/common/auth"
	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register prima podatke za registraciju: ime, prezime, email, lozinka, ponovljena lozinka
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	var input service.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	user, err := h.authService.Register(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmptyFields),
			errors.Is(err, service.ErrInvalidEmail),
			errors.Is(err, service.ErrPasswordMismatch),
			errors.Is(err, service.ErrWeakPassword),
			errors.Is(err, service.ErrInvalidNameLength):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, repository.ErrUserAlreadyExists):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri registraciji korisnika: "+err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, "Registracija uspesna! Kod za potvrdu naloga je poslat na vasu email adresu.", map[string]interface{}{
		"id":         user.ID,
		"email":      user.Email,
		"firstName":  user.FirstName,
		"lastName":   user.LastName,
		"quotaLimit": user.QuotaLimit,
		"isVerified": user.IsVerified,
	})
}

type VerifyEmailInput struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// VerifyEmail potvrdjuje registraciju preko koda poslatog na mejl
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	var input VerifyEmailInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	err := h.authService.VerifyEmail(r.Context(), input.Email, input.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserAlreadyVerified):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrInvalidToken):
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, repository.ErrUserNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri potvrdi registracije: "+err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Nalog je uspesno potvrdjen! Mozete se prijaviti.", nil)
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login vrsi 1. korak prijave: provera lozinke i slanje 6-cifrenog OTP koda na mejl
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	var input LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	err := h.authService.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, service.ErrUserNotVerified):
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrPasswordExpired):
			// zahtev 1.2: auditabilnost naloga - zabrana prijave nakon isteka 60 dana
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrAccountLocked):
			// zastita od brute-force napada: nalog zakljucan
			httputil.WriteError(w, http.StatusLocked, err.Error())
		default:
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Lozinka ispravna! Jednokratni kod za prijavu (OTP) je poslat na vasu email adresu.", map[string]interface{}{
		"step":  "OTP_REQUIRED",
		"email": strings.ToLower(strings.TrimSpace(input.Email)),
	})
}

type VerifyOTPInput struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// VerifyOTP vrsi 2. korak prijave: unosi se 6-cifreni OTP kod sa mejla i vraca se JWT token
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	var input VerifyOTPInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	res, err := h.authService.VerifyLoginOTP(r.Context(), input.Email, input.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, service.ErrInvalidCredentials):
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		default:
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Uspesna prijava na sistem!", map[string]interface{}{
		"token": res.Token,
		"user": map[string]interface{}{
			"id":         res.User.ID,
			"email":      res.User.Email,
			"firstName":  res.User.FirstName,
			"lastName":   res.User.LastName,
			"role":       res.User.Role,
			"quotaLimit": res.User.QuotaLimit,
			"quotaUsed":  res.User.QuotaUsed,
		},
	})
}

// Logout odjavljuje korisnika stavljanjem JWT tokena na crnu listu
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		httputil.WriteError(w, http.StatusUnauthorized, "Nedostaje Authorization Bearer token")
		return
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	err := h.authService.Logout(r.Context(), tokenStr)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri odjavi: "+err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Uspesno ste se odjavili sa sistema.", nil)
}

type ForgotPasswordInput struct {
	Email string `json:"email"`
}

// ForgotPassword [US 02.05] prima email adresu, proverava da li je lozinka minimalno 24h stara
// i salje kratkotrajni link/token za resetovanje lozinke na email adresu naloga
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	var input ForgotPasswordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	err := h.authService.RequestPasswordReset(r.Context(), input.Email)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrPasswordTooYoung):
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		default:
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Kratkotrajni link i kod za promenu zaboravljene lozinke su poslati na vasu email adresu.", map[string]interface{}{
		"email": strings.ToLower(strings.TrimSpace(input.Email)),
	})
}

// ResetPassword [US 02.05] menja zaboravljenu lozinku uz pomoc kratkotrajnog tokena sa email-a
// i proveru da je trenutna lozinka stara minimalno 24h (1 dan)
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	var input service.ResetPasswordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	err := h.authService.ResetPassword(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrPasswordTooYoung):
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrInvalidToken):
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, service.ErrPasswordMismatch),
			errors.Is(err, service.ErrWeakPassword):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Lozinka je uspesno promenjena! Mozete se prijaviti sa novom lozinkom.", nil)
}

// GetProfile vraca podatke o ulogovanom korisniku
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo GET metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims.Email == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return
	}

	user, err := h.authService.GetProfile(r.Context(), claims.Email)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Podaci korisnickog naloga uspesno dobavljeni", user)
}

// RequestAccountUpdate [US 02.04] salje kod za potvrdu izmene podataka na email adresu ulogovanog korisnika
func (h *AuthHandler) RequestAccountUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims.Email == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Morate biti ulogovani da biste izmenili podatke naloga")
		return
	}

	err := h.authService.RequestAccountUpdate(r.Context(), claims.Email)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri slanju koda za izmenu naloga: "+err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Verifikacioni kod za potvrdu izmene podataka je poslat na vasu email adresu.", map[string]interface{}{
		"email": claims.Email,
	})
}

// UpdateAccount [US 02.04] menja podatke naloga ulogovanog korisnika nakon provere verifikacionog koda sa email-a
func (h *AuthHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je PUT ili POST metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims.Email == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Morate biti ulogovani da biste izmenili podatke naloga")
		return
	}

	var input service.UpdateAccountInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	updatedUser, err := h.authService.UpdateAccount(r.Context(), claims.Email, input)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrInvalidToken):
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, service.ErrPasswordTooYoung):
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrInvalidNameLength),
			errors.Is(err, service.ErrPasswordMismatch),
			errors.Is(err, service.ErrWeakPassword):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Podaci korisnickog naloga su uspesno izmenjeni!", updatedUser)
}

type MagicLinkRequestInput struct {
	Email string `json:"email"`
}

// RequestMagicLink salje magicni link za povracaj naloga na email adresu korisnika (zahtev 1.3)
func (h *AuthHandler) RequestMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	var input MagicLinkRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	err := h.authService.RequestMagicLink(r.Context(), input.Email)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrUserNotVerified):
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		default:
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Magicni link za povracaj naloga je poslat na vasu email adresu.", map[string]interface{}{
		"email": strings.ToLower(strings.TrimSpace(input.Email)),
	})
}

type VerifyMagicLinkInput struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

// VerifyMagicLink vrsi autentifikaciju korisnika preko magicnog linka i vraca JWT token (zahtev 1.3)
func (h *AuthHandler) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	var input VerifyMagicLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	res, err := h.authService.VerifyMagicLink(r.Context(), input.Email, input.Token)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken),
			errors.Is(err, service.ErrInvalidCredentials):
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, service.ErrUserNotVerified):
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		default:
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Uspesan povracaj naloga i prijava putem magicnog linka!", map[string]interface{}{
		"token": res.Token,
		"user": map[string]interface{}{
			"id":         res.User.ID,
			"email":      res.User.Email,
			"firstName":  res.User.FirstName,
			"lastName":   res.User.LastName,
			"role":       res.User.Role,
			"quotaLimit": res.User.QuotaLimit,
			"quotaUsed":  res.User.QuotaUsed,
		},
	})
}
