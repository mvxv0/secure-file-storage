package config

import (
	"github.com/mvxv0/secure-file-storage/common/env"
)

type Config struct {
	Port                string
	MongoURI            string
	MongoDBName         string
	JWTSecret           string
	StorageDir          string
	DefaultStorageQuota int64
	UsersServiceURL     string
}

func Load() *Config {
	env.Load()

	return &Config{
		Port:                env.GetString("FILES_SERVICE_PORT", "8082"),
		MongoURI:            env.GetString("MONGO_URI", "mongodb://admin:password@localhost:27017"),
		MongoDBName:         env.GetString("MONGO_DB_FILES", "files_db"),
		JWTSecret:           env.GetString("JWT_SECRET", ""),
		StorageDir:          env.GetString("FILES_STORAGE_DIR", "uploads"),
		DefaultStorageQuota: env.GetInt64("DEFAULT_STORAGE_QUOTA_BYTES", 104857600), // 100 MB
		UsersServiceURL:     env.GetString("USERS_SERVICE_URL", "http://localhost:8081"),
	}
}
