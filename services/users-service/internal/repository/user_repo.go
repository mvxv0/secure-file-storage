package repository

import (
	"context"
	"errors"
	"time"

	"github.com/mvxv0/secure-file-storage/services/users-service/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrUserNotFound      = errors.New("korisnik nije pronadjen")
	ErrUserAlreadyExists = errors.New("korisnik sa ovom email adresom vec postoji")
	ErrTokenNotFound     = errors.New("token nije pronadjen ili je istekao")
)

type UserRepository struct {
	db *mongo.Database
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{db: db}
}

// FindByEmail trazi korisnika po email adresi
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.Collection("users").FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// CreateUser cuva novog korisnika u bazi
func (r *UserRepository) CreateUser(ctx context.Context, user *model.User) error {
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = time.Now().UTC()

	result, err := r.db.Collection("users").InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrUserAlreadyExists
		}
		return err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		user.ID = oid.Hex()
	}
	return nil
}

// SetUserVerified postavlja is_verified na true nakon sto korisnik unese ispravan token
func (r *UserRepository) SetUserVerified(ctx context.Context, email string) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$set": bson.M{
			"is_verified": true,
			"updated_at":  time.Now().UTC(),
		},
	}
	res, err := r.db.Collection("users").UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SaveAuthToken cuva token (za verifikaciju, otp ili reset lozinke)
func (r *UserRepository) SaveAuthToken(ctx context.Context, token *model.AuthToken) error {
	token.CreatedAt = time.Now().UTC()
	token.Used = false

	_, err := r.db.Collection("auth_tokens").InsertOne(ctx, token)
	return err
}

// FindValidToken pronalazi neiskoriscen i neistekao token za zadati email i tip
func (r *UserRepository) FindValidToken(ctx context.Context, email string, tokenType model.TokenType) (*model.AuthToken, error) {
	filter := bson.M{
		"email":      email,
		"type":       tokenType,
		"used":       false,
		"expires_at": bson.M{"$gt": time.Now().UTC()},
	}

	var token model.AuthToken
	err := r.db.Collection("auth_tokens").FindOne(ctx, filter).Decode(&token)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return &token, nil
}

// MarkTokenUsed oznacava token kao iskoriscen da se ne moze upotrebiti dva puta
func (r *UserRepository) MarkTokenUsed(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	var filter bson.M
	if err == nil {
		filter = bson.M{"_id": oid}
	} else {
		filter = bson.M{"_id": id}
	}

	_, err = r.db.Collection("auth_tokens").UpdateOne(ctx, filter, bson.M{
		"$set": bson.M{"used": true},
	})
	return err
}

