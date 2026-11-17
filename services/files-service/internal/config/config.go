package config

import (
	"github.com/mvxv0/secure-file-storage/common/env"
)

type Config struct {
	Port        string
	MongoURI    string
	MongoDBName string
	JWTSecret   string
}

func Load() *Config {
	env.Load()

	return &Config{
		Port:        env.GetString("FILES_SERVICE_PORT", "8082"),
		MongoURI:    env.GetString("MONGO_URI", "mongodb://admin:password@localhost:27017"),
		MongoDBName: env.GetString("MONGO_DB_FILES", "files_db"),
		JWTSecret:   env.GetString("JWT_SECRET", "dev-secret-key-promeni-u-produkciji"),
	}
}

