package models

import (
	"context"
	"errors"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	Predicate string             `json:"predicate" bson:"predicate" validate:"required"`
	Value     string             `json:"value" bson:"value" validate:"required"`
	Env       string             `json:"env" bson:"env" validate:"required"`
	IsEnabled bool               `json:"is_enabled" bson:"is_enabled" validate:"required,boolean"`
}

type Revision struct {
	ID             primitive.ObjectID `json:"_id,omitempty" bson:"_id"`
	UserID         primitive.ObjectID `json:"user_id" bson:"user_id"`
	Status         RevisionStatus     `json:"status" bson:"status"`
	DefaultValue   string             `json:"default_value" bson:"default_value"`
	LastRevisionID primitive.ObjectID `json:"last_revision_id,omitempty" bson:"last_revision_id,omitempty"`
	ChangeSet      string             `json:"change_set,omitempty" bson:"change_set,omitempty"`
	Rules          []Rule
}

type FlagType = string

const (
	Boolean FlagType = "boolean"
	JSON    FlagType = "json"
	String  FlagType = "string"
	Number  FlagType = "number"
)

type FeatureFlagRecord struct {
	ID             primitive.ObjectID `json:"_id,omitempty" bson:"_id"`
	OrganizationID primitive.ObjectID `json:"organization_id" bson:"organization_id"`
	UserID         primitive.ObjectID `json:"user_id" bson:"user_id"`
	Version        int                `json:"version" bson:"version"`
	Name           string             `json:"name" bson:"name"`
	Type           FlagType           `json:"type" bson:"type"`
	Revisions      []Revision         `json:"revisions" bson:"revisions"`
	Timeline       string             `json:"timeline" bson:"timeline"`
	Enviroment     []FeatureFlagEnviroment
	storage.Timestamps
}

type FeatureFlagEnviroment struct {
	Name      string `json:"name" bson:"name"`
	IsEnabled bool   `json:"is_enabled" bson:"is_enabled"`
}

func NewFeatureFlagRecord(
	name,
	defaultValue string,
	flagType FlagType,
	rules []Rule,
	organizationID,
	userID primitive.ObjectID,
) *FeatureFlagRecord {
	return &FeatureFlagRecord{
		OrganizationID: organizationID,
		UserID:         userID,
		Version:        1,
		Name:           name,
		Type:           flagType,
		Revisions: []Revision{
			{
				ID:           primitive.NewObjectID(),
				UserID:       userID,
				Status:       Live,
				DefaultValue: defaultValue,
				Rules:        rules,
			},
		},
		Timestamps: storage.Timestamps{
			CreatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
		},
	}
}

func NewRevisionRecord(defaultValue string, rules []Rule, userID primitive.ObjectID) *Revision {
	return &Revision{
		ID:           primitive.NewObjectID(),
		UserID:       userID,
		Status:       Draft,
		DefaultValue: defaultValue,
		Rules:        rules,
	}
}

func (ffm *FeatureFlagModel) InsertOne(ctx context.Context, record *FeatureFlagRecord) (primitive.ObjectID, error) {
	record.ID = primitive.NewObjectID()
	result, err := ffm.collection.InsertOne(ctx, record)
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
	if err := ffm.collection.FindOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "deleted_at", Value: bson.M{
			"$exists": false},
		}}).Decode(record); err != nil {
		return nil, err
	}
	return record, nil
}

var EmptyFeatureRecordList = []FeatureFlagRecord{}

func (ffm *FeatureFlagModel) FindMany(
	ctx context.Context,
	organizationID primitive.ObjectID,
	page,
	limit int,
) ([]FeatureFlagRecord, error) {
	findOptions := options.Find()
	findOptions.SetSkip(int64((page - 1) * limit))
	findOptions.SetLimit(int64(limit))

	records := make([]FeatureFlagRecord, 0)
	cursor, err := ffm.collection.Find(ctx, bson.D{
		{Key: "organization_id", Value: organizationID},
		{Key: "deleted_at", Value: bson.M{
			"$exists": false},
		}}, findOptions)
	if err != nil {
		return EmptyFeatureRecordList, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		record := new(FeatureFlagRecord)
		if err := cursor.Decode(record); err != nil {
			return EmptyFeatureRecordList, err
		}

		records = append(records, *record)
	}

	return records, nil
}

func (ffm *FeatureFlagModel) UpdateOne(
	ctx context.Context,
	filter,
	update bson.D,
) (primitive.ObjectID, error) {
	_, err := ffm.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return primitive.ObjectID{}, err
	}

	return primitive.NilObjectID, nil
}

func (ffm *FeatureFlagModel) FindOne(
	ctx context.Context,
	filter bson.D,
) (*FeatureFlagRecord, error) {
	record := new(FeatureFlagRecord)

	err := ffm.collection.FindOne(ctx, filter).Decode(record)
	if err != nil {
		return nil, err
	}

	return record, nil
}
