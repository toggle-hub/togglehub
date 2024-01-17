package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"

	api_errors "github.com/Roll-Play/togglelabs/pkg/api/error"
	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	api_utils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
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
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(request); err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	userID, err := api_utils.GetUserFromContext(c)
	if err != nil {
		// Should never happen but better safe than sorry
		if errors.Is(err, api_utils.ErrNotAuthenticated) {
			oh.logger.Debug("Client error",
				zap.String("cause", err.Error()),
			)
			return api_errors.CustomError(
				c,
				http.StatusUnauthorized,
				api_errors.UnauthorizedError,
			)
		}

		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	userModel := usermodel.New(oh.db)
	user, err := userModel.FindByID(context.Background(), userID)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
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
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	return c.JSON(http.StatusCreated, organization)
}

func (oh *OrganizationHandler) PostProject(c echo.Context) error {
	userID, err := api_utils.GetUserFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	organizationID, err := api_utils.GetOrganizationFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}
	organizationModel := organizationmodel.New(oh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	permission := api_utils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		oh.logger.Debug("Client error",
			zap.String("cause", api_errors.ForbiddenError),
		)
		return api_errors.CustomError(
			c,
			http.StatusForbidden,
			api_errors.ForbiddenError,
		)
	}

	request := new(ProjectPostRequest)
	log.Print("A")
	if err := c.Bind(request); err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	project := organizationmodel.Project{
		Name:        request.Name,
		Description: request.Description,
	}

	err = organizationModel.UpdateOne(
		context.Background(),
		bson.D{{Key: "_id", Value: organizationID}},
		bson.D{{Key: "$push", Value: bson.M{"projects": project}}},
	)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, project)
}

func (oh *OrganizationHandler) GetOrganization(c echo.Context) error {
	userID, err := api_utils.GetUserFromContext(c)
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	organizationID, err := primitive.ObjectIDFromHex(c.Param("organizationID"))
	if err != nil {
		oh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	organizationModel := organizationmodel.New(oh.db)
	organizationRecord, err := organizationModel.FindByID(context.Background(), organizationID)
	if err != nil {
		oh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	permission := api_utils.UserHasPermission(userID, organizationRecord, organizationmodel.Collaborator)
	if !permission {
		oh.logger.Debug("Client error",
			zap.String("cause", api_errors.ForbiddenError),
		)
		return api_errors.CustomError(
			c,
			http.StatusForbidden,
			api_errors.ForbiddenError,
		)
	}

	return c.JSON(http.StatusOK, organizationRecord)
}

func NewOrganizationHandler(db *mongo.Database, logger *zap.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		db:     db,
		logger: logger,
	}
}
