package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
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

	h := handlers.NewUserHandler(suite.db)
	suite.Server.PATCH("/user", middlewares.AuthMiddleware(h.PatchUser))
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

	model := models.NewUserModel(suite.db)

	r, err := models.NewUserRecord(
		"fizi@gmail.com",
		"123123",
		"",
		"",
	)
	assert.NoError(t, err)

	userID, err := model.InsertOne(context.Background(), r)
	assert.NoError(t, err)

	patchInfo := handlers.UserPatchRequest{
		FirstName: "fizi",
		LastName:  "valores",
	}
	requestBody, err := json.Marshal(patchInfo)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(userID, time.Second*120)

	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)
	var jsonRes handlers.UserPatchResponse

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

	token, err := apiutils.CreateJWT(primitive.NewObjectID(), time.Second*120)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)
	var jsonRes apierrors.Error

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, jsonRes, apierrors.Error{
		Error:   http.StatusText(http.StatusNotFound),
		Message: apierrors.NotFoundError,
	})
}

func (suite *UserHandlerTestSuite) TestUserPatchHandlerOnlyChangesAllowedFields() {
	t := suite.T()
	model := models.NewUserModel(suite.db)

	r, err := models.NewUserRecord(
		"fizi@gmail.com",
		"123123",
		"",
		"",
	)
	assert.NoError(t, err)

	userID, err := model.InsertOne(context.Background(), r)
	assert.NoError(t, err)

	requestBody := []byte(
		`{
			"first_name": "fizi",
			"last_name": "valores",
			"email": "new@email.mail"
		}`)

	token, err := apiutils.CreateJWT(userID, time.Second*120)
	assert.NoError(t, err)
	req := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))

	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)

	var jsonRes handlers.UserPatchResponse

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
