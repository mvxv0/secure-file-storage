package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/repository"
)

// InternalHandler obradjuje interne HTTP zahteve koji dolaze od drugih mikroservisa
// (npr. files-service za sinhronu proveru i azuriranje kvote).
// Ove rute nisu izlozene na API Gateway-u i namenjene su iskljucivo komunikaciji unutar sistema.
type InternalHandler struct {
	userRepo *repository.UserRepository
}

func NewInternalHandler(userRepo *repository.UserRepository) *InternalHandler {
	return &InternalHandler{userRepo: userRepo}
}

type UserQuotaResponse struct {
	UserID     string `json:"userId"`
	QuotaLimit int64  `json:"quotaLimit"`
	QuotaUsed  int64  `json:"quotaUsed"`
}

type UpdateQuotaUsedRequest struct {
	QuotaUsed int64 `json:"quotaUsed"`
}

// GetUserQuota vraca maksimalnu kvotu (quotaLimit) i trenutno zauzece (quotaUsed) za zadatog korisnika.
// Ruta: GET /internal/users/{id}/quota
func (h *InternalHandler) GetUserQuota(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "ID korisnika je obavezan")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "Korisnik nije pronadjen")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri citanju kvote korisnika")
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Kvota korisnika uspesno dobavljena", UserQuotaResponse{
		UserID:     user.ID,
		QuotaLimit: user.QuotaLimit,
		QuotaUsed:  user.QuotaUsed,
	})
}

// UpdateUserQuotaUsed azurira trenutno zauzece skladista (quotaUsed) za zadatog korisnika.
// Ruta: PUT /internal/users/{id}/quota-used
func (h *InternalHandler) UpdateUserQuotaUsed(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "ID korisnika je obavezan")
		return
	}

	var req UpdateQuotaUsedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format zahteva")
		return
	}

	if req.QuotaUsed < 0 {
		httputil.WriteError(w, http.StatusBadRequest, "Zauzece skladista ne moze biti negativno")
		return
	}

	if err := h.userRepo.UpdateQuotaUsed(r.Context(), userID, req.QuotaUsed); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "Korisnik nije pronadjen")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri azuriranju zauzeca skladista")
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Zauzece skladista je uspesno azurirano", map[string]interface{}{
		"userId":    userID,
		"quotaUsed": req.QuotaUsed,
	})
}

