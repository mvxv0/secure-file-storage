package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
