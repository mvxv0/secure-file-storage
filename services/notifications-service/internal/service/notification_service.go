package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/model"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/repository"
)

var ErrInvalidNotificationInput = errors.New("poruka i tip notifikacije su obavezni")

type NotificationService struct {
	repo     *repository.NotificationRepository
	autoSeed bool
}

func NewNotificationService(repo *repository.NotificationRepository, autoSeed bool) *NotificationService {
	return &NotificationService{
		repo:     repo,
		autoSeed: autoSeed,
	}
}

// GetUserNotifications vraca sve notifikacije korisnika iz Cassandre.
// ukoliko u Cassandri jos nema podataka za prijavljenog korisnika,
// baza se automatski popunjava inicijalnim notifikacijama kako bi bile vidljive u klijentskoj aplikaciji.
func (s *NotificationService) GetUserNotifications(ctx context.Context, userID string) ([]model.Notification, error) {
	notifications, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(notifications) == 0 && s.autoSeed {
		return s.repo.SeedInitialNotifications(ctx, userID)
	}

	return notifications, nil
}

type CreateNotificationInput struct {
	UserID       string `json:"userId"`
	Type         string `json:"type"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName"`
	Message      string `json:"message"`
}

// CreateNotification omogucava rucno dodavanje notifikacije u Cassandra bazu
func (s *NotificationService) CreateNotification(ctx context.Context, input CreateNotificationInput) (*model.Notification, error) {
	input.Message = strings.TrimSpace(input.Message)
	input.Type = strings.TrimSpace(input.Type)
	if input.Message == "" {
		return nil, ErrInvalidNotificationInput
	}
	if input.Type == "" {
		input.Type = "FILE_SHARED"
	}

	notif := &model.Notification{
		UserID:       strings.TrimSpace(input.UserID),
		CreatedAt:    time.Now().UTC(),
		Type:         input.Type,
		ResourceID:   strings.TrimSpace(input.ResourceID),
		ResourceName: strings.TrimSpace(input.ResourceName),
		Message:      input.Message,
		IsRead:       false,
	}

	if err := s.repo.Create(ctx, notif); err != nil {
		return nil, err
	}
	return notif, nil
}

// SeedForUser eksplicitno popunjava bazu test notifikacijama za datog korisnika
func (s *NotificationService) SeedForUser(ctx context.Context, userID string) ([]model.Notification, error) {
	return s.repo.SeedInitialNotifications(ctx, userID)
}

// MarkAllAsRead oznacava sve notifikacije korisnika kao procitane
func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}
