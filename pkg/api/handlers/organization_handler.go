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
	"go.mongodb.org/mongo-driver/mongo"
)

type OrganizationHandler struct {
	db *mongo.Database
}

type OrganizationPostRequest struct {
	Name string `json:"name" validate:"required"`
}

func (oh *OrganizationHandler) PostOrganization(c echo.Context) error {
	req := new(OrganizationPostRequest)
	if err := c.Bind(req); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
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

		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	userModel := models.NewUserModel(oh.db)
	user, err := userModel.FindByID(context.Background(), userID)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	user.Password = ""

	organization := models.NewOrganizationRecord(req.Name, user)

	model := models.NewOrganizationModel(oh.db)

	_, err = model.InsertOne(context.Background(), organization)

	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusCreated, organization)
}

func NewOrganizationHandler(db *mongo.Database) *OrganizationHandler {
	return &OrganizationHandler{
		db: db,
	}
}
