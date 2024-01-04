package handlers

import (
	"context"
	"errors"
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

type UserHandler struct {
	db *mongo.Database
}

func NewUserHandler(db *mongo.Database) *UserHandler {
	return &UserHandler{
		db: db,
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

func (sh *UserHandler) PatchUser(c echo.Context) error {
	req := new(UserPatchRequest)
	if err := c.Bind(req); err != nil {
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
			return apierrors.CustomError(
				c,
				http.StatusUnauthorized,
				apierrors.UnauthorizedError,
			)
		}
	}

	model := models.NewUserModel(sh.db)
	ur, err := model.FindByID(context.Background(), userID)
	if err != nil {
		return apierrors.CustomError(c,
			http.StatusNotFound,
			apierrors.NotFoundError,
		)
	}

	objectID, err := model.UpdateOne(
		context.Background(),
		userID,
		bson.D{
			{Key: "first_name", Value: req.FirstName},
			{Key: "last_name", Value: req.LastName},
		},
	)

	if err != nil {
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, UserPatchResponse{
		ID:        objectID,
		Email:     ur.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
}
