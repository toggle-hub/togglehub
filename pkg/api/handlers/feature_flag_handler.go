package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FeatureFlagHandler struct {
	db *mongo.Database
}

func NewFeatureFlagHandler(db *mongo.Database) *FeatureFlagHandler {
	return &FeatureFlagHandler{
		db: db,
	}
}

type PostFeatureFlagRequest struct {
	Name         string          `json:"name" validate:"required"`
	Type         models.FlagType `json:"type" validate:"required,oneof=boolean json string number"`
	DefaultValue string          `json:"default_value" validate:"required"`
	Rules        []models.Rule   `json:"rules" validate:"dive,required"`
}

type PatchFeatureFlagRequest struct {
	DefaultValue string        `json:"default_value"`
	Rules        []models.Rule `json:"rules" validate:"dive,required"`
}

func (ffh *FeatureFlagHandler) ListFeatureFlags(c echo.Context) error {
	userID, err := apiutils.GetObjectIDFromContext(c)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}
	organizationModel := models.NewOrganizationModel(ffh.db)
	if err := organizationModel.UserHasReadPermission(context.Background(), userID, organizationID); err != nil {
		if errors.Is(err, apiutils.ErrReadPermissionDenied) {
			return apierrors.CustomError(
				c,
				http.StatusForbidden,
				apierrors.ForbiddenError,
			)
		}

		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := models.NewFeatureFlagModel(ffh.db)

	featureFlags, err := model.FindMany(context.Background(), organizationID)
	if err != nil {
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	bytes, _ := json.Marshal(featureFlags)

	log.Println(apiutils.HandlerLogMessage(string(bytes), primitive.NewObjectID(), c))
	return c.JSON(http.StatusOK, featureFlags)
}

func (ffh *FeatureFlagHandler) PostFeatureFlag(c echo.Context) error {
	userID, err := apiutils.GetObjectIDFromContext(c)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := models.NewOrganizationModel(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := userHasPermission(userID, organizationRecord, models.Collaborator)
	if !permission {
		log.Println(apiutils.HandlerLogMessage("feature-flag", userID, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	req := new(PostFeatureFlagRequest)
	if err := c.Bind(req); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(req); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	featureFlagModel := models.NewFeatureFlagModel(ffh.db)
	featureFlagRecord := models.NewFeatureFlagRecord(
		req.Name,
		req.DefaultValue,
		req.Type,
		req.Rules,
		organizationID,
		userID,
	)

	insertedID, err := featureFlagModel.InsertOne(context.Background(), featureFlagRecord)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	featureFlagRecord.ID = insertedID

	return c.JSON(http.StatusCreated, featureFlagRecord)
}

func (ffh *FeatureFlagHandler) PatchFeatureFlag(c echo.Context) error {
	userID, err := apiutils.GetObjectIDFromContext(c)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := models.NewOrganizationModel(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := userHasPermission(userID, organizationRecord, models.Collaborator)
	if !permission {
		log.Println(apiutils.HandlerLogMessage("feature-flag", userID, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	featureFlagID, err := primitive.ObjectIDFromHex(c.Param("featureFlagID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	req := new(PatchFeatureFlagRequest)
	if err := c.Bind(req); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := models.NewFeatureFlagModel(ffh.db)

	rev := models.NewRevisionRecord(
		req.DefaultValue,
		req.Rules,
		userID,
	)
	_, err = model.UpdateOne(
		context.Background(),
		bson.D{{Key: "_id", Value: featureFlagID}},
		bson.D{{Key: "$push", Value: bson.M{"revisions": rev}}},
	)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, rev)
}

func (ffh *FeatureFlagHandler) ApproveRevision(c echo.Context) error {
	userID, err := apiutils.GetObjectIDFromContext(c)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := models.NewOrganizationModel(ffh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := userHasPermission(userID, organizationRecord, models.Collaborator)
	if !permission {
		log.Println(apiutils.HandlerLogMessage("feature-flag", userID, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	featureFlagID, err := primitive.ObjectIDFromHex(c.Param("featureFlagID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	revisionID, err := primitive.ObjectIDFromHex(c.Param("revisionID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}
	log.Print(revisionID)

	model := models.NewFeatureFlagModel(ffh.db)

	featureFlagRecord, err := model.FindByID(context.Background(), featureFlagID)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	filters := bson.D{{Key: "_id", Value: featureFlagID}, {Key: "revisions.status", Value: "live"}}
	newValues := bson.D{{Key: "$set", Value: bson.D{{Key: "revisions.$.status", Value: "draft"}}}}
	_, err = model.UpdateOne(context.Background(), filters, newValues)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	filters = bson.D{{Key: "_id", Value: featureFlagID}, {Key: "revisions._id", Value: revisionID}}
	newValues = bson.D{
		{
			Key: "$set", Value: bson.D{
				{Key: "version", Value: featureFlagRecord.Version + 1},
				{Key: "revisions.$.status", Value: "live"},
			},
		},
	}
	_, err = model.UpdateOne(context.Background(), filters, newValues)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, featureFlagID)
}

func userHasPermission(
	userID primitive.ObjectID,
	organization *models.OrganizationRecord,
	permission models.PermissionLevelEnum,
) bool {
	for _, member := range organization.Members {
		if member.User.ID == userID {
			switch permission {
			case models.Admin:
				return member.PermissionLevel == permission
			case models.Collaborator:
				return member.PermissionLevel == permission || member.PermissionLevel == models.Admin
			case models.ReadOnly:
				return member.PermissionLevel == permission ||
					member.PermissionLevel == models.Collaborator ||
					member.PermissionLevel == models.Admin
			}
		}
	}

	return false
}