package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

type CassandraDB struct {
	Session  *gocql.Session
	Keyspace string
}

// Connect se povezuje na Cassandra klaster, kreira keyspace i tabelu notifications_by_user ako ne postoje
func Connect(hostsStr string, keyspace string) (*CassandraDB, error) {
	hosts := strings.Split(hostsStr, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}

	// 1. Prvo se povezujemo na 'system' keyspace da bismo kreirali nas keyspace ako ne postoji
	sysCluster := gocql.NewCluster(hosts...)
	sysCluster.Keyspace = "system"
	sysCluster.Consistency = gocql.Quorum
	sysCluster.Timeout = 10 * time.Second
	sysCluster.ConnectTimeout = 10 * time.Second

	sysSession, err := sysCluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("neuspesno povezivanje na Cassandra system keyspace: %w", err)
	}

	createKeyspaceQuery := fmt.Sprintf(
		`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`,
		keyspace,
	)
	if err := sysSession.Query(createKeyspaceQuery).Exec(); err != nil {
		sysSession.Close()
		return nil, fmt.Errorf("neuspesno kreiranje keyspace-a '%s': %w", keyspace, err)
	}
	sysSession.Close()

	// 2. Povezujemo se na kreirani keyspace i kreiramo wide-column tabelu po upitu korisnika
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.One
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("neuspesno povezivanje na Cassandra keyspace '%s': %w", keyspace, err)
	}

	createTableQuery := `
		CREATE TABLE IF NOT EXISTS notifications_by_user (
			user_id text,
			created_at timestamp,
			id text,
			type text,
			resource_id text,
			resource_name text,
			message text,
			is_read boolean,
			PRIMARY KEY ((user_id), created_at, id)
		) WITH CLUSTERING ORDER BY (created_at DESC, id ASC)`

	if err := session.Query(createTableQuery).Exec(); err != nil {
		session.Close()
		return nil, fmt.Errorf("neuspesno kreiranje tabele notifications_by_user: %w", err)
	}

	log.Printf("[NOTIFICATIONS-SERVICE] Cassandra keyspace '%s' i tabela 'notifications_by_user' su spremni", keyspace)
	return &CassandraDB{
		Session:  session,
		Keyspace: keyspace,
	}, nil
}

func (db *CassandraDB) Ping() error {
	if db == nil || db.Session == nil || db.Session.Closed() {
		return fmt.Errorf("cassandra sesija nije aktivna")
	}
	return db.Session.Query("SELECT now() FROM system.local").Exec()
}

func (db *CassandraDB) Close() {
	if db != nil && db.Session != nil && !db.Session.Closed() {
		db.Session.Close()
	}
}
