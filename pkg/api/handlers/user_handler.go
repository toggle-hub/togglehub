package handlers

import (
	"context"
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
	"go.uber.org/zap"
)

type UserHandler struct {
	db     *mongo.Database
	logger *zap.Logger
}

func NewUserHandler(db *mongo.Database, logger *zap.Logger) *UserHandler {
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

func (uh *UserHandler) PatchUser(c echo.Context) error {
	request := new(UserPatchRequest)
	if err := c.Bind(request); err != nil {
		uh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(request); err != nil {
		uh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	userID, err := apiutils.GetObjectIDFromContext(c)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		// Should never happen but better safe than sorry
		if errors.Is(err, apiutils.ErrNotAuthenticated) {
			uh.logger.Debug("Client error",
				zap.String("cause", err.Error()),
			)
			return apierrors.CustomError(
				c,
				http.StatusUnauthorized,
				apierrors.UnauthorizedError,
			)
		}

		uh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	model := models.NewUserModel(uh.db)
	ur, err := model.FindByID(context.Background(), userID)
	if err != nil {
		uh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusNotFound,
			apierrors.NotFoundError,
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
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, UserPatchResponse{
		ID:        objectID,
		Email:     ur.Email,
		FirstName: request.FirstName,
		LastName:  request.LastName,
	})
}
