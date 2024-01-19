package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	featureflagmodel "github.com/Roll-Play/togglelabs/pkg/models/feature_flag"
	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	timelinemodel "github.com/Roll-Play/togglelabs/pkg/models/timeline"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type FeatureFlagHandler struct {
	db     *mongo.Database
	logger *zap.Logger
}

func NewFeatureFlagHandler(db *mongo.Database, logger *zap.Logger) *FeatureFlagHandler {
	return &FeatureFlagHandler{
		db:     db,
		logger: logger,
	}
}

type PostFeatureFlagRequest struct {
	Name         string                    `json:"name" validate:"required"`
	Type         featureflagmodel.FlagType `json:"type" validate:"required,oneof=boolean json string number"`
	DefaultValue string                    `json:"default_value" validate:"required"`
	Rules        []featureflagmodel.Rule   `json:"rules" validate:"dive,required"`
	Tags         []string                  `json:"tags"`
	Environment  string                    `json:"environment" validate:"required"`
	ProjectName  string                    `json:"project_name"`
}

type PatchFeatureFlagRequest struct {
	DefaultValue string                  `json:"default_value"`
	Rules        []featureflagmodel.Rule `json:"rules" validate:"dive,required"`
}

type PatchFeatureFlagTagsRequest struct {
	Tags []string `json:"tags"`
}

type ListFeatureFlagResponse struct {
	Data     []featureflagmodel.FeatureFlagRecord `json:"data"`
	Page     int                                  `json:"page"`
	PageSize int                                  `json:"page_size"`
	Total    int                                  `json:"total"`
}

func (ffh *FeatureFlagHandler) ListFeatureFlags(c echo.Context) error {
	pageQuery := c.QueryParam("page")
	limitQuery := c.QueryParam("page_size")

	page, limit := apiutils.GetPaginationParams(pageQuery, limitQuery)

	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(ffh.db)
	organization, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organization, organizationmodel.ReadOnly)
	if !permission {
		ffh.logger.Debug("Client error",
			zap.Error(errors.New(apierrors.ForbiddenError)),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	model := featureflagmodel.New(ffh.db)

	featureFlags, err := model.FindMany(context.Background(), organizationID, page, limit, bson.D{{
		Key:   "timestamps.created_at",
		Value: -1,
	}})
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, ListFeatureFlagResponse{
		Data:     featureFlags,
		Page:     page,
		PageSize: limit,
		Total:    len(featureFlags),
	})
}

func (ffh *FeatureFlagHandler) PostFeatureFlag(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		ffh.logger.Debug("Client error",
			zap.Error(errors.New(apierrors.ForbiddenError)),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	request := new(PostFeatureFlagRequest)
	if err := c.Bind(request); err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(request); err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	featureFlagModel := featureflagmodel.New(ffh.db)
	featureFlagRecord := featureflagmodel.NewFeatureFlagRecord(
		request.Name,
		request.DefaultValue,
		request.Type,
		request.Rules,
		organizationID,
		userID,
		request.Environment,
		request.ProjectName,
		request.Tags,
	)

	featureFlagID, err := featureFlagModel.InsertOne(context.Background(), featureFlagRecord)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	timelineModel := timelinemodel.New(ffh.db)
	_, err = timelineModel.InsertOne(context.Background(),
		&timelinemodel.TimelineRecord{
			FeatureFlagID: featureFlagID,
			Entries:       []timelinemodel.TimelineEntry{},
		})
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	timelineEntry := timelinemodel.NewTimelineEntry(
		userID,
		timelinemodel.Created,
	)
	err = timelineModel.UpdateOne(context.Background(), featureFlagID, timelineEntry)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusCreated, featureFlagRecord)
}

