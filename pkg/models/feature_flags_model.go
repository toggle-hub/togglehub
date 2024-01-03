package models

import (
	"context"
	"errors"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const FeatureFlagCollectionName = "feature_flag"

type FeatureFlagModel struct {
	db         *mongo.Database
	collection *mongo.Collection
}

func NewFeatureFlagModel(db *mongo.Database) *FeatureFlagModel {
	return &FeatureFlagModel{
		db:         db,
		collection: db.Collection(FeatureFlagCollectionName),
	}
}

type RevisionStatus = string

const (
	Live     RevisionStatus = "live"
	Draft    RevisionStatus = "draft"
	Archived RevisionStatus = "archived"
)

type Rule struct {
	Predicate string `json:"predicate" bson:"predicate"`
	Value     string `json:"value" bson:"value"`
	Env       string `json:"env" bson:"env"`
	IsEnabled bool   `json:"is_enabled" bson:"is_enabled"`
}

type Revision struct {
	UserID       primitive.ObjectID `json:"user_id" bson:"user_id"`
	Status       RevisionStatus     `json:"status" bson:"status"`
	DefaultValue string             `json:"default_value" bson:"default_value"`
	Rules        []Rule
}

type FlagType = string

const (
	Boolean FlagType = "boolean"
	JSON    FlagType = "json"
	String  FlagType = "string"
	Number  FlagType = "number"
)

type FeatureFlagRecord struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id"`
	OrgID     primitive.ObjectID `json:"org_id" bson:"org_id"`
	UserID    primitive.ObjectID `json:"user_id" bson:"user_id"`
	Version   int                `json:"version" bson:"version"`
	Type      FlagType           `json:"type" bson:"type"`
	Revisions []Revision
	storage.Timestamps
}

type FeatureFlagRequest struct {
	Type         FlagType `json:"type" `
	DefaultValue string   `json:"default_value"`
	Rules        []Rule
}

type RevisionRequest struct {
	DefaultValue string `json:"default_value"`
	Rules        []Rule
}

func NewFeatureFlagRecord(
	req *FeatureFlagRequest,
	orgID, userID primitive.ObjectID,
) *FeatureFlagRecord {
	return &FeatureFlagRecord{
		OrgID:   orgID,
		UserID:  userID,
		Version: 1,
		Type:    req.Type,
		Revisions: []Revision{
			{
				UserID:       userID,
				Status:       Draft,
				DefaultValue: req.DefaultValue,
				Rules:        req.Rules,
			},
		},
		Timestamps: storage.Timestamps{
			CreatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
			UpadtedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
		},
	}
}

func (ffm *FeatureFlagModel) NewRevisionRecord(req *RevisionRequest, userID primitive.ObjectID) *Revision {
	return &Revision{
		UserID:       userID,
		Status:       Draft,
		DefaultValue: req.DefaultValue,
		Rules:        req.Rules,
	}
}

func (ffm *FeatureFlagModel) InsertOne(ctx context.Context, rec *FeatureFlagRecord) (primitive.ObjectID, error) {
	rec.ID = primitive.NewObjectID()
	result, err := ffm.collection.InsertOne(ctx, rec)
	if err != nil {
		return primitive.NilObjectID, err
	}

	objectID, ok := result.InsertedID.(primitive.ObjectID)

	if !ok {
		return primitive.NilObjectID, errors.New("unable to assert type of objectID")
	}

	return objectID, nil
}

func (ffm *FeatureFlagModel) FindByID(ctx context.Context, id primitive.ObjectID) (*FeatureFlagRecord, error) {
	record := new(FeatureFlagRecord)
	if err := ffm.collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(record); err != nil {
		return nil, err
	}
	return record, nil
}

func (ffm *FeatureFlagModel) UpdateOne(
	ctx context.Context,
	id primitive.ObjectID,
	newValues bson.M,
) (primitive.ObjectID, error) {
	filter := bson.D{{Key: "_id", Value: id}}
	update := bson.D{{Key: "$push", Value: newValues}}
	_, err := ffm.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return primitive.ObjectID{}, err
	}

	return id, nil
}
