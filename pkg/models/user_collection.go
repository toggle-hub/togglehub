package models

import (
	"context"
	"errors"
	"time"

	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const UserCollectionName = "user"

type UserModel struct {
	collection *mongo.Collection
}

type KeyValue struct {
	Key, Value string
}

func NewUserModel(collection *mongo.Collection) *UserModel {
	return &UserModel{
		collection: collection,
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
	newValues ...KeyValue,
) (primitive.ObjectID, error) {
	filter := bson.D{{Key: "_id", Value: id}}
	var fields []bson.E
	for _, v := range newValues {
		fields = append(fields, bson.E{Key: v.Key, Value: v.Value})
	}
	update := bson.D{{Key: "$set", Value: fields}}
	_, err := um.collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return primitive.ObjectID{}, err
	}

	return id, nil
}

type UserRecord struct {
	ID        primitive.ObjectID  `json:"_id,omitempty" bson:"_id,omitempty"`
	Email     string              `json:"email" bson:"email"`
	Password  string              `json:"password" bson:"password"`
	FirstName string              `json:"first_name" bson:"first_name"`
	LastName  string              `json:"last_name" bson:"last_name"`
	CreatedAt primitive.Timestamp `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpadtedAt primitive.Timestamp `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.Timestamp `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

func NewUserRecord(email, password, firstName, lastName string) (*UserRecord, error) {
	ep, err := apiutils.EncryptPassword(password)
	if err != nil {
		return nil, err
	}

	return &UserRecord{
		Email:     email,
		Password:  ep,
		FirstName: firstName,
		LastName:  lastName,
		CreatedAt: primitive.Timestamp{T: uint32(time.Now().Unix())},
		UpadtedAt: primitive.Timestamp{T: uint32(time.Now().Unix())},
	}, nil
}
