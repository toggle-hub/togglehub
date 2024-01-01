package handlers

import (
	"context"
	"net/http"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/models"
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
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type UserPatchResponse struct {
	ID        primitive.ObjectID `json:"_id,omitempty"`
	Email     string             `json:"email" `
	FirstName string             `json:"first_name,omitempty" `
	LastName  string             `json:"last_name,omitempty" `
}

func (sh *UserHandler) PatchUser(c echo.Context) error {
	user := c.Get("user").(middlewares.ContextUser)
	model := models.NewUserModel(sh.db)
	ur, err := model.FindByID(context.Background(), user.ID)
	if err != nil {
		return apierrors.CustomError(c,
			http.StatusNotFound,
			apierrors.NotFoundError,
		)
	}

	req := new(UserPatchRequest)
	if err := c.Bind(req); err != nil {
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	objectID, err := model.UpdateOne(
		context.Background(),
		user.ID,
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
