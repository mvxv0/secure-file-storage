package main

import (
	"log"
	"net/http"
	"time"

	"github.com/mvxv0/secure-file-storage/common/env"
	"github.com/mvxv0/secure-file-storage/common/httputil"
)

// privremeni skeleton servisa: dize http server sa health rutom da bi kontejner
// ostao aktivan i bio dostupan preko api-gateway-a dok se ne implementira logika
func main() {
	env.Load()
	port := env.GetString("ANALYTICS_SERVICE_PORT", "8085")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		httputil.WriteSuccess(w, http.StatusOK, "analytics-service je aktivan", map[string]string{
			"service": "analytics-service",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[ANALYTICS-SERVICE] Pokrenut na http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[ANALYTICS-SERVICE] Greska pri pokretanju servera: %v", err)
	}
}
