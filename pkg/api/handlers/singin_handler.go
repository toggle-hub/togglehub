package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	"github.com/Roll-Play/togglelabs/pkg/config"
	apierror "github.com/Roll-Play/togglelabs/pkg/error"
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
		return apierror.CustomError(c,
			http.StatusInternalServerError,
			apierror.InternalServerError,
		)
	}

	collection := sh.db.Collection(models.UserCollectionName)
	model := models.NewUserModel(collection)

	ur, err := model.FindByEmail(context.Background(), req.Email)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return apierror.CustomError(c,
				http.StatusNotFound,
				apierror.NotFoundError,
			)
		}

		return apierror.CustomError(
			c,
			http.StatusInternalServerError,
			apierror.InternalServerError,
		)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(ur.Password), []byte(req.Password)); err != nil {
		return apierror.CustomError(c, http.StatusUnauthorized, apierror.UnauthorizedError)
	}

	token, err := apiutils.CreateJWT(ur.ID, config.JWTExpireTime)
	if err != nil {
		return apierror.CustomError(c,
			http.StatusInternalServerError,
			apierror.InternalServerError,
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