func (ffh *FeatureFlagHandler) PatchFeatureFlag(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		ffh.logger.Debug("Client error",
			zap.Error(errors.New(apierrors.ForbiddenError)),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	featureFlagID, err := primitive.ObjectIDFromHex(c.Param("featureFlagID"))
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	request := new(PatchFeatureFlagRequest)
	if err := c.Bind(request); err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	featureFlagModel := featureflagmodel.New(ffh.db)

	revision := featureflagmodel.NewRevisionRecord(
		request.DefaultValue,
		request.Rules,
		userID,
	)
	err = featureFlagModel.UpdateOne(
		context.Background(),
		bson.M{"$and": []bson.M{
			{"_id": featureFlagID},
			{"organization_id": organizationID},
		}},
		bson.D{{Key: "$push", Value: bson.M{"revisions": revision}}},
	)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	timelineModel := timelinemodel.New(ffh.db)
	timelineEntry := timelinemodel.NewTimelineEntry(userID, timelinemodel.RevisionCreated)
	err = timelineModel.UpdateOne(context.Background(), featureFlagID, timelineEntry)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, revision)
}

func (ffh *FeatureFlagHandler) ApproveRevision(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		ffh.logger.Debug("Server error",
			zap.Error(errors.New(apierrors.UnauthorizedError)),
		)
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	featureFlagID, err := primitive.ObjectIDFromHex(c.Param("featureFlagID"))
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	revisionID, err := primitive.ObjectIDFromHex(c.Param("revisionID"))
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := featureflagmodel.New(ffh.db)
	featureFlagRecord, err := model.FindByID(context.Background(), featureFlagID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	var lastRevisionID primitive.ObjectID
	for index, revision := range featureFlagRecord.Revisions {
		if revision.Status == featureflagmodel.Live {
			featureFlagRecord.Revisions[index].Status = featureflagmodel.Archived
			lastRevisionID = revision.ID
		}
		if revision.ID == revisionID && revision.Status == featureflagmodel.Draft {
			featureFlagRecord.Revisions[index].Status = featureflagmodel.Live
			featureFlagRecord.Revisions[index].LastRevisionID = lastRevisionID
		}
	}
	featureFlagRecord.Version++

	filters := bson.M{"$and": []bson.M{
		{"_id": featureFlagID},
		{"organization_id": organizationID},
	}}
	newValues := bson.D{
		{
			Key: "$set", Value: bson.D{
				{Key: "version", Value: featureFlagRecord.Version},
				{Key: "revisions", Value: featureFlagRecord.Revisions},
			},
		},
	}
	err = model.UpdateOne(context.Background(), filters, newValues)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	timelineModel := timelinemodel.New(ffh.db)
	timelineEntry := timelinemodel.NewTimelineEntry(userID, timelinemodel.RevisionApproved)
	err = timelineModel.UpdateOne(context.Background(), featureFlagID, timelineEntry)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, featureFlagRecord)
}

