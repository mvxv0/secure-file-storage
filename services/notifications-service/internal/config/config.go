package config

import "github.com/mvxv0/secure-file-storage/common/env"

type Config struct {
	Port              string
	CassandraHosts    string
	CassandraKeyspace string
	JWTSecret         string
	AutoSeed          bool
}

func Load() *Config {
	env.Load()

	return &Config{
		Port:              env.GetString("NOTIFICATIONS_SERVICE_PORT", "8084"),
		CassandraHosts:    env.GetString("CASSANDRA_HOSTS", "localhost:9042"),
		CassandraKeyspace: env.GetString("CASSANDRA_KEYSPACE", "notifications_ks"),
		JWTSecret:         env.GetString("JWT_SECRET", "dev-secret-key-promeni-u-produkciji"),
		AutoSeed:          env.GetString("NOTIFICATIONS_AUTO_SEED", "true") == "true",
	}
}
