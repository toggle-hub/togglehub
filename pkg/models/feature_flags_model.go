package models

import (
	"context"
	"errors"

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

type Environment struct {
	Name      string `json:"name" bson:"name"`
	IsEnabled bool   `json:"is_enabled" bson:"is_enabled"`
}

type RevisionRule struct {
	Attributes map[string]string `json:"attributes" bson:"attributes"`
}

type Revision struct {
	Creator      primitive.ObjectID `json:"creator_id" bson:"creator_id"`
	Version      int                `json:"version" bson:"version"`
	Status       RevisionStatus     `json:"status" bson:"status"`
	DefaultValue string             `json:"default_value" bson:"default_value"`
	Environments []Environment
	Rules        []RevisionRule
}

type FlagType = string

const (
	Boolean FlagType = "boolean"
	Json    FlagType = "json"
	String  FlagType = "string"
	Number  FlagType = "number"
)

type FeatureFlagRecord struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id"`
	OrgID     primitive.ObjectID `json:"org_id" bson:"org_id"`
	CreatorID primitive.ObjectID `json:"creator_id" bson:"creator_id"`
	Type      FlagType           `json:"type" bson:"type"`
	Revisions []Revision
}

type FeatureFlagRequest struct {
	Type         FlagType `json:"type" `
	DefaultValue string   `json:"default_value"`
	Environments Environment
	Rules        []RevisionRule
}

func (ffm *FeatureFlagModel) NewFeatureFlagRecord(
	req *FeatureFlagRequest,
	orgID, creatorID primitive.ObjectID,
) (*FeatureFlagRecord, error) {
	return &FeatureFlagRecord{
		OrgID:     orgID,
		CreatorID: creatorID,
		Type:      req.Type,
		Revisions: []Revision{
			Revision{
				Creator:      creatorID,
				Version:      1,
				Status:       Draft,
				DefaultValue: req.DefaultValue,
				Environments: []Environment{
					req.Environments,
				},
				Rules: req.Rules,
			},
		},
	}, nil
}

func (ffm *FeatureFlagModel) InsertOne(ctx context.Context, rec *FeatureFlagRecord) (primitive.ObjectID, error) {
	result, err := ffm.collection.InsertOne(ctx, rec)
	if err != nil {
		return primitive.ObjectID{}, err
	}

	objectId, ok := result.InsertedID.(primitive.ObjectID)

	if !ok {
		return primitive.ObjectID{}, errors.New("unable to assert type of objectID")
	}

	return objectId, nil
}
