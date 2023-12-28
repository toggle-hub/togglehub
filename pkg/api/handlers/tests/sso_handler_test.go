package handlers_test

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/models"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"
)

type SsoTestSuite struct {
	testutils.DefaultTestSuite
	server *echo.Echo
	db     *mongo.Database
}

type MockHTTPClient struct{}

func (c *MockHTTPClient) Get(_ string) (*http.Response, error) {
	response := httptest.NewRecorder()
	userInfo := handlers.UserInfo{
		SsoID: "12345",
		Email: "test@test.com",
	}
	body, err := json.Marshal(userInfo)
	if err != nil {
		panic(err)
	}

	response.Body.Write(body)
	return response.Result(), nil
}

type MockOAuthClient struct {
	ExchangeFunc func(ctx context.Context, code string) (*oauth2.Token, error)
}

func (m *MockOAuthClient) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return m.ExchangeFunc(ctx, code)
}

func (suite *SsoTestSuite) SetupTest() {
	testCtx := context.Background()
	if err := godotenv.Load("../../../../.env"); err != nil {
		log.Panic(err)
	}

	client, err := mongo.Connect(testCtx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		panic(err)
	}
	suite.db = client.Database("togglelabs_test")
	suite.server = echo.New()
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
	suite.server.Close()
}

func (suite *SignUpHandlerTestSuite) TestSSoHandlerNewUserSuccess() {
	t := suite.T()
	collection := suite.db.Collection(models.UserCollectionName)
	mockOAuthClient := &MockOAuthClient{
		ExchangeFunc: func(ctc context.Context, code string) (*oauth2.Token, error) {
			return &oauth2.Token{
				AccessToken: "mockAccessToken",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/callback", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	urlq := req.URL.Query()
	urlq.Add("state", "random-string")
	urlq.Add("code", "code")
	req.URL.RawQuery = urlq.Encode()
	c := suite.Server.NewContext(req, rec)

	h := handlers.NewSsoHandlerForTest(suite.db, &MockHTTPClient{}, mockOAuthClient)
	var jsonRes common.AuthResponse

	assert.NoError(t, h.Callback(c))

	var ur models.UserRecord
	assert.NoError(t, collection.FindOne(context.Background(),
		bson.D{{
			Key:   "email",
			Value: "test@test.com",
		}}).Decode(&ur))

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, jsonRes.Email, ur.Email)
	assert.NotEmpty(t, jsonRes.Token)
}

func (suite *SignUpHandlerTestSuite) TestSSoHandlerExistingUserSuccess() {
	t := suite.T()
	collection := suite.db.Collection(models.UserCollectionName)
	mockOAuthClient := &MockOAuthClient{
		ExchangeFunc: func(ctc context.Context, code string) (*oauth2.Token, error) {
			return &oauth2.Token{
				AccessToken: "mockAccessToken",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			}, nil
		},
	}

	r := handlers.UserInfo{
		Email: "test@test.com",
	}
	_, err := collection.InsertOne(context.Background(), r)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/callback", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	urlq := req.URL.Query()
	urlq.Add("state", "random-string")
	urlq.Add("code", "code")
	req.URL.RawQuery = urlq.Encode()
	c := suite.Server.NewContext(req, rec)

	h := handlers.NewSsoHandlerForTest(suite.db, &MockHTTPClient{}, mockOAuthClient)
	var jsonRes common.AuthResponse

	assert.NoError(t, h.Callback(c))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, jsonRes.Email, r.Email)
	assert.NotEmpty(t, jsonRes.Token)
}

func TestSsoHandler(t *testing.T) {
	suite.Run(t, new(SsoTestSuite))
}
