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

const OrganizationCollectionName = "organization"

type OrganizationModel struct {
	db         *mongo.Database
	collection *mongo.Collection
}

func NewOrganizationModel(db *mongo.Database) *OrganizationModel {
	return &OrganizationModel{
		db:         db,
		collection: db.Collection(OrganizationCollectionName),
	}
}

func (om *OrganizationModel) FindByID(ctx context.Context, id primitive.ObjectID) (*OrganizationRecord, error) {
	record := new(OrganizationRecord)
	if err := om.collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(record); err != nil {
		return nil, err
	}

	return record, nil
}

func (om *OrganizationModel) InsertOne(ctx context.Context, record *OrganizationRecord) (primitive.ObjectID, error) {
	record.ID = primitive.NewObjectID()
	result, err := om.collection.InsertOne(ctx, record)
	if err != nil {
		return primitive.NilObjectID, err
	}

	objectID, ok := result.InsertedID.(primitive.ObjectID)

	if !ok {
		return primitive.NilObjectID, errors.New("unable to assert type of objectID")
	}

	return objectID, nil
}

type PermissionLevelEnum = string

const (
	Admin        PermissionLevelEnum = "ADMIN"
	Collaborator PermissionLevelEnum = "COLLABORATOR"
	ReadOnly     PermissionLevelEnum = "READ_ONLY"
)

type OrganizationMember struct {
	User            UserRecord          `json:"user" bson:"user"`
	PermissionLevel PermissionLevelEnum `json:"permission_level" bson:"permission_level"`
}

type OrganizationInviteStatus = string

const (
	Pending   OrganizationInviteStatus = "PENDING"
	Accepted  OrganizationInviteStatus = "ACCEPTED"
	Denied    OrganizationInviteStatus = "DENIED"
	Cancelled OrganizationInviteStatus = "CANCELED"
)

type OrganizationInvite struct {
	Email  string
	Status OrganizationInviteStatus
}

type OrganizationRecord struct {
	ID           primitive.ObjectID   `json:"_id" bson:"_id"`
	Name         string               `json:"name" bson:"name"`
	Members      []OrganizationMember `json:"members" bson:"members"`
	Invites      []OrganizationInvite `json:"invites" bson:"invites"`
	Environments []Environment
	storage.Timestamps
}

type Environment struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
}

func NewOrganizationRecord(name string, members []OrganizationMember) *OrganizationRecord {
	return &OrganizationRecord{
		Name:    name,
		Members: members,
		Timestamps: storage.Timestamps{
			CreatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
		},
	}
}
