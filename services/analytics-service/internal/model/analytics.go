package model

import "time"

// Activity predstavlja zapis o aktivnosti korisnika na nalogu
type Activity struct {
	ID           string    `json:"id" bson:"_id,omitempty"`
	UserID       string    `json:"userId" bson:"user_id"`
	Action       string    `json:"action" bson:"action"` // npr. UPLOAD, RENAME, DELETE, DOWNLOAD, SHARE
	ResourceID   string    `json:"resourceId" bson:"resource_id"`
	ResourceName string    `json:"resourceName" bson:"resource_name"`
	ResourceType string    `json:"resourceType" bson:"resource_type"` // FILE ili FOLDER
	CreatedAt    time.Time `json:"createdAt" bson:"created_at"`
}

// AccountAnalytics predstavlja analitiku korisnickog naloga
type AccountAnalytics struct {
	UserID            string         `json:"userId" bson:"user_id"`
	TotalStorageUsed  int64          `json:"totalStorageUsed" bson:"total_storage_used"`    // ukupno zauzece skladista u bajtovima
	FileCountByType   map[string]int `json:"fileCountByType" bson:"file_count_by_type"`     // broj datoteka po tipu
	TopAccessedFiles  []string       `json:"topAccessedFiles" bson:"top_accessed_files"`    // 5 datoteka sa najvise pristupa
	SharedWithMeCount int            `json:"sharedWithMeCount" bson:"shared_with_me_count"` // broj datoteka podeljenih sa korisnikom
	UpdatedAt         time.Time      `json:"updatedAt" bson:"updated_at"`
}
