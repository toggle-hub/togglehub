package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/api/sqs_helper"
	"github.com/Roll-Play/togglelabs/pkg/config"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
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
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusBadRequest,
			apierrors.BadRequestError,
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
	_, err := model.FindByEmail(context.Background(), request.Email)
	if err == nil {
		sh.logger.Debug("Client error",
			zap.Error(errors.New(apierrors.EmailConflictError)),
		)
		return apierrors.CustomError(c,
			http.StatusConflict,
			apierrors.EmailConflictError,
		)
	}

	ur, err := usermodel.NewUserRecord(request.Email, request.Password, "", "")
	if err != nil {
		sh.logger.Debug("Client error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	objectID, err := model.InsertOne(context.Background(), ur)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	token, err := apiutils.CreateJWT(objectID, config.JWTExpireTime)
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

	sqsHelper, err := sqs_helper.NewSqsHelper()
	messageAttributes := map[string]*sqs.MessageAttributeValue{
		"Title": &sqs.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String("Email validation"),
		},
		"UserId": &sqs.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(ur.ID.String()),
		},
	}
	sqsHelper.SendMessage(10, messageAttributes, "Email validation information that I'm not really sure how to generate")
	if err != nil {
		return err
	}

	sh.logger.Debug("Created user",
		zap.String("_id", objectID.Hex()),
	)
	return c.JSON(http.StatusCreated, common.AuthResponse{
		ID:        objectID,
		Email:     ur.Email,
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
	})
}
