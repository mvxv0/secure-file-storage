package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mvxv0/secure-file-storage/common/auth"
	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/model"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/service"
)

type QuotaHandler struct {
	quotaService *service.QuotaService
}

func NewQuotaHandler(quotaService *service.QuotaService) *QuotaHandler {
	return &QuotaHandler{quotaService: quotaService}
}

type SubmitQuotaRequestInput struct {
	RequestedQuota int64  `json:"requestedQuota"`
	Reason         string `json:"reason"`
}

func requireAdmin(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := auth.GetClaims(r)
	if !ok || claims == nil || claims.UserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return nil, false
	}
	if claims.Role != string(model.RoleAdmin) {
		httputil.WriteError(w, http.StatusForbidden, "Pristup je dozvoljen samo administratoru")
		return nil, false
	}
	return claims, true
}

// SubmitQuotaRequest omogucava korisniku podnosenje zahteva za povecanje kvote (POST /api/users/quota-requests)
func (h *QuotaHandler) SubmitQuotaRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok || claims == nil || claims.UserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return
	}

	var input SubmitQuotaRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	req, err := h.quotaService.SubmitRequest(r.Context(), claims.UserID, input.RequestedQuota, input.Reason)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequestedQuota),
			errors.Is(err, service.ErrQuotaLimitTooHigh):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrPendingRequestExists):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		case errors.Is(err, repository.ErrUserNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri podnosenju zahteva za kvotu: "+err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, "Zahtev za povecanje kvote je uspesno podnet", req)
}

// GetMyQuotaRequests vraca sve zahteve ulogovanog korisnika (GET /api/users/quota-requests/me)
func (h *QuotaHandler) GetMyQuotaRequests(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok || claims == nil || claims.UserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return
	}

	requests, err := h.quotaService.GetMyRequests(r.Context(), claims.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri dobavljanju zahteva za kvotu: "+err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Zahtevi za povecanje kvote uspesno dobavljeni", requests)
}

// GetAllUsers vraca sve korisnike sa trenutnom kvotom i zauzecem (GET /api/users - samo Admin)
func (h *QuotaHandler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	users, err := h.quotaService.GetAllUsers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri dobavljanju korisnika: "+err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Korisnici uspesno dobavljeni", users)
}

// GetAllQuotaRequests vraca sve pristigle zahteve za povecanje kvote (GET /api/users/quota-requests - samo Admin)
func (h *QuotaHandler) GetAllQuotaRequests(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	requests, err := h.quotaService.GetAllRequests(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri dobavljanju zahteva za kvotu: "+err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Svi zahtevi za povecanje kvote uspesno dobavljeni", requests)
}

// ApproveQuotaRequest odobrava zahtev i povecava kvotu korisnika (POST /api/users/quota-requests/{id}/approve - samo Admin)
func (h *QuotaHandler) ApproveQuotaRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdmin(w, r)
	if !ok {
		return
	}

	requestID := strings.TrimSpace(r.PathValue("id"))
	if requestID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "ID zahteva je obavezan")
		return
	}

	updated, err := h.quotaService.ApproveRequest(r.Context(), claims.UserID, requestID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrQuotaRequestNotFound),
			errors.Is(err, repository.ErrUserNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrRequestAlreadyHandled):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri odobravanju zahteva: "+err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Zahtev je odobren i kvota korisnika je azurirana", updated)
}

// RejectQuotaRequest odbija zahtev bez izmene kvote (POST /api/users/quota-requests/{id}/reject - samo Admin)
func (h *QuotaHandler) RejectQuotaRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdmin(w, r)
	if !ok {
		return
	}

	requestID := strings.TrimSpace(r.PathValue("id"))
	if requestID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "ID zahteva je obavezan")
		return
	}

	updated, err := h.quotaService.RejectRequest(r.Context(), claims.UserID, requestID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrQuotaRequestNotFound):
			httputil.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrRequestAlreadyHandled):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri odbijanju zahteva: "+err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Zahtev za povecanje kvote je odbijen", updated)
}

