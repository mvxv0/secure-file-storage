package model

import "time"

// Folder predstavlja metapodatke o folderu u MongoDB kolekciji 'folders'
type Folder struct {
	ID             string    `json:"id" bson:"_id,omitempty"`
	Name           string    `json:"name" bson:"name"`
	OwnerID        string    `json:"ownerId" bson:"owner_id"`
	ParentFolderID *string   `json:"parentFolderId,omitempty" bson:"parent_folder_id,omitempty"` // nil za korenski (root) folder
	CreatedAt      time.Time `json:"createdAt" bson:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" bson:"updated_at"`
}

// FileMetadata cuva metapodatke o datoteci u MongoDB kolekciji 'files'
type FileMetadata struct {
	ID             string    `json:"id" bson:"_id,omitempty"`
	Name           string    `json:"name" bson:"name"`
	MimeType       string    `json:"mimeType" bson:"mime_type"`                                  // tip datoteke
	Size           int64     `json:"size" bson:"size"`                                           // velicina u bajtovima
	OwnerID        string    `json:"ownerId" bson:"owner_id"`                                    // vlasnik datoteke
	ParentFolderID *string   `json:"parentFolderId,omitempty" bson:"parent_folder_id,omitempty"` // roditeljski folder
	ContentRef     string    `json:"contentRef" bson:"content_ref"`                              // referenca na sadrzaj datoteke
	Version        int       `json:"version" bson:"version"`                                     // trenutna verzija datoteke
	Checksum       string    `json:"checksum,omitempty" bson:"checksum,omitempty"`               // hes za proveru integriteta (2.18)
	CreatedAt      time.Time `json:"createdAt" bson:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" bson:"updated_at"` // vreme poslednje izmene
}
