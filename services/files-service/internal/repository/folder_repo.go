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
	ErrFolderNotFound      = errors.New("folder nije pronadjen")
	ErrFolderAlreadyExists = errors.New("folder sa ovim nazivom vec postoji na ovoj lokaciji")
)

type FolderRepository struct {
	db *mongo.Database
}

func NewFolderRepository(db *mongo.Database) *FolderRepository {
	return &FolderRepository{db: db}
}

// Create kreira novi folder u bazi
func (r *FolderRepository) Create(ctx context.Context, folder *model.Folder) error {
	folder.CreatedAt = time.Now().UTC()
	folder.UpdatedAt = time.Now().UTC()

	result, err := r.db.Collection("folders").InsertOne(ctx, folder)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrFolderAlreadyExists
		}
		return err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		folder.ID = oid.Hex()
	}
	return nil
}

// FindByID pronalazi folder po njegovom ID-ju
func (r *FolderRepository) FindByID(ctx context.Context, id string) (*model.Folder, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	var filter bson.M
	if err == nil {
		filter = bson.M{"_id": oid}
	} else {
		filter = bson.M{"_id": id}
	}

	var folder model.Folder
	err = r.db.Collection("folders").FindOne(ctx, filter).Decode(&folder)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrFolderNotFound
		}
		return nil, err
	}
	return &folder, nil
}

// FindByNameAndParent proverava da li korisnik vec ima folder sa tim imenom u tom folderu
func (r *FolderRepository) FindByNameAndParent(ctx context.Context, ownerID string, parentFolderID *string, name string) (*model.Folder, error) {
	filter := bson.M{
		"owner_id":         ownerID,
		"parent_folder_id": parentFolderID,
		"name":             name,
	}

	var folder model.Folder
	err := r.db.Collection("folders").FindOne(ctx, filter).Decode(&folder)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // ne postoji, slobodno je
		}
		return nil, err
	}
	return &folder, nil
}

// FindSubfolders vraca sve direktne podfoldere unutar nekog foldera (ili u root-u ako je parentFolderID == nil)
func (r *FolderRepository) FindSubfolders(ctx context.Context, ownerID string, parentFolderID *string) ([]model.Folder, error) {
	filter := bson.M{
		"owner_id":         ownerID,
		"parent_folder_id": parentFolderID,
	}

	cursor, err := r.db.Collection("folders").Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	folders := make([]model.Folder, 0)
	if err := cursor.All(ctx, &folders); err != nil {
		return nil, err
	}
	return folders, nil
}

