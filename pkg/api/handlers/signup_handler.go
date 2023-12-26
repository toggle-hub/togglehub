package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/config"
	apierror "github.com/Roll-Play/togglelabs/pkg/error"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const UserCollectionName = "user"

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

type SignUpResponse struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Email     string             `json:"email" bson:"email"`
	FirstName string             `json:"first_name" bson:"first_name"`
	LastName  string             `json:"last_name" bson:"last_name"`
	Token     string             `json:"token"`
}

type UserRecord struct {
	ID        primitive.ObjectID  `json:"_id,omitempty" bson:"_id,omitempty"`
	Email     string              `json:"email" bson:"email"`
	Password  string              `json:"password" bson:"password"`
	FirstName string              `json:"first_name" bson:"first_name"`
	LastName  string              `json:"last_name" bson:"last_name"`
	CreatedAt primitive.Timestamp `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpadtedAt primitive.Timestamp `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.Timestamp `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

func NewUserRecord(req *SignUpRequest) (*UserRecord, error) {
	password, err := apiutils.EncryptPassword(req.Password)
	if err != nil {
		return nil, err
	}

	return &UserRecord{
		Email:     req.Email,
		Password:  password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		CreatedAt: primitive.Timestamp{T: uint32(time.Now().Unix())},
		UpadtedAt: primitive.Timestamp{T: uint32(time.Now().Unix())},
	}, nil
}

func (sh *SignUpHandler) PostUser(c echo.Context) error {
	req := new(SignUpRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	var foundRecord UserRecord
	collection := sh.db.Collection(UserCollectionName)
	err := collection.FindOne(context.Background(), bson.D{{Key: "email", Value: req.Email}}).Decode(&foundRecord)
	if err == nil {
		return apierror.CustomError(c, http.StatusConflict, apierror.EmailConflictError)
	}

	ur, err := NewUserRecord(req)
	if err != nil {
		return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
	}

	result, err := collection.InsertOne(context.Background(), ur)
	if err != nil {
		return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
	}

	objectID := result.InsertedID
	oID, ok := objectID.(primitive.ObjectID)
	if !ok {
		return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
	}

	token, err := apiutils.CreateJWT(oID, config.JWTExpireTime)
	if err != nil {
		return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
	}

	return c.JSON(http.StatusCreated, SignUpResponse{
		ID:        oID,
		Email:     ur.Email,
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
		Token:     token,
	})
}
