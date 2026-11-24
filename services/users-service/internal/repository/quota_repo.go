package repository

import (
	"context"
	"errors"
	"time"

	"github.com/mvxv0/secure-file-storage/services/users-service/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrQuotaRequestNotFound = errors.New("zahtev za povecanje kvote nije pronadjen")
)

type QuotaRepository struct {
	db *mongo.Database
}

func NewQuotaRepository(db *mongo.Database) *QuotaRepository {
	return &QuotaRepository{db: db}
}

func (r *QuotaRepository) Create(ctx context.Context, req *model.QuotaRequest) error {
	req.CreatedAt = time.Now().UTC()
	req.Status = model.QuotaStatusPending

	res, err := r.db.Collection("quota_requests").InsertOne(ctx, req)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		req.ID = oid.Hex()
	}
	return nil
}

func (r *QuotaRepository) FindByID(ctx context.Context, id string) (*model.QuotaRequest, error) {
	var req model.QuotaRequest
	err := r.db.Collection("quota_requests").FindOne(ctx, buildIDFilter(id)).Decode(&req)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrQuotaRequestNotFound
		}
		return nil, err
	}
	return &req, nil
}

func (r *QuotaRepository) FindAll(ctx context.Context) ([]model.QuotaRequest, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.db.Collection("quota_requests").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	requests := make([]model.QuotaRequest, 0)
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, err
	}
	return requests, nil
}

func (r *QuotaRepository) FindByUserID(ctx context.Context, userID string) ([]model.QuotaRequest, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.db.Collection("quota_requests").Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	requests := make([]model.QuotaRequest, 0)
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, err
	}
	return requests, nil
}

func (r *QuotaRepository) HasPendingForUser(ctx context.Context, userID string) (bool, error) {
	count, err := r.db.Collection("quota_requests").CountDocuments(ctx, bson.M{
		"user_id": userID,
		"status":  model.QuotaStatusPending,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *QuotaRepository) UpdateStatus(ctx context.Context, id string, status model.QuotaRequestStatus, resolvedBy string) (*model.QuotaRequest, error) {
	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"status":      status,
			"resolved_by": resolvedBy,
			"resolved_at": now,
		},
	}
	res, err := r.db.Collection("quota_requests").UpdateOne(ctx, buildIDFilter(id), update)
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return nil, ErrQuotaRequestNotFound
	}
	return r.FindByID(ctx, id)
}

