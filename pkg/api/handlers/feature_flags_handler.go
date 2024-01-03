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

	req := new(models.FeatureFlagRequest)
	if err := c.Bind(req); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := models.NewFeatureFlagModel(ffh.db)
	ffr := models.NewFeatureFlagRecord(req, orgID, userID)
	newRecID, err := model.InsertOne(context.Background(), ffr)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	ffr.ID = newRecID

	return c.JSON(http.StatusCreated, ffr)
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

	permisson, err := userHasPermission(userID, orgID, models.Collaborator, ffh.db)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	if !permisson {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
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
	model.UpdateOne(
		context.Background(),
		ffID,
		bson.M{
			"revisions": rev,
		},
	)

	return c.JSON(http.StatusOK, rev)
}

func userHasPermission(userID, orgID primitive.ObjectID, permission models.PermissionLevelEnum, db *mongo.Database) (bool, error) {
	om := models.NewOrganizationModel(db)
	or, err := om.FindByID(context.Background(), orgID)
	if err != nil {
		return false, err
	}
	for _, m := range or.Members {
		if m.User.ID == userID {
			switch permission {
			case models.Admin:
				return m.PermissionLevel == permission, nil
			case models.Collaborator:
				return m.PermissionLevel == permission || m.PermissionLevel == models.Admin, nil
			case models.ReadOnly:
				return m.PermissionLevel == permission || m.PermissionLevel == models.Collaborator || m.PermissionLevel == models.Admin, nil
			}
		}
	}

	return false, nil

}
