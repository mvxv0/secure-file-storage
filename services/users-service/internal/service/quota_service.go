package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mvxv0/secure-file-storage/services/users-service/internal/config"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/model"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

const MaxAllowedStorageQuota int64 = 10 * 1024 * 1024 * 1024 // 10 GB

var (
	ErrInvalidRequestedQuota = errors.New("trazena kvota mora biti veca od vase trenutne kvote")
	ErrQuotaLimitTooHigh     = errors.New("trazena kvota ne moze biti veca od 10 GB")
	ErrPendingRequestExists  = errors.New("vec imate aktivan zahtev za povecanje kvote na cekanju")
	ErrRequestAlreadyHandled = errors.New("ovaj zahtev za povecanje kvote je vec obradjen")
)

type QuotaService struct {
	userRepo  *repository.UserRepository
	quotaRepo *repository.QuotaRepository
}

func NewQuotaService(userRepo *repository.UserRepository, quotaRepo *repository.QuotaRepository) *QuotaService {
	return &QuotaService{
		userRepo:  userRepo,
		quotaRepo: quotaRepo,
	}
}

// ValidateQuotaRequest proverava da li je trazena kvota validna i veca od trenutne
func ValidateQuotaRequest(currentQuota, requestedQuota int64) error {
	if requestedQuota <= 0 || requestedQuota <= currentQuota {
		return ErrInvalidRequestedQuota
	}
	if requestedQuota > MaxAllowedStorageQuota {
		return ErrQuotaLimitTooHigh
	}
	return nil
}

// SubmitRequest kreira novi zahtev za povecanje kvote za ulogovanog korisnika
func (s *QuotaService) SubmitRequest(ctx context.Context, userID string, requestedQuota int64, reason string) (*model.QuotaRequest, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := ValidateQuotaRequest(user.QuotaLimit, requestedQuota); err != nil {
		return nil, err
	}

	hasPending, err := s.quotaRepo.HasPendingForUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if hasPending {
		return nil, ErrPendingRequestExists
	}

	req := &model.QuotaRequest{
		UserID:         user.ID,
		UserEmail:      user.Email,
		CurrentQuota:   user.QuotaLimit,
		RequestedQuota: requestedQuota,
		Reason:         strings.TrimSpace(reason),
		Status:         model.QuotaStatusPending,
	}

	if err := s.quotaRepo.Create(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

// GetMyRequests vraca zahteve za povecanje kvote datog korisnika
func (s *QuotaService) GetMyRequests(ctx context.Context, userID string) ([]model.QuotaRequest, error) {
	return s.quotaRepo.FindByUserID(ctx, userID)
}

// GetAllUsers vraca sve korisnike sa njihovom trenutnom kvotom i zauzecem (za administratora)
func (s *QuotaService) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return s.userRepo.FindAllUsers(ctx)
}

// GetAllRequests vraca sve pristigle zahteve za povecanje kvote (za administratora)
func (s *QuotaService) GetAllRequests(ctx context.Context) ([]model.QuotaRequest, error) {
	return s.quotaRepo.FindAll(ctx)
}

// ApproveRequest odobrava zahtev i azurira maksimalnu kvotu (quota_limit) korisnika
func (s *QuotaService) ApproveRequest(ctx context.Context, adminID, requestID string) (*model.QuotaRequest, error) {
	req, err := s.quotaRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.Status != model.QuotaStatusPending {
		return nil, ErrRequestAlreadyHandled
	}

	if err := s.userRepo.UpdateQuotaLimit(ctx, req.UserID, req.RequestedQuota); err != nil {
		return nil, fmt.Errorf("greska pri azuriranju kvote korisnika: %w", err)
	}

	return s.quotaRepo.UpdateStatus(ctx, req.ID, model.QuotaStatusApproved, adminID)
}

// RejectRequest odbija zahtev bez izmene kvote korisnika
func (s *QuotaService) RejectRequest(ctx context.Context, adminID, requestID string) (*model.QuotaRequest, error) {
	req, err := s.quotaRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.Status != model.QuotaStatusPending {
		return nil, ErrRequestAlreadyHandled
	}

	return s.quotaRepo.UpdateStatus(ctx, req.ID, model.QuotaStatusRejected, adminID)
}

// EnsureDefaultAdmin kreira podrazumevani administratorski nalog pri pokretanju ako jos ne postoji u bazi
func (s *QuotaService) EnsureDefaultAdmin(ctx context.Context, cfg *config.Config) error {
	email := strings.ToLower(strings.TrimSpace(cfg.AdminEmail))
	if email == "" || cfg.AdminPassword == "" {
		return nil
	}

	_, err := s.userRepo.FindByEmail(ctx, email)
	if err == nil {
		return nil // admin vec postoji
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("greska pri hesiranju lozinke za admin nalog: %w", err)
	}

	now := time.Now().UTC()
	adminUser := &model.User{
		FirstName:         cfg.AdminFirstName,
		LastName:          cfg.AdminLastName,
		Email:             email,
		PasswordHash:      string(hashed),
		Role:              model.RoleAdmin,
		IsVerified:        true,
		QuotaLimit:        cfg.DefaultStorageQuota,
		QuotaUsed:         0,
		PasswordChangedAt: now,
	}

	if err := s.userRepo.CreateUser(ctx, adminUser); err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return nil
		}
		return err
	}

	log.Printf("[USERS-SERVICE] Kreiran podrazumevani admin nalog: %s", email)
	return nil
}

