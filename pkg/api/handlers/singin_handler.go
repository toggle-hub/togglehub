package handlers

import (
	"context"
	"errors"
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
	Email    string `json:"email" bson:"email"`
	Password string `json:"password"`
}

func (sh *SignInHandler) PostSignIn(c echo.Context) error {
	req := new(SignInRequest)

	if err := c.Bind(req); err != nil {
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	model := models.NewUserModel(sh.db.Collection(models.UserCollectionName))

	ur, err := model.FindByEmail(context.Background(), req.Email)

	if err != nil {
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

	if err := bcrypt.CompareHashAndPassword([]byte(ur.Password), []byte(req.Password)); err != nil {
		return apierrors.CustomError(c, http.StatusUnauthorized, apierrors.UnauthorizedError)
	}

	token, err := apiutils.CreateJWT(ur.ID, config.JWTExpireTime)
	if err != nil {
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
