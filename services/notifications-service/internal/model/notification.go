package model

import "time"

// Notification predstavlja notifikaciju u Cassandra wide-column bazi
// PRIMARY KEY ((user_id), created_at, id) WITH CLUSTERING ORDER BY (created_at DESC)
type Notification struct {
	UserID       string    `json:"userId" cql:"user_id"`             // Partition Key: korisnik koji prima notifikaciju
	CreatedAt    time.Time `json:"createdAt" cql:"created_at"`       // Clustering Key: vreme kreiranja (DESC)
	ID           string    `json:"id" cql:"id"`                      // Clustering Key: jedinstveni ID notifikacije
	Type         string    `json:"type" cql:"type"`                  // tip dogadjaja (npr. FILE_SHARED, FOLDER_SHARED, NEW_VERSION)
	ResourceID   string    `json:"resourceId" cql:"resource_id"`     // ID datoteke ili foldera
	ResourceName string    `json:"resourceName" cql:"resource_name"` // naziv datoteke ili foldera
	Message      string    `json:"message" cql:"message"`            // tekst notifikacije za prikaz korisniku
	IsRead       bool      `json:"isRead" cql:"is_read"`
}
