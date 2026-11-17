package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/mvxv0/secure-file-storage/common/auth"
	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/config"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/database"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/handler"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/service"
)

func main() {
	cfg := config.Load()

	// povezivanje na MongoDB bazu (files_db na portu 27017)
	mongoDB, err := database.Connect(cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		log.Printf("[FILES-SERVICE] Upozorenje: MongoDB trenutno nije dostupan (%v). Pokrenite 'docker compose up -d'", err)
	} else {
		log.Printf("[FILES-SERVICE] Uspesno povezan na MongoDB bazu '%s'", cfg.MongoDBName)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mongoDB.Disconnect(ctx)
		}()
	}

	folderRepo := repository.NewFolderRepository(mongoDB.DB)
	folderService := service.NewFolderService(folderRepo)
	folderHandler := handler.NewFolderHandler(folderService)

	mux := http.NewServeMux()

	// health check ruta
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		mongoStatus := "connected"
		if mongoDB == nil || mongoDB.Ping(ctx) != nil {
			mongoStatus = "disconnected"
		}

		httputil.WriteSuccess(w, http.StatusOK, "files-service je aktivan", map[string]string{
			"service": "files-service",
			"mongodb": mongoStatus,
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// rute za upravljanje folderima (zasticene JWT autorizacijom)
	mux.HandleFunc("POST /api/folders", auth.Middleware(cfg.JWTSecret, folderHandler.CreateFolder))
	mux.HandleFunc("GET /api/folders", auth.Middleware(cfg.JWTSecret, folderHandler.GetFolderContents))
	mux.HandleFunc("GET /api/folders/", auth.Middleware(cfg.JWTSecret, folderHandler.GetFolderContents))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[FILES-SERVICE] Pokrenut na http://localhost:%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[FILES-SERVICE] Greska pri pokretanju servera: %v", err)
	}
}
