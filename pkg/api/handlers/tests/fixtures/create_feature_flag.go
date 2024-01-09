package fixtures

import (
	"context"
	"fmt"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/models"
	"github.com/Roll-Play/togglelabs/pkg/storage"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var revisionCounter = 0

func CreateFeatureFlag(
	userID,
	organizationID primitive.ObjectID,
	name string,
	version int,
	flagType models.FlagType,
	revision []models.Revision,
	db *mongo.Database,
) *models.FeatureFlagRecord {
	model := models.NewFeatureFlagModel(db)
	if name == "" {
		name = "feature"
	}
	if revision == nil {
		revision = []models.Revision{
			*CreateRevision(userID, models.Draft, primitive.NilObjectID),
		}
	}
	record := &models.FeatureFlagRecord{
		OrganizationID: organizationID,
		UserID:         userID,
		Version:        version,
		Name:           name,
		Type:           flagType,
		Revisions:      revision,
		Timestamps: storage.Timestamps{
			CreatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
		},
	}
	_, err := model.InsertOne(context.Background(), record)
	if err != nil {
		panic(err)
	}
	return record
}

func CreateRevision(
	userId primitive.ObjectID,
	status models.RevisionStatus,
	lastRevisionID primitive.ObjectID,
) *models.Revision {
	revisionCounter++
	return &models.Revision{
		ID:             primitive.NewObjectID(),
		UserID:         userId,
		Status:         status,
		DefaultValue:   fmt.Sprintf("default value %d", revisionCounter),
		LastRevisionID: lastRevisionID,
		Rules: []models.Rule{
			{
				Predicate: fmt.Sprintf("predicate %d", revisionCounter),
				Value:     fmt.Sprintf("rule value %d", revisionCounter),
				Env:       fmt.Sprintf("rule env %d", revisionCounter),
				IsEnabled: false,
			},
		},
	}
}
