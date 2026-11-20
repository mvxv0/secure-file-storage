package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrUserNotFound            = errors.New("korisnik nije pronadjen u users-service")
	ErrUsersServiceUnavailable = errors.New("users-service trenutno nije dostupan")
)

// QuotaInfo predstavlja podatak o kvoti korisnika dobijen sinhrono od users-service-a
type QuotaInfo struct {
	UserID     string `json:"userId"`
	QuotaLimit int64  `json:"quotaLimit"`
	QuotaUsed  int64  `json:"quotaUsed"`
}

// QuotaClient definise interfejs za sinhronu komunikaciju sa users-service-om oko kvote
type QuotaClient interface {
	GetUserQuota(ctx context.Context, userID string) (*QuotaInfo, error)
	UpdateQuotaUsed(ctx context.Context, userID string, quotaUsed int64) error
}

// HTTPUsersClient salje sinhrone HTTP REST pozive ka users-service-u
type HTTPUsersClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPUsersClient(baseURL string) *HTTPUsersClient {
	return &HTTPUsersClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type quotaAPIResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message,omitempty"`
	Error   string    `json:"error,omitempty"`
	Data    QuotaInfo `json:"data"`
}

// GetUserQuota sinhrono poziva GET /internal/users/{id}/quota na users-service-u
func (c *HTTPUsersClient) GetUserQuota(ctx context.Context, userID string) (*QuotaInfo, error) {
	url := fmt.Sprintf("%s/internal/users/%s/quota", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUsersServiceUnavailable, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUsersServiceUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrUserNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrUsersServiceUnavailable, resp.StatusCode)
	}

	var apiResp quotaAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("%w: nevalidan odgovor (%v)", ErrUsersServiceUnavailable, err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("%w: %s", ErrUsersServiceUnavailable, apiResp.Error)
	}

	return &apiResp.Data, nil
}

// UpdateQuotaUsed sinhrono poziva PUT /internal/users/{id}/quota-used na users-service-u
func (c *HTTPUsersClient) UpdateQuotaUsed(ctx context.Context, userID string, quotaUsed int64) error {
	url := fmt.Sprintf("%s/internal/users/%s/quota-used", c.baseURL, userID)
	payload, err := json.Marshal(map[string]int64{
		"quotaUsed": quotaUsed,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUsersServiceUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUsersServiceUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrUserNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUsersServiceUnavailable, resp.StatusCode)
	}

	return nil
}

