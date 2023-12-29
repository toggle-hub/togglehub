package models

import (
	"context"

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

// func (om *OrganizationModel) InsertOne(ctx context.Context, record *OrganizationRecord, adminID primitive.ObjectID) (primitive.ObjectID, error) {

// }

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
	Pending  OrganizationInviteStatus = "PENDING"
	Accepted OrganizationInviteStatus = "ACCEPTED"
	Denied   OrganizationInviteStatus = "DENIED"
	CANCELED OrganizationInviteStatus = "CANCELED"
)

type OrganizationInvite struct {
	Email  string
	Status OrganizationInviteStatus
}

type OrganizationRecord struct {
	ID      primitive.ObjectID `json:"_id" bson:"_id"`
	Name    string
	Members []OrganizationMember `json:"members" bson:"members"`
	Invites []OrganizationInvite `json:"invites" bson:"invites"`
	storage.Timestamps
}
