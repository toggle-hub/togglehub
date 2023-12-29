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
)

type SignUpHandler struct {
	db *mongo.Database
}

func NewSignUpHandler(db *mongo.Database) *SignUpHandler {
	return &SignUpHandler{
		db: db,
	}
}

type SignUpRequest struct {
	Email     string `json:"email" bson:"email"`
	Password  string `json:"password" bson:"password"`
	FirstName string `json:"first_name" bson:"first_name"`
	LastName  string `json:"last_name" bson:"last_name"`
}

func (sh *SignUpHandler) PostUser(c echo.Context) error {
	req := new(SignUpRequest)
	if err := c.Bind(req); err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
		)
	}

	model := models.NewUserModel(sh.db)
	_, err := model.FindByEmail(context.Background(), req.Email)
	if err == nil {
		log.Println(apiutils.HandlerErrorLogMessage(errors.New(apierrors.EmailConflictError), c))
		return apierrors.CustomError(c,
			http.StatusConflict,
			apierrors.EmailConflictError,
		)
	}

	ur, err := models.NewUserRecord(req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	objectID, err := model.InsertOne(context.Background(), ur)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	token, err := apiutils.CreateJWT(objectID, config.JWTExpireTime)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	return c.JSON(http.StatusCreated, common.AuthResponse{
		ID:        objectID,
		Email:     ur.Email,
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
		Token:     token,
	})
}
