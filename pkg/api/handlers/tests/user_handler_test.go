package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserHandlerTestSuite struct {
	testutils.DefaultTestSuite
	db *mongo.Database
}

func (suite *UserHandlerTestSuite) SetupTest() {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
	if err != nil {
		panic(err)
	}

	suite.db = client.Database(config.TestDBName)
	suite.Server = echo.New()
}

func (suite *UserHandlerTestSuite) AfterTest(_, _ string) {
	if err := suite.db.Drop(context.Background()); err != nil {
		panic(err)
	}
}

func (suite *UserHandlerTestSuite) TearDownSuite() {
	if err := suite.db.Client().Disconnect(context.Background()); err != nil {
		panic(err)
	}

	suite.Server.Close()
}

func (suite *UserHandlerTestSuite) TestUserPatchHandlerSuccess() {
	t := suite.T()

	model := models.NewUserModel(suite.db.Collection(models.UserCollectionName))

	r, err := models.NewUserRecord(
		"fizi@gmail.com",
		"123123",
		"",
		"",
	)
	assert.NoError(t, err)

	userId, err := model.InsertOne(context.Background(), r)
	assert.NoError(t, err)

	patchInfo := handlers.UserPatchRequest{
		FirstName: "fizi",
		LastName:  "valores",
	}
	requestBody, err := json.Marshal(patchInfo)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := suite.Server.NewContext(req, rec)
	c.Set("user", userId)

	h := handlers.NewUserHandler(suite.db)
	var jsonRes common.PatchResponse

	assert.NoError(t, h.PatchUser(c))

	ur, err := model.FindByEmail(context.Background(), "fizi@gmail.com")
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, ur.Email, jsonRes.Email)
	assert.Equal(t, ur.FirstName, jsonRes.FirstName)
	assert.Equal(t, ur.LastName, jsonRes.LastName)
}

func (suite *UserHandlerTestSuite) TestUserPatchHandlerNotFound() {
	t := suite.T()
	patchInfo := handlers.UserPatchRequest{
		FirstName: "fizi",
		LastName:  "valores",
	}
	requestBody, err := json.Marshal(patchInfo)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := suite.Server.NewContext(req, rec)
	c.Set("user", primitive.NewObjectID())

	h := handlers.NewUserHandler(suite.db)
	var jsonRes apierrors.Error

	assert.NoError(t, h.PatchUser(c))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, jsonRes, apierrors.Error{
		Error:   http.StatusText(http.StatusNotFound),
		Message: apierrors.NotFoundError,
	})
}

func (suite *UserHandlerTestSuite) TestUserPatchHandlerOnlyChangesAllowedFields() {
	t := suite.T()
	model := models.NewUserModel(suite.db.Collection(models.UserCollectionName))

	r, err := models.NewUserRecord(
		"fizi@gmail.com",
		"123123",
		"",
		"",
	)
	assert.NoError(t, err)

	userId, err := model.InsertOne(context.Background(), r)
	assert.NoError(t, err)

	requestBody := []byte(
		`{
			"first_name": "fizi",
			"last_name": "valores",
			"email": "new@email.mail"
		}`)

	req := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := suite.Server.NewContext(req, rec)
	c.Set("user", userId)

	h := handlers.NewUserHandler(suite.db)
	var jsonRes common.PatchResponse

	assert.NoError(t, h.PatchUser(c))

	ur, err := model.FindByEmail(context.Background(), "fizi@gmail.com")
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, ur.Email, jsonRes.Email)
	assert.Equal(t, ur.FirstName, jsonRes.FirstName)
	assert.Equal(t, ur.LastName, jsonRes.LastName)
}

func TestUserHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(UserHandlerTestSuite))
}
