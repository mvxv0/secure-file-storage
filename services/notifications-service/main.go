package main

import (
	"log"
	"net/http"
	"time"

	"github.com/mvxv0/secure-file-storage/common/auth"
	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/config"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/database"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/handler"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/notifications-service/internal/service"
)

func main() {
	cfg := config.Load()

	// inicijalizacija repozitorijuma (povezuje se na Cassandru cim klaster postane dostupan)
	notificationRepo := repository.NewNotificationRepository(nil)

	var cassandraDB *database.CassandraDB
	connectCassandra := func() {
		for {
			db, err := database.Connect(cfg.CassandraHosts, cfg.CassandraKeyspace)
			if err != nil {
				log.Printf("[NOTIFICATIONS-SERVICE] Ceka se Cassandra (%s): %v. Ponovni pokusaj za 5s...", cfg.CassandraHosts, err)
				time.Sleep(5 * time.Second)
				continue
			}
			cassandraDB = db
			notificationRepo.SetSession(db.Session)
			log.Printf("[NOTIFICATIONS-SERVICE] Uspesno povezan na Cassandra klaster (%s)", cfg.CassandraHosts)
			return
		}
	}

	// pokrecemo povezivanje u pozadini kako servis ne bi pao dok se Cassandra podize u Dockeru
	go connectCassandra()
	defer func() {
		if cassandraDB != nil {
			cassandraDB.Close()
		}
	}()

	notificationService := service.NewNotificationService(notificationRepo, cfg.AutoSeed)
	notificationHandler := handler.NewNotificationHandler(notificationService)

	mux := http.NewServeMux()

	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		cassandraStatus := "connected"
		if cassandraDB == nil || cassandraDB.Ping() != nil {
			cassandraStatus = "disconnected"
		}

		httputil.WriteSuccess(w, http.StatusOK, "notifications-service je aktivan", map[string]string{
			"service":   "notifications-service",
			"cassandra": cassandraStatus,
			"keyspace":  cfg.CassandraKeyspace,
			"time":      time.Now().Format(time.RFC3339),
		})
	}

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/notifications/health", healthHandler)

	// rute za pregled i rucno popunjavanje notifikacija
	mux.HandleFunc("GET /api/notifications", auth.Middleware(cfg.JWTSecret, notificationHandler.GetNotifications))
	mux.HandleFunc("POST /api/notifications", auth.Middleware(cfg.JWTSecret, notificationHandler.CreateNotification))
	mux.HandleFunc("POST /api/notifications/seed", auth.Middleware(cfg.JWTSecret, notificationHandler.SeedNotifications))
	mux.HandleFunc("POST /api/notifications/read", auth.Middleware(cfg.JWTSecret, notificationHandler.MarkAllAsRead))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[NOTIFICATIONS-SERVICE] Pokrenut na http://localhost:%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[NOTIFICATIONS-SERVICE] Greska pri pokretanju servera: %v", err)
	}
}
