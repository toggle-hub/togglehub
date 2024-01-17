package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	api_errors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/config"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	api_utils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type SignInHandler struct {
	db     *mongo.Database
	logger *zap.Logger
}

type SignInRequest struct {
	Email    string `json:"email" bson:"email" validate:"email"`
	Password string `json:"password" validate:"required"`
}

func (sh *SignInHandler) PostSignIn(c echo.Context) error {
	request := new(SignInRequest)

	if err := c.Bind(request); err != nil {
		sh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(request); err != nil {
		sh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusBadRequest,
			api_errors.BadRequestError,
		)
	}

	model := usermodel.New(sh.db)

	ur, err := model.FindByEmail(context.Background(), request.Email)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			sh.logger.Debug("Client error",
				zap.String("cause", api_errors.NotFoundError),
			)
			return api_errors.CustomError(c,
				http.StatusNotFound,
				api_errors.NotFoundError,
			)
		}

		sh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(ur.Password), []byte(request.Password)); err != nil {
		sh.logger.Debug("Client error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusUnauthorized,
			api_errors.UnauthorizedError,
		)
	}

	token, err := api_utils.CreateJWT(ur.ID, config.JWTExpireTime)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	sh.logger.Debug("User logged in",
		zap.String("_id", ur.ID.Hex()),
	)
	return c.JSON(http.StatusOK, common.AuthResponse{
		ID:        ur.ID,
		Email:     ur.Email,
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
		Token:     token,
	})
}

func NewSignInHandler(db *mongo.Database, logger *zap.Logger) *SignInHandler {
	return &SignInHandler{
		db:     db,
		logger: logger,
	}
}
