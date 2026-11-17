package repository

import (
	"context"
	"errors"
	"time"

	"github.com/mvxv0/secure-file-storage/services/files-service/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrFileNotFound      = errors.New("datoteka nije pronadjena")
	ErrFileAlreadyExists = errors.New("datoteka sa ovim nazivom vec postoji u izabranom folderu")
)

type FileRepository struct {
	db *mongo.Database
}

func NewFileRepository(db *mongo.Database) *FileRepository {
	return &FileRepository{db: db}
}

// Create upisuje metapodatke nove datoteke u MongoDB kolekciju 'files'
func (r *FileRepository) Create(ctx context.Context, file *model.FileMetadata) error {
	now := time.Now().UTC()
	file.CreatedAt = now
	file.UpdatedAt = now

	result, err := r.db.Collection("files").InsertOne(ctx, file)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrFileAlreadyExists
		}
		return err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		file.ID = oid.Hex()
	}
	return nil
}

// UpdateVersion azurira metapodatke datoteke pri otpremanju nove verzije
func (r *FileRepository) UpdateVersion(ctx context.Context, file *model.FileMetadata) error {
	oid, err := primitive.ObjectIDFromHex(file.ID)
	var filter bson.M
	if err == nil {
		filter = bson.M{"_id": oid}
	} else {
		filter = bson.M{"_id": file.ID}
	}

	file.UpdatedAt = time.Now().UTC()

	update := bson.M{
		"$set": bson.M{
			"mime_type":   file.MimeType,
			"size":        file.Size,
			"content_ref": file.ContentRef,
			"version":     file.Version,
			"checksum":    file.Checksum,
			"updated_at":  file.UpdatedAt,
		},
	}

	res, err := r.db.Collection("files").UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrFileNotFound
	}
	return nil
}

// FindByID pronalazi metapodatke datoteke po ID-ju
func (r *FileRepository) FindByID(ctx context.Context, id string) (*model.FileMetadata, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	var filter bson.M
	if err == nil {
		filter = bson.M{"_id": oid}
	} else {
		filter = bson.M{"_id": id}
	}

	var file model.FileMetadata
	err = r.db.Collection("files").FindOne(ctx, filter).Decode(&file)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	return &file, nil
}

// FindByNameAndFolder trazi datoteku po nazivu unutar datog foldera za datog vlasnika
func (r *FileRepository) FindByNameAndFolder(ctx context.Context, ownerID string, parentFolderID *string, name string) (*model.FileMetadata, error) {
	filter := bson.M{
		"owner_id":         ownerID,
		"parent_folder_id": parentFolderID,
		"name":             name,
	}

	var file model.FileMetadata
	err := r.db.Collection("files").FindOne(ctx, filter).Decode(&file)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &file, nil
}

// FindByFolder vraca sve datoteke unutar izabranog foldera (ili u root-u ako je parentFolderID == nil)
func (r *FileRepository) FindByFolder(ctx context.Context, ownerID string, parentFolderID *string) ([]model.FileMetadata, error) {
	filter := bson.M{
		"owner_id":         ownerID,
		"parent_folder_id": parentFolderID,
	}

	cursor, err := r.db.Collection("files").Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	files := make([]model.FileMetadata, 0)
	if err := cursor.All(ctx, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// GetTotalUsedBytes racuna ukupno zauzece skladista za datog korisnika (zbir velicina svih njegovih datoteka)
func (r *FileRepository) GetTotalUsedBytes(ctx context.Context, ownerID string) (int64, error) {
	cursor, err := r.db.Collection("files").Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var total int64
	for cursor.Next(ctx) {
		var file model.FileMetadata
		if err := cursor.Decode(&file); err == nil {
			total += file.Size
		}
	}
	return total, nil
}
