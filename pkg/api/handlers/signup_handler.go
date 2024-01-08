package handlers

import (
	"context"
	"net/http"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type SignUpHandler struct {
	db     *mongo.Database
	logger *zap.Logger
}

func NewSignUpHandler(db *mongo.Database, logger *zap.Logger) *SignUpHandler {
	return &SignUpHandler{
		db:     db,
		logger: logger,
	}
}

type SignUpRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"gte=8"`
}

func (sh *SignUpHandler) PostUser(c echo.Context) error {
	request := new(SignUpRequest)
	if err := c.Bind(request); err != nil {
		sh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(request); err != nil {
		sh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := models.NewUserModel(sh.db)
	_, err := model.FindByEmail(context.Background(), request.Email)
	if err == nil {
		sh.logger.Debug("Client error",
			zap.String("cause", apierrors.EmailConflictError),
		)
		return apierrors.CustomError(c,
			http.StatusConflict,
			apierrors.EmailConflictError,
		)
	}

	ur, err := models.NewUserRecord(request.Email, request.Password, "", "")
	if err != nil {
		sh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	objectID, err := model.InsertOne(context.Background(), ur)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	token, err := apiutils.CreateJWT(objectID, config.JWTExpireTime)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	sh.logger.Debug("Created user",
		zap.String("_id", objectID.Hex()),
	)
	return c.JSON(http.StatusCreated, common.AuthResponse{
		ID:        objectID,
		Email:     ur.Email,
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
		Token:     token,
	})
}
