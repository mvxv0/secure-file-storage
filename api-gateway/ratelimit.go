package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvxv0/secure-file-storage/common/auth"
	commonhttp "github.com/mvxv0/secure-file-storage/common/httputil"
)

type rateBucket struct {
	count   int
	resetAt time.Time
}

// RateLimiter implementira zastitu od DoS napada ogranicavanjem broja zahteva
// koje jedan korisnik (po userId iz JWT-a) ili IP adresa sme da izvrsi u zadatom vremenskom prozoru.
//
// demonstracija na kontrolnoj tacki (limit za /api/auth/* je 20 req/min):
//
//	for i in $(seq 1 25); do
//	  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/auth/health)
//	  echo "Zahtev $i -> HTTP $code"
//	done
//
// provera logova blokiranih zahteva na gateway-u:
//
//	docker compose logs --tail 10 api-gateway
type RateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*rateBucket
	maxRequests  int
	authMaxReqs  int
	window       time.Duration
	jwtSecret    string
	cleanupEvery time.Duration
}

func NewRateLimiter(maxRequests, authMaxRequests int, window time.Duration, jwtSecret string) *RateLimiter {
	if maxRequests <= 0 {
		maxRequests = 60
	}
	if authMaxRequests <= 0 {
		authMaxRequests = 20
	}
	if window <= 0 {
		window = time.Minute
	}

	rl := &RateLimiter{
		buckets:      make(map[string]*rateBucket),
		maxRequests:  maxRequests,
		authMaxReqs:  authMaxRequests,
		window:       window,
		jwtSecret:    jwtSecret,
		cleanupEvery: 2 * window,
	}

	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupEvery)
	defer ticker.Stop()

	for now := range ticker.C {
		rl.mu.Lock()
		for key, b := range rl.buckets {
			if now.After(b.resetAt) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// allow proverava da li je zahtev za dati kljuc dozvoljen u tekucem prozoru.
// Vraca: (dozvoljeno, preostalo, vreme reseta).
func (rl *RateLimiter) allow(key string, limit int, now time.Time) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists || !now.Before(b.resetAt) {
		resetAt := now.Add(rl.window)
		rl.buckets[key] = &rateBucket{
			count:   1,
			resetAt: resetAt,
		}
		return true, limit - 1, resetAt
	}

	if b.count >= limit {
		return false, 0, b.resetAt
	}

	b.count++
	return true, limit - b.count, b.resetAt
}

// extractClientKey odredjuje identitet klijenta: ako postoji validan JWT, koristi userId;
// u suprotnom koristi IP adresu klijenta.
func (rl *RateLimiter) extractClientKey(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") && rl.jwtSecret != "" {
		tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if claims, err := auth.ValidateToken(tokenStr, rl.jwtSecret); err == nil && claims.UserID != "" {
			return "user:" + claims.UserID
		}
	}
	return "ip:" + extractClientIP(r)
}

func extractClientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// Middleware ogranicava broj zahteva po korisniku/IP adresi i vraca 429 Too Many Requests pri prekoracenju.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Preflight zahtevi browsera (CORS OPTIONS) ne trose korisnicku kvotu zahteva
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		clientKey := rl.extractClientKey(r)
		limit := rl.maxRequests
		bucketKey := "general:" + clientKey

		// Strozi limit za autentifikacione rute (/api/auth/*)
		if strings.HasPrefix(r.URL.Path, "/api/auth") {
			limit = rl.authMaxReqs
			bucketKey = "auth:" + clientKey
		}

		now := time.Now()
		allowed, remaining, resetAt := rl.allow(bucketKey, limit, now)

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

		if !allowed {
			retryAfter := int(time.Until(resetAt).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			log.Printf("[API-GATEWAY] [DoS-ZASTITA] Prekoracen limit (%d/%s) za %s na ruti %s %s",
				limit, rl.window, clientKey, r.Method, r.URL.Path)

			commonhttp.WriteError(
				w,
				http.StatusTooManyRequests,
				fmt.Sprintf("Prekoracen dozvoljeni broj zahteva (%d za %s). Pokusajte ponovo za %d sekundi.", limit, rl.window, retryAfter),
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}
