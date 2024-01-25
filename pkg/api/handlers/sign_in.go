package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/config"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
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
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	validate := validator.New()

	if err := validate.Struct(request); err != nil {
		sh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := usermodel.New(sh.db)

	ur, err := model.FindByEmail(context.Background(), request.Email)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			sh.logger.Debug("Client error",
				zap.Error(errors.New(apierrors.NotFoundError)),
			)
			return apierrors.CustomError(c,
				http.StatusNotFound,
				apierrors.NotFoundError,
			)
		}

		sh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(ur.Password), []byte(request.Password)); err != nil {
		sh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	token, err := apiutils.CreateJWT(ur.ID, config.JWTExpireTime)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	cookie := new(http.Cookie)
	cookie.Name = "Authorization"
	cookie.Value = "Bearer " + token
	cookie.Expires = time.Now().Add(config.JWTExpireTime * time.Millisecond)
	cookie.HttpOnly = true
	c.SetCookie(cookie)

	sh.logger.Debug("User logged in",
		zap.String("_id", ur.ID.Hex()),
	)
	return c.JSON(http.StatusOK, common.AuthResponse{
		ID:        ur.ID,
		Email:     ur.Email,
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
	})
}

func NewSignInHandler(db *mongo.Database, logger *zap.Logger) *SignInHandler {
	return &SignInHandler{
		db:     db,
		logger: logger,
	}
}
