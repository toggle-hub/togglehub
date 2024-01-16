package userhandler

import (
	"context"
	"errors"
	"log"
	"net/http"

	api_errors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/models"
	api_utils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type UserHandler struct {
	db     *mongo.Database
	logger *zap.Logger
}

func New(db *mongo.Database, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		db:     db,
		logger: logger,
	}
}

type UserPatchRequest struct {
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
}

type UserPatchResponse struct {
	ID        primitive.ObjectID `json:"_id,omitempty"`
	Email     string             `json:"email" `
	FirstName string             `json:"first_name,omitempty" `
	LastName  string             `json:"last_name,omitempty" `
}

type UserOrganization struct {
	ID   primitive.ObjectID `json:"_id"`
	Name string             `json:"name"`
}

type UserGetResponse struct {
	ID            primitive.ObjectID `json:"_id"`
	Email         string             `json:"email"`
	FirstName     string             `json:"first_name,omitempty"`
	LastName      string             `json:"last_name,omitempty"`
	Organizations []UserOrganization `json:"organizations"`
}

func NewUserGetResponse(user *models.UserRecord, organizations []models.OrganizationRecord) *UserGetResponse {
	userOrganizations := make([]UserOrganization, len(organizations))

	for index, organization := range organizations {
		userOrganizations[index] = UserOrganization{
			ID:   organization.ID,
			Name: organization.Name,
		}
	}

	return &UserGetResponse{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Organizations: userOrganizations,
	}
}

func (uh *UserHandler) GetUser(c echo.Context) error {
	userID, err := api_utils.GetUserFromContext(c)
	if err != nil {
		log.Println(api_utils.HandlerErrorLogMessage(err, c))
		// Should never happen but better safe than sorry
		if errors.Is(err, api_utils.ErrNotAuthenticated) {
			uh.logger.Debug("Client error",
				zap.String("cause", err.Error()),
			)
			return api_errors.CustomError(
				c,
				http.StatusUnauthorized,
				api_errors.UnauthorizedError,
			)
		}

		uh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	model := models.NewUserModel(uh.db)
	user, err := model.FindByID(context.Background(), userID)
	if err != nil {
		uh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusNotFound,
			api_errors.NotFoundError,
		)
	}

	organizationModel := models.NewOrganizationModel(uh.db)
	organizations, err := organizationModel.FindByMember(context.Background(), user.ID)
	if err != nil {
		uh.logger.Debug("Server error",
			zap.String("cause", err.Error()))
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	return c.JSON(
		http.StatusOK,
		NewUserGetResponse(user, organizations),
	)
}

func (uh *UserHandler) PatchUser(c echo.Context) error {
	request := new(UserPatchRequest)
	if err := c.Bind(request); err != nil {
		uh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(request); err != nil {
		uh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	userID, err := api_utils.GetUserFromContext(c)
	if err != nil {
		log.Println(api_utils.HandlerErrorLogMessage(err, c))
		// Should never happen but better safe than sorry
		if errors.Is(err, api_utils.ErrNotAuthenticated) {
			uh.logger.Debug("Client error",
				zap.String("cause", err.Error()),
			)
			return api_errors.CustomError(
				c,
				http.StatusUnauthorized,
				api_errors.UnauthorizedError,
			)
		}

		uh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	model := models.NewUserModel(uh.db)
	ur, err := model.FindByID(context.Background(), userID)
	if err != nil {
		uh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusNotFound,
			api_errors.NotFoundError,
		)
	}

	objectID, err := model.UpdateOne(
		context.Background(),
		userID,
		bson.D{
			{Key: "first_name", Value: request.FirstName},
			{Key: "last_name", Value: request.LastName},
		},
	)

	if err != nil {
		uh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, UserPatchResponse{
		ID:        objectID,
		Email:     ur.Email,
		FirstName: request.FirstName,
		LastName:  request.LastName,
	})
}
