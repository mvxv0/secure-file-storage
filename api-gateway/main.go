package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	frontendURL := env.GetString("FRONTEND_URL", "http://localhost:5173")

	// adrese i portovi mikroservisa na koje gateway prosledjuje zahteve
	usersURL := env.GetString("USERS_SERVICE_URL", "http://localhost:8081")                 // port 8081
	filesURL := env.GetString("FILES_SERVICE_URL", "http://localhost:8082")                 // port 8082
	sharingURL := env.GetString("SHARING_SERVICE_URL", "http://localhost:8083")             // port 8083
	notificationsURL := env.GetString("NOTIFICATIONS_SERVICE_URL", "http://localhost:8084") // port 8084
	analyticsURL := env.GetString("ANALYTICS_SERVICE_URL", "http://localhost:8085")         // port 8085

	mux := http.NewServeMux()

	// health check ruta za sam api-gateway (GET http://localhost:8080/health)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		commonhttp.WriteSuccess(w, http.StatusOK, "api-gateway je aktivan", map[string]string{
			"service": "api-gateway",
			"time":    time.Now().Format(time.RFC3339),
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
	mux.Handle("/api/users/", usersProxy)
	mux.Handle("/api/files/", filesProxy)
	mux.Handle("/api/folders/", filesProxy)
	mux.Handle("/api/sharing/", sharingProxy)
	mux.Handle("/api/groups/", sharingProxy)
	mux.Handle("/api/notifications/", notificationsProxy)
	mux.Handle("/api/analytics/", analyticsProxy)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      commonhttp.CORSMiddleware(frontendURL, mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[API-GATEWAY] Pokrenut na http://localhost:%s (Users -> %s)", port, usersURL)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[API-GATEWAY] Greska pri pokretanju servera: %v", err)
	}
}
