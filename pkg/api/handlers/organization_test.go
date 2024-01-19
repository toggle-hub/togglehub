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
	api_errors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers/fixtures"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	featureflagmodel "github.com/Roll-Play/togglelabs/pkg/models/feature_flag"
	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type OrganizationHandlerTestSuite struct {
	testutils.DefaultTestSuite
	db *mongo.Database
}

func (suite *OrganizationHandlerTestSuite) SetupTest() {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
	if err != nil {
		panic(err)
	}

	suite.db = client.Database(config.TestDBName)
	suite.Server = echo.New()

	logger, _ := logger.NewZapLogger()

	h := handlers.NewOrganizationHandler(suite.db, logger)
	suite.Server.POST("/organizations", middlewares.AuthMiddleware(h.PostOrganization))

	testGroup := suite.Server.Group("", middlewares.AuthMiddleware, middlewares.OrganizationMiddleware)
	testGroup.POST("/projects", h.PostProject)
}

func (suite *OrganizationHandlerTestSuite) AfterTest(_, _ string) {
	if err := suite.db.Drop(context.Background()); err != nil {
		panic(err)
	}
}

func (suite *OrganizationHandlerTestSuite) TearDownSuite() {
	if err := suite.db.Client().Disconnect(context.Background()); err != nil {
		panic(err)
	}

	suite.Server.Close()
}

func (suite *OrganizationHandlerTestSuite) TestPostOrganizationHandlerSuccess() {
	t := suite.T()

	model := organizationmodel.New(suite.db)

	user := fixtures.CreateUser("", "", "", "", suite.db)
	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	requestBody := []byte(`{
		"name": "the company"
	}`)

	request := httptest.NewRequest(http.MethodPost, "/organizations", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response organizationmodel.OrganizationRecord

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusCreated, recorder.Code)

	organization, err := model.FindByID(context.Background(), response.ID)
	assert.NoError(t, err)
	assert.Equal(t, organization.ID, response.ID)
	assert.Equal(t, organization.Members, response.Members)
	assert.Equal(t, organization.Name, response.Name)
}

func (suite *OrganizationHandlerTestSuite) TestPostProjectHandlerSuccess() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, suite.db)

	newProject := handlers.ProjectPostRequest{
		Name:        "project",
		Description: "project description",
	}
	requestBody, err := json.Marshal(newProject)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"/projects",
		bytes.NewBuffer(requestBody),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response featureflagmodel.Revision
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))

	model := organizationmodel.New(suite.db)
	savedOrganization, err := model.FindByID(context.Background(), organization.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(savedOrganization.Projects))
	savedProject := savedOrganization.Projects[0]
	assert.Equal(t, newProject.Name, savedProject.Name)
	assert.Equal(t, newProject.Description, savedProject.Description)
}

func (suite *OrganizationHandlerTestSuite) TestPostProjectUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	unauthorizedUser := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			unauthorizedUser,
			organizationmodel.ReadOnly,
		),
	}, suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"/projects",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response api_errors.Error

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, api_errors.Error{
		Error:   http.StatusText(http.StatusForbidden),
		Message: api_errors.ForbiddenError,
	}, response)
}

func TestOrganizationHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(OrganizationHandlerTestSuite))
}
