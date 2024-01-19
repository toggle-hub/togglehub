package handlers

import (
	"context"
	"errors"
	"net/http"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
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

func NewOrganizationHandler(db *mongo.Database, logger *zap.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		db:     db,
		logger: logger,
	}
}
