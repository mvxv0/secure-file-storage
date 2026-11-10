package handler

import (
	"encoding/json"
	"errors"
	"net/http"

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

