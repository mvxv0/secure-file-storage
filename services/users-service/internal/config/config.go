package config

import (
	"time"

	"github.com/mvxv0/secure-file-storage/common/env"
)

// Config cuva sve parametre za users-service ucitane iz .env fajla
type Config struct {
	Port                 string
	MongoURI             string
	MongoDBName          string
	SMTPHost             string
	SMTPPort             string
	SMTPFrom             string
	JWTSecret            string
	JWTExpiration        time.Duration
	OTPExpiration        time.Duration
	ResetTokenExpiration time.Duration
	MinPasswordAge       time.Duration
	MaxPasswordAge       time.Duration
	AccountLockDuration  time.Duration
	DefaultStorageQuota  int64
	FrontendURL          string
}

func Load() *Config {
	env.Load()

	return &Config{
		Port:                 env.GetString("USERS_SERVICE_PORT", "8081"),                           // port na kom se dize users-service
		MongoURI:             env.GetString("MONGO_URI", "mongodb://admin:password@localhost:27017"), // mongo u dockeru na 27017
		MongoDBName:          env.GetString("MONGO_DB_USERS", "users_db"),                           // naziv mongo baze za korisnike
		SMTPHost:             env.GetString("SMTP_HOST", "localhost"),                               // mailhog host
		SMTPPort:             env.GetString("SMTP_PORT", "1025"),                                    // mailhog smtp port za slanje mejlova (ui je na 8025)
		SMTPFrom:             env.GetString("SMTP_FROM", "noreply@secure-storage.local"),
		JWTSecret:            env.GetString("JWT_SECRET", "dev-secret-key-promeni-u-produkciji"),
		JWTExpiration:        time.Duration(env.GetInt("JWT_EXPIRATION_HOURS", 24)) * time.Hour,
		OTPExpiration:        time.Duration(env.GetInt("OTP_EXPIRATION_SECONDS", 300)) * time.Second,
		ResetTokenExpiration: time.Duration(env.GetInt("RESET_TOKEN_EXPIRATION_SECONDS", 900)) * time.Second,
		MinPasswordAge:       time.Duration(env.GetInt("MIN_PASSWORD_AGE_SECONDS", 86400)) * time.Second,
		MaxPasswordAge:       time.Duration(env.GetInt("MAX_PASSWORD_AGE_SECONDS", 5184000)) * time.Second,
		AccountLockDuration:  time.Duration(env.GetInt("ACCOUNT_LOCK_DURATION_SECONDS", 300)) * time.Second,
		DefaultStorageQuota:  env.GetInt64("DEFAULT_STORAGE_QUOTA_BYTES", 104857600), // 100 MB pocetna kvota
		FrontendURL:          env.GetString("FRONTEND_URL", "http://localhost:5173"),
	}
}
