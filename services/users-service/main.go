package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/config"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/database"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/handler"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/mailer"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/service"
)

func main() {
	// ucitavamo konfiguraciju iz .env fajla (port 8081, mongo na 27017, mailhog na 1025)
	cfg := config.Load()

	// povezivanje na mongo bazu (docker kontejner gdrive-mongo na portu 27017)
	mongoDB, err := database.Connect(cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		log.Printf("[USERS-SERVICE] Upozorenje: MongoDB trenutno nije dostupan (%v). Pokrenite 'docker compose up -d'", err)
	} else {
		log.Printf("[USERS-SERVICE] Uspesno povezan na MongoDB bazu '%s'", cfg.MongoDBName)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mongoDB.Disconnect(ctx)
		}()
	}

	// inicijalizacija smtp klijenta za mailhog (docker kontejner gdrive-mailhog na portu 1025, web inbox na 8025)
	mailClient := mailer.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)

	// inicijalizacija slojeva: repository -> service -> handler
	userRepo := repository.NewUserRepository(mongoDB.DB)
	authService := service.NewAuthService(userRepo, mailClient, cfg)
	authHandler := handler.NewAuthHandler(authService)

	mux := http.NewServeMux()

	// health endpoint koji proverava status users-service-a i da li je mongo baza povezana
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		mongoStatus := "connected"
		if mongoDB == nil || mongoDB.Ping(ctx) != nil {
			mongoStatus = "disconnected"
		}

		httputil.WriteSuccess(w, http.StatusOK, "users-service je aktivan", map[string]string{
			"service": "users-service",
			"mongodb": mongoStatus,
			"smtp":    cfg.SMTPHost + ":" + cfg.SMTPPort,
			"time":    time.Now().Format(time.RFC3339),
		})
	}

	// rute za proveru zdravlja servisa
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/auth/health", healthHandler)
	mux.HandleFunc("GET /api/users/health", healthHandler)

	// rute za registraciju i potvrdu registracije (zahtev 1.1)
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/verify-email", authHandler.VerifyEmail)

	// rute za kombinovanu prijavu (lozinka + OTP) i odjavu (zahtev 1.2)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/verify-otp", authHandler.VerifyOTP)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

	// pokretanje http servera za users-service na portu 8081
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[USERS-SERVICE] Pokrenut na http://localhost:%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[USERS-SERVICE] Greska pri pokretanju servera: %v", err)
	}
}
