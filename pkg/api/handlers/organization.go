package handlers

import (
	"context"
	"errors"
	"net/http"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	featureflagmodel "github.com/Roll-Play/togglelabs/pkg/models/feature_flag"
	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type OrganizationHandler struct {
	db     *mongo.Database
	logger *zap.Logger
}

type OrganizationPostRequest struct {
	Name string `json:"name" validate:"required"`
}

type ProjectPostRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description" validate:"required"`
}

func (oh *OrganizationHandler) PostOrganization(c echo.Context) error {
	request := new(OrganizationPostRequest)
	if err := c.Bind(request); err != nil {
		oh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(request); err != nil {
		oh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		// Should never happen but better safe than sorry
		if errors.Is(err, apiutils.ErrNotAuthenticated) {
			oh.logger.Debug("Client error",
				zap.Error(err),
			)
			return apierrors.CustomError(
				c,
				http.StatusUnauthorized,
				apierrors.UnauthorizedError,
			)
		}

		oh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	userModel := usermodel.New(oh.db)
	user, err := userModel.FindByID(context.Background(), userID)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	user.Password = ""

	organization := organizationmodel.NewOrganizationRecord(request.Name, []organizationmodel.OrganizationMember{{
		User:            *user,
		PermissionLevel: organizationmodel.Admin,
	}})

	model := organizationmodel.New(oh.db)

	_, err = model.InsertOne(context.Background(), organization)

	if err != nil {
		oh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusCreated, organization)
}

func (oh *OrganizationHandler) PostProject(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}
	organizationModel := organizationmodel.New(oh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		oh.logger.Debug("Client error",
			zap.String("cause", apierrors.ForbiddenError),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	request := new(ProjectPostRequest)
	if err := c.Bind(request); err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	project := organizationmodel.NewProjectRecord(request.Name, request.Description)

	err = organizationModel.UpdateOne(
		context.Background(),
		bson.D{{Key: "_id", Value: organizationID}},
		bson.D{{Key: "$push", Value: bson.M{"projects": project}}},
	)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, project)
}

func (oh *OrganizationHandler) GetOrganization(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(oh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.ReadOnly)
	if !permission {
		oh.logger.Debug("Client error",
			zap.String("cause", apierrors.ForbiddenError),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	return c.JSON(http.StatusOK, organizationRecord)
}

func (oh *OrganizationHandler) DeleteProject(c echo.Context) error {
	userID, err := apiutils.GetUserFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationID, err := apiutils.GetOrganizationFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(oh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	permission := apiutils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		oh.logger.Debug("Server error",
			zap.String("cause", apierrors.ForbiddenError),
		)
		return apierrors.CustomError(
			c,
			http.StatusForbidden,
			apierrors.ForbiddenError,
		)
	}

	projectID, err := primitive.ObjectIDFromHex(c.Param("projectID"))
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	featureFlagModel := featureflagmodel.New(oh.db)
	err = featureFlagModel.UpdateMany(context.Background(),
		bson.D{{Key: "project._id", Value: projectID}},
		bson.D{{
			Key: "$unset",
			Value: bson.M{
				"project": 1,
			},
		}},
	)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	err = organizationModel.UpdateOne(context.Background(),
		bson.D{{Key: "_id", Value: organizationID}},
		bson.D{
			{Key: "$pull", Value: bson.D{
				{Key: "projects", Value: bson.M{"_id": projectID}},
			}},
		},
	)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	oh.logger.Info("Project deleted",
		zap.String("_id", projectID.Hex()))
	return c.NoContent(http.StatusNoContent)
}

func NewOrganizationHandler(db *mongo.Database, logger *zap.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		db:     db,
		logger: logger,
	}
}
