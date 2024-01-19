package fixtures

import (
	"context"
	"fmt"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/models"
	featureflagmodel "github.com/Roll-Play/togglelabs/pkg/models/feature_flag"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var revisionCounter = 0

func CreateFeatureFlag(
	userID,
	organizationID primitive.ObjectID,
	name string,
	version int,
	flagType featureflagmodel.FlagType,
	revision []featureflagmodel.Revision,
	environments []featureflagmodel.FeatureFlagEnvironment,
	tags []string,
	db *mongo.Database,
) *featureflagmodel.FeatureFlagRecord {
	model := featureflagmodel.New(db)
	if name == "" {
		name = "feature"
	}

	if environments == nil {
		environments = []featureflagmodel.FeatureFlagEnvironment{
			{
				Name:      "prod",
				IsEnabled: true,
			},
		}
	}

	if revision == nil {
		revision = []featureflagmodel.Revision{
			*CreateRevision(userID, featureflagmodel.Draft, primitive.NilObjectID),
		}
	}

	if tags == nil {
		tags = []string{}
	}

	record := &featureflagmodel.FeatureFlagRecord{
		OrganizationID: organizationID,
		UserID:         userID,
		Version:        version,
		Name:           name,
		Type:           flagType,
		Revisions:      revision,
		Tags:           tags,
		Timestamps: models.Timestamps{
			CreatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
		},
		Environments: environments,
	}
	_, err := model.InsertOne(context.Background(), record)
	if err != nil {
		panic(err)
	}
	return record
}

func CreateRevision(
	userID primitive.ObjectID,
	status featureflagmodel.RevisionStatus,
	lastRevisionID primitive.ObjectID,
) *featureflagmodel.Revision {
	revisionCounter++
	return &featureflagmodel.Revision{
		ID:             primitive.NewObjectID(),
		UserID:         userID,
		Status:         status,
		DefaultValue:   fmt.Sprintf("default value %d", revisionCounter),
		LastRevisionID: lastRevisionID,
		Rules: []featureflagmodel.Rule{
			{
				Predicate: fmt.Sprintf("predicate %d", revisionCounter),
				Value:     fmt.Sprintf("rule value %d", revisionCounter),
				Env:       fmt.Sprintf("rule env %d", revisionCounter),
				IsEnabled: false,
			},
		},
	}
}
