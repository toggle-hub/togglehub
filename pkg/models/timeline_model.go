package models

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const TimelineCollectionName = "timeline"

const (
	Created             = "FeatureFlag creted"
	RevisionCreated     = "Revision created"
	RevisionApproved    = "Revision approved"
	FeatureFlagRollback = "FeatureFlag rollback"
	FeatureFlagDeleted  = "FeatureFlag deleted"
	FeatureFlagToggle   = "FeatureFlag environment %s toggle"
)

type TimelineModel struct {
	db         *mongo.Database
	collection *mongo.Collection
}

func NewTimelineModel(db *mongo.Database) *TimelineModel {
	return &TimelineModel{
		db:         db,
		collection: db.Collection(TimelineCollectionName),
	}
}

type TimelineEntry struct {
	UserID    primitive.ObjectID `json:"user_id" bson:"user_id"`
	Action    string             `json:"action" bson:"action"`
	Timestamp primitive.DateTime `json:"timestamp" bson:"timestamp"`
}

type TimelineRecord struct {
	ID            primitive.ObjectID `json:"_id" bson:"_id"`
	FeatureFlagID primitive.ObjectID `json:"feature_flag_id" bson:"feature_flag_id"`
	Entries       []TimelineEntry
}

func NewTimelineEntry(userID primitive.ObjectID, action string) *TimelineEntry {
	return &TimelineEntry{
		UserID:    userID,
		Action:    action,
		Timestamp: primitive.NewDateTimeFromTime(time.Now().UTC()),
	}
}

func (tm *TimelineModel) InsertOne(ctx context.Context, record *TimelineRecord) (primitive.ObjectID, error) {
	record.ID = primitive.NewObjectID()
	result, err := tm.collection.InsertOne(ctx, record)
	if err != nil {
		return primitive.NilObjectID, err
	}

	objectID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("unable to assert type of objectID")
	}

	return objectID, nil
}

func (tm *TimelineModel) UpdateOne(
	ctx context.Context,
	featureFlagID primitive.ObjectID,
	entry *TimelineEntry,
) error {
	_, err := tm.collection.UpdateOne(
		ctx,
		bson.D{{Key: "feature_flag_id", Value: featureFlagID}},
		bson.D{{Key: "$push", Value: bson.M{"entries": entry}}},
	)
	if err != nil {
		return err
	}

	return nil
}

func (ffm *TimelineModel) FindByID(ctx context.Context, id primitive.ObjectID) (*TimelineRecord, error) {
	record := new(TimelineRecord)
	if err := ffm.collection.FindOne(ctx, bson.D{{Key: "feature_flag_id", Value: id}}).Decode(record); err != nil {
		return nil, err
	}
	return record, nil
}
