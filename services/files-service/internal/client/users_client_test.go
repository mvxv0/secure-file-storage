package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPUsersClient_GetUserQuota_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("ocekivan GET metod, dobijen %s", r.Method)
		}
		if r.URL.Path != "/internal/users/user-123/quota" {
			t.Errorf("neocekivana putanja: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "ok",
			"data": map[string]interface{}{
				"userId":     "user-123",
				"quotaLimit": int64(209715200), // 200 MB
				"quotaUsed":  int64(1024),
			},
		})
	}))
	defer server.Close()

	c := NewHTTPUsersClient(server.URL)
	quota, err := c.GetUserQuota(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("neocekivana greska: %v", err)
	}
	if quota.QuotaLimit != 209715200 || quota.QuotaUsed != 1024 {
		t.Fatalf("neocekivane vrednosti kvote: %+v", quota)
	}
}

func TestHTTPUsersClient_GetUserQuota_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewHTTPUsersClient(server.URL)
	_, err := c.GetUserQuota(context.Background(), "nepostojeci")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("ocekivana greska ErrUserNotFound, dobijeno: %v", err)
	}
}

func TestHTTPUsersClient_GetUserQuota_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewHTTPUsersClient(server.URL)
	_, err := c.GetUserQuota(context.Background(), "user-123")
	if !errors.Is(err, ErrUsersServiceUnavailable) {
		t.Fatalf("ocekivana greska ErrUsersServiceUnavailable, dobijeno: %v", err)
	}
}

func TestHTTPUsersClient_UpdateQuotaUsed_Success(t *testing.T) {
	var receivedBody map[string]int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("ocekivan PUT metod, dobijen %s", r.Method)
		}
		if r.URL.Path != "/internal/users/user-123/quota-used" {
			t.Errorf("neocekivana putanja: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("neuspesno dekodiranje tela: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}))
	defer server.Close()

	c := NewHTTPUsersClient(server.URL)
	if err := c.UpdateQuotaUsed(context.Background(), "user-123", 4096); err != nil {
		t.Fatalf("neocekivana greska: %v", err)
	}
	if receivedBody["quotaUsed"] != 4096 {
		t.Fatalf("ocekivano quotaUsed=4096, dobijeno %d", receivedBody["quotaUsed"])
	}
}

