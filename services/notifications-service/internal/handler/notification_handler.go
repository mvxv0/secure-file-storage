package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mvxv0/secure-file-storage/common/auth"
	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/service"
)

type NotificationHandler struct {
	notificationService *service.NotificationService
}

func NewNotificationHandler(notificationService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

// GetNotifications vraca sve notifikacije prijavljenog korisnika iz Cassandra baze
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo GET metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Korisnik nije autentifikovan")
		return
	}

	notifications, err := h.notificationService.GetUserNotifications(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrDatabaseNotConnected) {
			httputil.WriteError(w, http.StatusServiceUnavailable, "Cassandra baza za notifikacije trenutno nije dostupna")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri dobavljanju notifikacija: "+err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Notifikacije uspesno dobavljene", notifications)
}

// CreateNotification omogucava rucno ubacivanje notifikacije u Cassandru (POST /api/notifications)
func (h *NotificationHandler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Korisnik nije autentifikovan")
		return
	}

	var input service.CreateNotificationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	// samo administrator moze da prosledi tudji userId; redovan korisnik uvek kreira za sebe
	if input.UserID == "" || claims.Role != "A" {
		input.UserID = claims.UserID
	}

	notif, err := h.notificationService.CreateNotification(r.Context(), input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidNotificationInput) {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri kreiranju notifikacije: "+err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, "Notifikacija uspesno sacuvana u Cassandri", notif)
}

// SeedNotifications rucno popunjava Cassandra bazu demo notifikacijama za prijavljenog korisnika (POST /api/notifications/seed)
func (h *NotificationHandler) SeedNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Korisnik nije autentifikovan")
		return
	}

	notifications, err := h.notificationService.SeedForUser(r.Context(), claims.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri popunjavanju baze notifikacijama: "+err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, "Cassandra baza je uspesno popunjena notifikacijama", notifications)
}

// MarkAllAsRead oznacava sve notifikacije prijavljenog korisnika kao procitane (POST /api/notifications/read)
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Korisnik nije autentifikovan")
		return
	}

	if err := h.notificationService.MarkAllAsRead(r.Context(), claims.UserID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri oznacavanju notifikacija kao procitanih: "+err.Error())
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Sve notifikacije su oznacene kao procitane", nil)
}
