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

	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers/tests/fixtures"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
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

	h := handlers.NewOrganizationHandler(suite.db)
	suite.Server.POST("/organization", middlewares.AuthMiddleware(h.PostOrganization))
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

	model := models.NewOrganizationModel(suite.db)

	user := fixtures.CreateUser("", "", "", "", suite.db)
	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	requestBody := []byte(`{
		"name": "the company"
	}`)

	request := httptest.NewRequest(http.MethodPost, "/organization", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response models.OrganizationRecord

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusCreated, recorder.Code)

	organization, err := model.FindByID(context.Background(), response.ID)
	assert.NoError(t, err)
	assert.Equal(t, response.ID, organization.ID)
	assert.Equal(t, response.Members, organization.Members)
	assert.Equal(t, response.Name, organization.Name)
}

func TestOrganizationHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(OrganizationHandlerTestSuite))
}
