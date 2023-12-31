package testutils

import (
	"github.com/Roll-Play/togglelabs/pkg/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type DefaultTestSuite struct {
	suite.Suite
	Server *echo.Echo
}

func CreateTestUser(db *mongo.Database) (primitive.ObjectID, error) {
	model := models.NewUserModel()
}
