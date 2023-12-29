package models

import (
	"context"
	"errors"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/storage"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const UserCollectionName = "user"

type UserModel struct {
	db         *mongo.Database
	collection *mongo.Collection
}

func NewUserModel(db *mongo.Database) *UserModel {
	return &UserModel{
		db:         db,
		collection: db.Collection(UserCollectionName),
	}
}

func (um *UserModel) FindByID(ctx context.Context, id primitive.ObjectID) (*UserRecord, error) {
	record := new(UserRecord)
	if err := um.collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(record); err != nil {
		return nil, err
	}

	return record, nil
}

func (um *UserModel) FindByEmail(ctx context.Context, email string) (*UserRecord, error) {
	record := new(UserRecord)
	if err := um.collection.FindOne(ctx, bson.D{{Key: "email", Value: email}}).Decode(record); err != nil {
		return nil, err
	}

	return record, nil
}

func (um *UserModel) InsertOne(ctx context.Context, record *UserRecord) (primitive.ObjectID, error) {
	result, err := um.collection.InsertOne(ctx, record)
	if err != nil {
		return primitive.ObjectID{}, err
	}

	objectID, ok := result.InsertedID.(primitive.ObjectID)

	if !ok {
		return primitive.ObjectID{}, errors.New("unable to assert type of objectID")
	}

	return objectID, nil
}

func (um *UserModel) UpdateOne(
	ctx context.Context,
	id primitive.ObjectID,
	newValues bson.D,
) (primitive.ObjectID, error) {
	filter := bson.D{{Key: "_id", Value: id}}
	update := bson.D{{Key: "$set", Value: newValues}}
	_, err := um.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return primitive.ObjectID{}, err
	}

	return id, nil
}

type UserRecord struct {
<<<<<<< HEAD:pkg/models/user_model.go
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	Email     string             `json:"email" bson:"email"`
	SsoID     string             `json:"sso_id,omitempty" bson:"sso_id,omitempty"`
	Password  string             `json:"password,omitempty" bson:"password,omitempty"`
	FirstName string             `json:"first_name,omitempty" bson:"first_name,omitempty"`
	LastName  string             `json:"last_name,omitempty" bson:"last_name,omitempty"`
	storage.Timestamps
=======
	ID        primitive.ObjectID  `json:"_id,omitempty" bson:"_id,omitempty"`
	Email     string              `json:"email" bson:"email"`
	Password  string              `json:"password" bson:"password"`
	FirstName string              `json:"first_name,omitempty" bson:"first_name,omitempty"`
	LastName  string              `json:"last_name,omitempty" bson:"last_name,omitempty"`
	CreatedAt primitive.Timestamp `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpadtedAt primitive.Timestamp `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.Timestamp `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
>>>>>>> 1d564d630e53fba521f547b28edeac0b185a31b6:pkg/models/user_collection.go
}

func NewUserRecord(email, password, firstName, lastName string) (*UserRecord, error) {
	ep, err := apiutils.EncryptPassword(password)
	if err != nil {
		return nil, err
	}

	return &UserRecord{
		ID:        primitive.NewObjectID(),
		Email:     email,
		Password:  ep,
		FirstName: firstName,
		LastName:  lastName,
		Timestamps: storage.Timestamps{
			CreatedAt: primitive.Timestamp{T: uint32(time.Now().Unix())},
			UpadtedAt: primitive.Timestamp{T: uint32(time.Now().Unix())},
		}}, nil
}
