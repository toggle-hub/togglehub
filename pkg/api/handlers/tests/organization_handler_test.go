package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
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

func (suite *OrganizationHandlerTestSuite) TestPostSigninHandlerSuccess() {
	t := suite.T()

	model := models.NewOrganizationModel(suite.db)
	userModel := models.NewUserModel(suite.db)
	r, err := models.NewUserRecord(
		"fizi@gmail.com",
		"123123",
		"fizi",
		"valores",
	)
	assert.NoError(t, err)

	userID, err := userModel.InsertOne(context.Background(), r)
	assert.Error(t, err)

	token, err := apiutils.CreateJWT(userID, time.Second*120)
	assert.NoError(t, err)

	requestBody := []byte(`{
		"name": "the company",
	}`)

	req := httptest.NewRequest(http.MethodPost, "/organization", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	h := handlers.NewSignInHandler(suite.db)
	suite.Server.POST("/organization", middlewares.AuthMiddleware(h.PostSignIn))
	suite.Server.ServeHTTP(rec, req)

	var jsonRes models.OrganizationRecord

	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, http.StatusCreated, rec.Code)

	organization, err := model.FindByID(context.Background(), jsonRes.ID)
	assert.NoError(t, err)
	assert.Equal(t, jsonRes, organization)
}