func (ffh *FeatureFlagHandler) RollbackFeatureFlagVersion(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		ffh.logger.Debug("Server error",
			zap.Error(errors.New(apierrors.ForbiddenError)),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	featureFlagID, err := primitive.ObjectIDFromHex(c.Param("featureFlagID"))
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := featureflagmodel.New(ffh.db)
	featureFlagRecord, err := model.FindByID(context.Background(), featureFlagID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	var newRevisionID primitive.ObjectID
	for index, revision := range featureFlagRecord.Revisions {
		if revision.Status == featureflagmodel.Live {
			featureFlagRecord.Revisions[index].Status = featureflagmodel.Draft
			newRevisionID = revision.LastRevisionID
			featureFlagRecord.Revisions[index].LastRevisionID = primitive.NilObjectID
		}
	}
	for index, revision := range featureFlagRecord.Revisions {
		if revision.ID == newRevisionID && revision.Status == featureflagmodel.Archived {
			featureFlagRecord.Revisions[index].Status = featureflagmodel.Live
		}
	}
	featureFlagRecord.Version--

	filters := bson.M{"$and": []bson.M{
		{"_id": featureFlagID},
		{"organization_id": organizationID},
	}}
	newValues := bson.D{
		{
			Key: "$set", Value: bson.D{
				{Key: "version", Value: featureFlagRecord.Version},
				{Key: "revisions", Value: featureFlagRecord.Revisions},
			},
		},
	}
	err = model.UpdateOne(context.Background(), filters, newValues)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	timelineModel := timelinemodel.New(ffh.db)
	timelineEntry := timelinemodel.NewTimelineEntry(userID, timelinemodel.FeatureFlagRollback)
	err = timelineModel.UpdateOne(context.Background(), featureFlagID, timelineEntry)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, featureFlagRecord)
}

func (ffh *FeatureFlagHandler) DeleteFeatureFlag(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		ffh.logger.Debug("Server error",
			zap.Error(errors.New(apierrors.ForbiddenError)),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	featureFlagID, err := primitive.ObjectIDFromHex(c.Param("featureFlagID"))
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := featureflagmodel.New(ffh.db)

	err = model.UpdateOne(
		context.Background(),
		bson.M{"$and": []bson.M{
			{"_id": featureFlagID},
			{"organization_id": organizationID},
		}},
		bson.D{
			{Key: "$set", Value: bson.D{
				{
					Key:   "deleted_at",
					Value: primitive.NewDateTimeFromTime(time.Now().UTC()),
				},
			}},
		},
	)

	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	timelineModel := timelinemodel.New(ffh.db)
	timelineEntry := timelinemodel.NewTimelineEntry(userID, timelinemodel.FeatureFlagDeleted)
	err = timelineModel.UpdateOne(context.Background(), featureFlagID, timelineEntry)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	ffh.logger.Info("Soft deleted feature flag",
		zap.String("_id", featureFlagID.Hex()))
	return c.NoContent(http.StatusNoContent)
}

func (ffh *FeatureFlagHandler) ToggleFeatureFlag(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		ffh.logger.Debug("Server error",
			zap.Error(errors.New(apierrors.ForbiddenError)),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	featureFlagID, err := primitive.ObjectIDFromHex(c.Param("featureFlagID"))
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := featureflagmodel.New(ffh.db)
	featureFlagRecord, err := model.FindByID(context.Background(), featureFlagID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	environmentName := c.QueryParams().Get("env")
	for index, environment := range featureFlagRecord.Environments {
		if environment.Name == environmentName {
			featureFlagRecord.Environments[index].IsEnabled = !(featureFlagRecord.Environments[index].IsEnabled)
		}
	}

	filters := bson.M{"$and": []bson.M{
		{"_id": featureFlagID},
		{"organization_id": organizationID},
	}}
	newValues := bson.D{
		{
			Key: "$set", Value: bson.D{
				{Key: "environments", Value: featureFlagRecord.Environments},
			},
		},
	}
	err = model.UpdateOne(context.Background(), filters, newValues)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	timelineModel := timelinemodel.New(ffh.db)
	timelineEntry := timelinemodel.NewTimelineEntry(userID, fmt.Sprintf(timelinemodel.FeatureFlagToggle, environmentName))
	err = timelineModel.UpdateOne(context.Background(), featureFlagID, timelineEntry)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, featureFlagRecord)
}

func (ffh *FeatureFlagHandler) PatchFeatureFlagTags(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	organizationModel := organizationmodel.New(ffh.db)
	organization, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organization, organizationmodel.Collaborator)
	if !permission {
		ffh.logger.Debug("Client error",
			zap.Error(errors.New(apierrors.ForbiddenError)),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	featureFlagID, err := primitive.ObjectIDFromHex(c.Param("featureFlagID"))
	if err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	request := new(PatchFeatureFlagTagsRequest)
	if err := c.Bind(request); err != nil {
		ffh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	model := featureflagmodel.New(ffh.db)

	err = model.UpdateOne(
		context.Background(),
		bson.M{"$and": []bson.M{
			{"_id": featureFlagID},
			{"organization_id": organizationID},
		}},
		bson.D{{Key: "$addToSet",
			Value: bson.M{"tags": bson.M{"$each": request.Tags}},
		}},
	)
	if err != nil {
		ffh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	ffh.logger.Info("Feature flag updated",
		zap.String("_id", featureFlagID.Hex()))
	return c.NoContent(http.StatusNoContent)
}
