package usermodel

import (
	"context"
	"errors"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const UserCollectionName = "user"

type UserModel struct {
	db         *mongo.Database
	collection *mongo.Collection
}

func New(db *mongo.Database) *UserModel {
	return &UserModel{
		db:         db,
		collection: db.Collection(UserCollectionName),
	}
}

type UserOrganization struct {
	ID   primitive.ObjectID `json:"_id" bson:"_id"`
	Name string             `json:"name" bson:"name"`
}

type UserWithOrganization struct {
	ID            primitive.ObjectID `json:"_id" bson:"_id"`
	OAuthID       string             `json:"oauth_id,omitempty" bson:"oauth_id,omitempty"`
	Email         string             `json:"email" bson:"email"`
	FirstName     string             `json:"first_name,omitempty" bson:"first_name,omitempty"`
	LastName      string             `json:"last_name,omitempty" bson:"last_name,omitempty"`
	Organizations []UserOrganization `json:"organizations" bson:"organizations"`
}

func (um *UserModel) FindUserOrganization(ctx context.Context, id primitive.ObjectID) (*UserWithOrganization, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"_id": id}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "organization",
			"localField":   "_id",
			"foreignField": "members.user._id",
			"as":           "organizations",
		}}},
		{{Key: "$project", Value: bson.M{
			"_id":        1,
			"email":      1,
			"oauth_id":   1,
			"first_name": 1,
			"last_name":  1,
			"organizations": bson.M{
				"_id":  1,
				"name": 1,
			},
		}}},
	}

	user := new(UserWithOrganization)
	cursor, err := um.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	cursor.Next(ctx)
	if err := cursor.Decode(user); err != nil {
		return nil, err
	}

	return user, nil
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
	record.ID = primitive.NewObjectID()
	result, err := um.collection.InsertOne(ctx, record)
	if err != nil {
		return primitive.NilObjectID, err
	}

	objectID, ok := result.InsertedID.(primitive.ObjectID)

	if !ok {
		return primitive.NilObjectID, errors.New("unable to assert type of objectID")
	}

	return objectID, nil
}

func (um *UserModel) UpdateOne(
	ctx context.Context,
	id primitive.ObjectID,
	update bson.D,
) error {
	filter := bson.D{{Key: "_id", Value: id}}
	update = append(update, bson.E{
		Key:   "timestamps.updated_at",
		Value: primitive.NewDateTimeFromTime(time.Now().UTC()),
	})

	_, err := um.collection.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: update}})

	return err
}

type UserRecord struct {
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	Email     string             `json:"email" bson:"email"`
	OAuthID   string             `json:"oauth_id,omitempty" bson:"oauth_id,omitempty"`
	Password  string             `json:"password,omitempty" bson:"password,omitempty"`
	FirstName string             `json:"first_name,omitempty" bson:"first_name,omitempty"`
	LastName  string             `json:"last_name,omitempty" bson:"last_name,omitempty"`
	models.Timestamps
}

func NewUserRecord(email, password, firstName, lastName string) (*UserRecord, error) {
	ep, err := encryptPassword(password)
	if err != nil {
		return nil, err
	}

	return &UserRecord{
		Email:     email,
		Password:  ep,
		FirstName: firstName,
		LastName:  lastName,
		Timestamps: models.Timestamps{
			CreatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
		}}, nil
}

func encryptPassword(password string) (string, error) {
	encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), config.BCryptCost)
	if err != nil {
		return "", err
	}

	return string(encryptedPassword), nil
}
