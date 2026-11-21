package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/model"
)

var ErrDatabaseNotConnected = errors.New("cassandra baza podataka trenutno nije povezana")

type NotificationRepository struct {
	session *gocql.Session
}

func NewNotificationRepository(session *gocql.Session) *NotificationRepository {
	return &NotificationRepository{session: session}
}

func (r *NotificationRepository) SetSession(session *gocql.Session) {
	r.session = session
}

// GetByUserID dobavlja sve notifikacije za datog korisnika iz Cassandre (sortirane po created_at DESC)
func (r *NotificationRepository) GetByUserID(ctx context.Context, userID string) ([]model.Notification, error) {
	if r.session == nil || r.session.Closed() {
		return nil, ErrDatabaseNotConnected
	}

	query := `
		SELECT user_id, created_at, id, type, resource_id, resource_name, message, is_read
		FROM notifications_by_user
		WHERE user_id = ?`

	iter := r.session.Query(query, userID).WithContext(ctx).Iter()

	notifications := make([]model.Notification, 0)
	var n model.Notification
	for iter.Scan(&n.UserID, &n.CreatedAt, &n.ID, &n.Type, &n.ResourceID, &n.ResourceName, &n.Message, &n.IsRead) {
		notifications = append(notifications, n)
		n = model.Notification{}
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("greska pri citanju notifikacija iz Cassandre: %w", err)
	}

	return notifications, nil
}

// Create upisuje novu notifikaciju u Cassandra tabelu notifications_by_user
func (r *NotificationRepository) Create(ctx context.Context, n *model.Notification) error {
	if r.session == nil || r.session.Closed() {
		return ErrDatabaseNotConnected
	}

	if n.ID == "" {
		n.ID = gocql.TimeUUID().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO notifications_by_user (user_id, created_at, id, type, resource_id, resource_name, message, is_read)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	return r.session.Query(
		query,
		n.UserID,
		n.CreatedAt,
		n.ID,
		n.Type,
		n.ResourceID,
		n.ResourceName,
		n.Message,
		n.IsRead,
	).WithContext(ctx).Exec()
}

// SeedInitialNotifications popunjava Cassandra bazu inicijalnim notifikacijama za korisnika
func (r *NotificationRepository) SeedInitialNotifications(ctx context.Context, userID string) ([]model.Notification, error) {
	if r.session == nil || r.session.Closed() {
		return nil, ErrDatabaseNotConnected
	}

	now := time.Now().UTC()
	initial := []model.Notification{
		{
			UserID:       userID,
			CreatedAt:    now.Add(-2 * time.Hour),
			ID:           "notif-seed-1",
			Type:         "FOLDER_SHARED",
			ResourceID:   "folder-soa-2026",
			ResourceName: "Projekat SOA i NoSQL",
			Message:      "Korisnik Miloš Maksimović je podelio folder 'Projekat SOA i NoSQL' sa vama.",
			IsRead:       false,
		},
		{
			UserID:       userID,
			CreatedAt:    now.Add(-45 * time.Minute),
			ID:           "notif-seed-2",
			Type:         "FILE_ADDED_TO_SHARED_FOLDER",
			ResourceID:   "file-specifikacija-pdf",
			ResourceName: "Specifikacija_Projekta_2026.pdf",
			Message:      "U deljeni folder 'Projekat SOA i NoSQL' je dodata nova datoteka 'Specifikacija_Projekta_2026.pdf'.",
			IsRead:       false,
		},
		{
			UserID:       userID,
			CreatedAt:    now.Add(-15 * time.Minute),
			ID:           "notif-seed-3",
			Type:         "FILE_SHARED",
			ResourceID:   "file-arhitektura-png",
			ResourceName: "Dijagram_Arhitekture.png",
			Message:      "Korisnik Marko Petrović je podelio datoteku 'Dijagram_Arhitekture.png' sa grupom 'Tim 3'.",
			IsRead:       false,
		},
		{
			UserID:       userID,
			CreatedAt:    now.Add(-5 * time.Minute),
			ID:           "notif-seed-4",
			Type:         "NEW_VERSION",
			ResourceID:   "file-specifikacija-pdf",
			ResourceName: "Specifikacija_Projekta_2026.pdf",
			Message:      "Otpremljena je nova verzija (v2) deljene datoteke 'Specifikacija_Projekta_2026.pdf'.",
			IsRead:       true,
		},
	}

	for i := range initial {
		if err := r.Create(ctx, &initial[i]); err != nil {
			return nil, err
		}
	}

	return r.GetByUserID(ctx, userID)
}

// MarkAllAsRead oznacava sve neprocitane notifikacije korisnika kao procitane (is_read = true) u Cassandri
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	if r.session == nil || r.session.Closed() {
		return ErrDatabaseNotConnected
	}

	notifications, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	updateQuery := `
		UPDATE notifications_by_user
		SET is_read = true
		WHERE user_id = ? AND created_at = ? AND id = ?`

	for _, n := range notifications {
		if !n.IsRead {
			if err := r.session.Query(updateQuery, n.UserID, n.CreatedAt, n.ID).WithContext(ctx).Exec(); err != nil {
				return fmt.Errorf("greska pri azuriranju is_read u Cassandri: %w", err)
			}
		}
	}

	return nil
}
