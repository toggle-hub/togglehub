package handlers

import (
	"context"
	"log"
	"net/http"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
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

func (sh *UserHandler) PatchUser(c echo.Context) error {
	userID := c.Get("user").(primitive.ObjectID)
	model := models.NewUserModel(sh.db.Collection(models.UserCollectionName))
	ur, err := model.FindByID(context.Background(), userID)
	if err != nil {
		return apierrors.CustomError(c,
			http.StatusNotFound,
			apierrors.NotFoundError,
		)
	}

	patchReq := new(UserPatchRequest)
	if err := c.Bind(patchReq); err != nil {
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	log.Print("request ", patchReq)

	objectID, err := model.UpdateOne(
		context.Background(),
		userID,
		bson.D{
			{Key: "first_name", Value: patchReq.FirstName},
			{Key: "last_name", Value: patchReq.LastName},
		},
	)

	if err != nil {
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, common.PatchResponse{
		ID:        objectID,
		Email:     ur.Email,
		FirstName: patchReq.FirstName,
		LastName:  patchReq.LastName,
	})
}
