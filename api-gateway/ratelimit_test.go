package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvxv0/secure-file-storage/common/auth"
)

func TestRateLimiter_BlocksAfterLimitAndResets(t *testing.T) {
	rl := NewRateLimiter(3, 2, 150*time.Millisecond, "test-secret")
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Prva 3 zahteva moraju da prodju
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
		req.RemoteAddr = "192.168.1.10:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("zahtev %d: ocekivan 200 OK, dobijeno %d", i, rec.Code)
		}
		if rec.Header().Get("X-RateLimit-Limit") != "3" {
			t.Errorf("ocekivan X-RateLimit-Limit=3, dobijeno %s", rec.Header().Get("X-RateLimit-Limit"))
		}
	}

	// 4. zahtev mora biti blokiran sa 429 Too Many Requests
	reqBlocked := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
	reqBlocked.RemoteAddr = "192.168.1.10:12345"
	recBlocked := httptest.NewRecorder()
	handler.ServeHTTP(recBlocked, reqBlocked)

	if recBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("4. zahtev: ocekivan 429 Too Many Requests, dobijeno %d", recBlocked.Code)
	}
	if recBlocked.Header().Get("Retry-After") == "" {
		t.Errorf("ocekivan Retry-After header pri 429 odgovoru")
	}

	// Nakon isteka vremenskog prozora zahtev ponovo prolazi
	time.Sleep(180 * time.Millisecond)
	reqAfterReset := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
	reqAfterReset.RemoteAddr = "192.168.1.10:12345"
	recAfterReset := httptest.NewRecorder()
	handler.ServeHTTP(recAfterReset, reqAfterReset)

	if recAfterReset.Code != http.StatusOK {
		t.Fatalf("nakon reseta prozora ocekivan 200 OK, dobijeno %d", recAfterReset.Code)
	}
}

func TestRateLimiter_IsolatesPerAuthenticatedUser(t *testing.T) {
	secret := "test-secret"
	rl := NewRateLimiter(2, 2, time.Minute, secret)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tokenA, _ := auth.GenerateToken(secret, "user-A", "a@example.com", "RK", time.Hour)
	tokenB, _ := auth.GenerateToken(secret, "user-B", "b@example.com", "RK", time.Hour)

	// Korisnik A istrosi svoj limit (2 zahteva), 3. pada sa 429
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		req.RemoteAddr = "10.0.0.1:5000"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("korisnik A zahtev %d: ocekivan 200, dobijeno %d", i+1, rec.Code)
		}
	}

	reqABlocked := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	reqABlocked.Header.Set("Authorization", "Bearer "+tokenA)
	reqABlocked.RemoteAddr = "10.0.0.1:5000"
	recABlocked := httptest.NewRecorder()
	handler.ServeHTTP(recABlocked, reqABlocked)
	if recABlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("korisnik A 3. zahtev: ocekivan 429, dobijeno %d", recABlocked.Code)
	}

	// Korisnik B sa iste IP adrese i dalje moze normalno da salje zahteve
	reqB := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	reqB.Header.Set("Authorization", "Bearer "+tokenB)
	reqB.RemoteAddr = "10.0.0.1:5000"
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("korisnik B ne sme biti blokiran zbog korisnika A: dobijeno %d", recB.Code)
	}
}

func TestRateLimiter_StricterAuthLimitAndSkipsOptions(t *testing.T) {
	rl := NewRateLimiter(10, 2, time.Minute, "test-secret")
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// OPTIONS ne trosi kvotu
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
		req.RemoteAddr = "127.0.0.1:9000"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("OPTIONS ne sme biti blokiran, dobijeno %d", rec.Code)
		}
	}

	// 2 POST zahteva na /api/auth/login prolaze, 3. dobija 429
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "127.0.0.1:9000"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("auth zahtev %d: ocekivan 200, dobijeno %d", i+1, rec.Code)
		}
	}

	req3 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req3.RemoteAddr = "127.0.0.1:9000"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("3. auth zahtev: ocekivan 429, dobijeno %d", rec3.Code)
	}
}

