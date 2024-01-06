package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type SignInHandler struct {
	db *mongo.Database
}

type SignInRequest struct {
	Email    string `json:"email" bson:"email" validate:"email"`
	Password string `json:"password" validate:"required"`
}

func (sh *SignInHandler) PostSignIn(c echo.Context) error {
	request := new(SignInRequest)

	if err := c.Bind(request); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	model := models.NewUserModel(sh.db)

	ur, err := model.FindByEmail(context.Background(), request.Email)

	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		if errors.Is(err, mongo.ErrNoDocuments) {
			return apierrors.CustomError(c,
				http.StatusNotFound,
				apierrors.NotFoundError,
			)
		}

		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(ur.Password), []byte(request.Password)); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusUnauthorized,
			apierrors.UnauthorizedError,
		)
	}

	token, err := apiutils.CreateJWT(ur.ID, config.JWTExpireTime)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusOK, common.AuthResponse{
		ID:        ur.ID,
		Email:     ur.Email,
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
		Token:     token,
	})
}

func NewSignInHandler(db *mongo.Database) *SignInHandler {
	return &SignInHandler{
		db: db,
	}
}
