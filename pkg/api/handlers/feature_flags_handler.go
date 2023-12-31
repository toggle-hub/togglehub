package handlers

import (
	"context"
	"net/http"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/models"
	"github.com/labstack/echo/v4"
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
	userID := c.Get("user").(primitive.ObjectID)
	orgID, err := primitive.ObjectIDFromHex(c.Param("orgId"))
	if err != nil {
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	req := new(models.FeatureFlagRequest)
	if err := c.Bind(req); err != nil {
		return apierrors.CustomError(
			c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}
	model := models.NewFeatureFlagModel(ffh.db)

	ffr, err := model.NewFeatureFlagRecord(req, orgID, userID)

	newRecID, err := model.InsertOne(context.Background(), ffr)
	if err != nil {
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	return c.JSON(http.StatusCreated, newRecID)
}
