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

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers/fixtures"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
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
	logger, _ := logger.NewZapLogger()
	h := handlers.NewUserHandler(suite.db, logger)
	testGroup := suite.Server.Group("/user", middlewares.AuthMiddleware)
	testGroup.PATCH("", h.PatchUser)
	testGroup.GET("", h.GetUser)
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

func (suite *UserHandlerTestSuite) TestUserGetHandlerUnauthorized() {
	t := suite.T()

	request := httptest.NewRequest(http.MethodGet, "/user", nil)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func (suite *UserHandlerTestSuite) TestUserGetHandlerSuccess() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		)}, nil, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, "/user", nil)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)
	response := new(usermodel.UserWithOrganization)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), response))
	expected := &usermodel.UserWithOrganization{
		ID:        user.ID,
		OAuthID:   user.OAuthID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Organizations: []usermodel.UserOrganization{
			{
				ID:   organization.ID,
				Name: organization.Name,
			},
		},
	}
	assert.Equal(t, expected, response)
}

func (suite *UserHandlerTestSuite) TestUserPatchHandlerSuccess() {
	t := suite.T()

	model := usermodel.New(suite.db)

	user := fixtures.CreateUser("fizi@gmail.com", "", "", "", suite.db)

	patchInfo := handlers.UserPatchRequest{
		FirstName: "fizi",
		LastName:  "valores",
	}
	requestBody, err := json.Marshal(patchInfo)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)

	assert.NoError(t, err)

	request := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)
	var response handlers.UserPatchResponse

	ur, err := model.FindByEmail(context.Background(), "fizi@gmail.com")
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.NotEqual(t, user.UpdatedAt, ur.UpdatedAt)
	assert.Equal(t, ur.Email, response.Email)
	assert.Equal(t, ur.FirstName, response.FirstName)
	assert.Equal(t, ur.LastName, response.LastName)
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

	request := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)
	var response apierrors.Error

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusNotFound),
		Message: apierrors.NotFoundError,
	}, response)
}

func (suite *UserHandlerTestSuite) TestUserPatchHandlerOnlyChangesAllowedFields() {
	t := suite.T()
	model := usermodel.New(suite.db)

	user := fixtures.CreateUser("fizi@gmail.com", "", "", "", suite.db)

	requestBody := []byte(
		`{
			"first_name": "fizi",
			"last_name": "valores",
			"email": "new@email.mail"
		}`)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)
	request := httptest.NewRequest(http.MethodPatch, "/user", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))

	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response handlers.UserPatchResponse

	ur, err := model.FindByEmail(context.Background(), "fizi@gmail.com")
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, ur.Email, response.Email)
	assert.Equal(t, ur.FirstName, response.FirstName)
	assert.Equal(t, ur.LastName, response.LastName)
}

func TestUserHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(UserHandlerTestSuite))
}
