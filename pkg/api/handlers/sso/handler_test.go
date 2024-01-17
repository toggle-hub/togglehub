package ssohandler_test

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers/fixtures"
	ssohandler "github.com/Roll-Play/togglelabs/pkg/api/handlers/sso"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"
)

type SsoTestSuite struct {
	testutils.DefaultTestSuite
	db *mongo.Database
}

func (suite *SsoTestSuite) SetupTest() {
	testCtx := context.Background()
	if err := godotenv.Load("../../../../.env.test"); err != nil {
		log.Panic(err)
	}

	client, err := mongo.Connect(testCtx, options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
	if err != nil {
		panic(err)
	}
	suite.db = client.Database("togglelabs_test")

	suite.Server = echo.New()
}

func (suite *SsoTestSuite) AfterTest(_, _ string) {
	if err := suite.db.Drop(context.Background()); err != nil {
		panic(err)
	}
}

func (suite *SsoTestSuite) TearDownSuite() {
	if err := suite.db.Client().Disconnect(context.Background()); err != nil {
		panic(err)
	}
	suite.Server.Close()
}

func (suite *SsoTestSuite) TestSSoHandlerNewUserSuccess() {
	t := suite.T()

	mockOAuthClient := &fixtures.MockOAuthClient{
		ExchangeFunc: func(ctx context.Context, code string) (*oauth2.Token, error) {
			return &oauth2.Token{
				AccessToken: "mockAccessToken",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			}, nil
		},
	}

	model := usermodel.New(suite.db)

	request := httptest.NewRequest(http.MethodGet, "/callback", nil)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()
	urlq := request.URL.Query()
	urlq.Add("state", "random-string")
	urlq.Add("code", "code")
	request.URL.RawQuery = urlq.Encode()

	logger, _ := logger.NewZapLogger()

	h := ssohandler.New(suite.db, &oauth2.Config{}, logger, &fixtures.MockHTTPClient{}, mockOAuthClient)
	assert.NotNil(t, h)
	suite.Server.GET("/callback", h.Callback)
	suite.Server.ServeHTTP(recorder, request)
	var response common.AuthResponse

	ur, err := model.FindByEmail(context.Background(), "test@test.com")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, ur.Email, response.Email)
	assert.NotEmpty(t, response.Token)
}

func (suite *SsoTestSuite) TestSSoHandlerExistingUserSuccess() {
	t := suite.T()
	mockOAuthClient := &fixtures.MockOAuthClient{
		ExchangeFunc: func(ctc context.Context, code string) (*oauth2.Token, error) {
			return &oauth2.Token{
				AccessToken: "mockAccessToken",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			}, nil
		},
	}

	user := fixtures.CreateUser("test@test.com", "", "", "", suite.db)

	request := httptest.NewRequest(http.MethodGet, "/callback", nil)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()
	urlq := request.URL.Query()
	urlq.Add("state", "random-string")
	urlq.Add("code", "code")
	request.URL.RawQuery = urlq.Encode()

	logger, _ := logger.NewZapLogger()

	h := ssohandler.New(suite.db, &oauth2.Config{}, logger, &fixtures.MockHTTPClient{}, mockOAuthClient)
	suite.Server.GET("/callback", h.Callback)
	suite.Server.ServeHTTP(recorder, request)
	var response common.AuthResponse

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, user.Email, response.Email)
	assert.NotEmpty(t, response.Token)
}

func TestSsoHandler(t *testing.T) {
	suite.Run(t, new(SsoTestSuite))
}
