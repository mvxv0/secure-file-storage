package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/mvxv0/secure-file-storage/common/env"
	commonhttp "github.com/mvxv0/secure-file-storage/common/httputil"
)

// kreira reverse proxy koji prosledjuje zahteve sa api-gateway-a na odgovarajuci mikroservis
func newServiceProxy(targetURL string, serviceName string) http.Handler {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("Nevalidan URL za %s (%s): %v", serviceName, targetURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parsedURL)
	// ako mikroservis nije upaljen, vraca 502 json gresku umesto da srusi gateway
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[API-GATEWAY] Greska pri komunikaciji sa %s (%s): %v", serviceName, r.URL.Path, err)
		commonhttp.WriteError(w, http.StatusBadGateway, fmt.Sprintf("Servis '%s' trenutno nije dostupan", serviceName))
	}
	return proxy
}

func main() {
	// ucitava promenljive iz .env fajla
	env.Load()

	// api-gateway slusa na portu 8080 (glavna ulazna tacka za klijenta / postman)
	port := env.GetString("API_GATEWAY_PORT", "8080")
	frontendURL := env.GetString("FRONTEND_URL", "https://localhost:4200")
	jwtSecret := env.GetString("JWT_SECRET", "dev-secret-key-promeni-u-produkciji")

	// HTTPS / TLS konfiguracija izmedju klijentske aplikacije i API Gateway-a 
	tlsEnabled := strings.ToLower(env.GetString("TLS_ENABLED", "true")) != "false"
	tlsCertFile := env.GetString("TLS_CERT_FILE", "certs/gateway.crt")
	tlsKeyFile := env.GetString("TLS_KEY_FILE", "certs/gateway.key")

	// parametri za DoS zastitu / rate limiting
	rateLimitMax := env.GetInt("RATE_LIMIT_MAX_REQUESTS", 60)
	rateLimitAuthMax := env.GetInt("RATE_LIMIT_AUTH_MAX_REQUESTS", 20)
	rateLimitWindow := time.Duration(env.GetInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second
	rateLimiter := NewRateLimiter(rateLimitMax, rateLimitAuthMax, rateLimitWindow, jwtSecret)

	// adrese i portovi mikroservisa na koje gateway prosledjuje zahteve (interna komunikacija je HTTP za ocenu 6)
	usersURL := env.GetString("USERS_SERVICE_URL", "http://localhost:8081")                 // port 8081
	filesURL := env.GetString("FILES_SERVICE_URL", "http://localhost:8082")                 // port 8082
	sharingURL := env.GetString("SHARING_SERVICE_URL", "http://localhost:8083")             // port 8083
	notificationsURL := env.GetString("NOTIFICATIONS_SERVICE_URL", "http://localhost:8084") // port 8084
	analyticsURL := env.GetString("ANALYTICS_SERVICE_URL", "http://localhost:8085")         // port 8085

	mux := http.NewServeMux()

	// health check ruta za sam api-gateway (GET https://localhost:8080/health)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		protocol := "http"
		if r.TLS != nil {
			protocol = "https"
		}
		commonhttp.WriteSuccess(w, http.StatusOK, "api-gateway je aktivan", map[string]string{
			"service":  "api-gateway",
			"protocol": protocol,
			"time":     time.Now().Format(time.RFC3339),
		})
	})

	// inicijalizacija proxy-ja za svaki mikroservis
	usersProxy := newServiceProxy(usersURL, "users-service")
	filesProxy := newServiceProxy(filesURL, "files-service")
	sharingProxy := newServiceProxy(sharingURL, "sharing-service")
	notificationsProxy := newServiceProxy(notificationsURL, "notifications-service")
	analyticsProxy := newServiceProxy(analyticsURL, "analytics-service")

	// mapiranje ruta: sve sto stigne na /api/auth/* i /api/users/* ide na users-service (:8081)
	mux.Handle("/api/auth/", usersProxy)
	mux.Handle("/api/auth", usersProxy)
	mux.Handle("/api/users/", usersProxy)
	mux.Handle("/api/users", usersProxy)

	// files-service rute (:8082)
	mux.Handle("/api/files/", filesProxy)
	mux.Handle("/api/files", filesProxy)
	mux.Handle("/api/folders/", filesProxy)
	mux.Handle("/api/folders", filesProxy)

	// ostali servisi
	mux.Handle("/api/sharing/", sharingProxy)
	mux.Handle("/api/sharing", sharingProxy)
	mux.Handle("/api/groups/", sharingProxy)
	mux.Handle("/api/groups", sharingProxy)
	mux.Handle("/api/notifications/", notificationsProxy)
	mux.Handle("/api/notifications", notificationsProxy)
	mux.Handle("/api/analytics/", analyticsProxy)
	mux.Handle("/api/analytics", analyticsProxy)

	handlerChain := commonhttp.CORSMiddleware(
		frontendURL,
		SecurityAndMethodMiddleware(rateLimiter.Middleware(mux)),
	)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handlerChain,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if tlsEnabled {
		tlsCfg, err := EnsureTLSCertificate(tlsCertFile, tlsKeyFile)
		if err != nil {
			log.Fatalf("[API-GATEWAY] Greska pri inicijalizaciji TLS sertifikata: %v", err)
		}
		server.TLSConfig = tlsCfg

		log.Printf("[API-GATEWAY] Pokrenut preko HTTPS-a na https://localhost:%s (Users -> %s, Files -> %s, RateLimit: %d req/%s, Auth: %d req/%s)",
			port, usersURL, filesURL, rateLimitMax, rateLimitWindow, rateLimitAuthMax, rateLimitWindow)
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Fatalf("[API-GATEWAY] Greska pri pokretanju HTTPS servera: %v", err)
		}
		return
	}

	log.Printf("[API-GATEWAY] Pokrenut preko HTTP-a na http://localhost:%s (Users -> %s, Files -> %s)",
		port, usersURL, filesURL)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[API-GATEWAY] Greska pri pokretanju servera: %v", err)
	}
}
