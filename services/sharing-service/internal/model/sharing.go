package model

import "time"

// UserNode predstavlja cvor korisnika (:User) u Neo4j graf bazi
type UserNode struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
}

// GroupNode predstavlja grupu korisnika (:Group) u Neo4j graf bazi
type GroupNode struct {
	GroupID   string    `json:"groupId"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"ownerId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ResourceNode predstavlja datoteku ili folder (:Resource) u Neo4j graf bazi
type ResourceNode struct {
	ResourceID     string  `json:"resourceId"`
	ResourceType   string  `json:"resourceType"` // "FILE" ili "FOLDER"
	OwnerID        string  `json:"ownerId"`
	ParentFolderID *string `json:"parentFolderId,omitempty"`
}

// Share predstavlja relaciju deljenja [:SHARED_WITH] u Neo4j graf bazi
type Share struct {
	ID           string    `json:"id"`
	ResourceID   string    `json:"resourceId"`
	ResourceType string    `json:"resourceType"` // "FILE" ili "FOLDER"
	TargetID     string    `json:"targetId"`     // ID korisnika ili grupe
	TargetType   string    `json:"targetType"`   // "USER" ili "GROUP"
	SharedAt     time.Time `json:"sharedAt"`
}
