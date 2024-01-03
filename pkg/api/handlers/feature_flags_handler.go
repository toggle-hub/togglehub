package handlers

import (
	"context"
	"log"
	"net/http"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
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

	orgID, err := primitive.ObjectIDFromHex(c.Param("orgID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	orgModel := models.NewOrganizationModel(ffh.db)
	orgRecord, err := orgModel.FindByID(context.Background(), orgID)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := userHasPermission(userID, orgRecord, models.Collaborator)
	if !permission {
		log.Println(apiutils.HandlerLogMessage("feature-flag", userID, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	req := new(models.FeatureFlagRequest)
	if err := c.Bind(req); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	featureFlagModel := models.NewFeatureFlagModel(ffh.db)
	featureFlagRecord := models.NewFeatureFlagRecord(req, orgID, userID)
	newRecID, err := featureFlagModel.InsertOne(context.Background(), featureFlagRecord)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	featureFlagRecord.ID = newRecID

	return c.JSON(http.StatusCreated, featureFlagRecord)
}

func (ffh *FeatureFlagHandler) PostRevision(c echo.Context) error {
	userID, err := apiutils.GetObjectIDFromContext(c)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	orgID, err := primitive.ObjectIDFromHex(c.Param("orgID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	orgModel := models.NewOrganizationModel(ffh.db)
	orgRecord, err := orgModel.FindByID(context.Background(), orgID)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := userHasPermission(userID, orgRecord, models.Collaborator)
	if !permission {
		log.Println(apiutils.HandlerLogMessage("feature-flag", userID, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	ffID, err := primitive.ObjectIDFromHex(c.Param("ffID"))
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	req := new(models.RevisionRequest)
	if err := c.Bind(req); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := models.NewFeatureFlagModel(ffh.db)

	rev := model.NewRevisionRecord(req, userID)
	_, err = model.UpdateOne(
		context.Background(),
		ffID,
		bson.M{
			"revisions": rev,
		},
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

func userHasPermission(
	userID primitive.ObjectID,
	orgRecord *models.OrganizationRecord,
	permission models.PermissionLevelEnum,
) bool {
	for _, member := range orgRecord.Members {
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
