package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequireRolesMiddleware(t *testing.T) {
	secret := "test-jwt-secret"

	rkToken, err := GenerateToken(secret, "user-1", "rk@primer.com", "RK", time.Hour)
	if err != nil {
		t.Fatalf("failed to generate RK token: %v", err)
	}

	adminToken, err := GenerateToken(secret, "admin-1", "admin@primer.com", "A", time.Hour)
	if err != nil {
		t.Fatalf("failed to generate Admin token: %v", err)
	}

	okHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	// 1. Neautentifikovani korisnik (NK) -> 401 Unauthorized
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	recUnauth := httptest.NewRecorder()
	RequireRoles(secret, []string{"A"}, okHandler)(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("ocekivan status 401 za neautentifikovanog korisnika, dobijeno %d", recUnauth.Code)
	}

	// 2. Redovan korisnik (RK) pokusava da pristupi Admin ruti -> 403 Forbidden
	reqForbidden := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	reqForbidden.Header.Set("Authorization", "Bearer "+rkToken)
	recForbidden := httptest.NewRecorder()
	RequireRoles(secret, []string{"A"}, okHandler)(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Errorf("ocekivan status 403 za RK na Admin ruti, dobijeno %d", recForbidden.Code)
	}

	// 3. Administrator (A) pristupa Admin ruti -> 200 OK
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	recAdmin := httptest.NewRecorder()
	RequireRoles(secret, []string{"A"}, okHandler)(recAdmin, reqAdmin)
	if recAdmin.Code != http.StatusOK {
		t.Errorf("ocekivan status 200 za Admina na Admin ruti, dobijeno %d", recAdmin.Code)
	}

	// 4. Redovan korisnik (RK) pristupa RK ruti -> 200 OK
	reqRK := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
	reqRK.Header.Set("Authorization", "Bearer "+rkToken)
	recRK := httptest.NewRecorder()
	RequireRoles(secret, []string{"RK", "A"}, okHandler)(recRK, reqRK)
	if recRK.Code != http.StatusOK {
		t.Errorf("ocekivan status 200 za RK na dozvoljenoj ruti, dobijeno %d", recRK.Code)
	}
}
