package handlers_test

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
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
	server *echo.Echo
	db     *mongo.Database
}

type MockHTTPClient struct{}

func (c *MockHTTPClient) Get(url string) (*http.Response, error) {
	response := httptest.NewRecorder()
	response.Body.Write([]byte("Mock response"))
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
	suite.db.Drop(context.Background())
}

func (suite *SsoTestSuite) TearDownSuite() {
	suite.db.Client().Disconnect(context.Background())
	suite.server.Close()
}

func (suite *SsoTestSuite) TestCallback() {
	t := suite.T()

	mockOAuthClient := &MockOAuthClient{
		ExchangeFunc: func(ctc context.Context, code string) (*oauth2.Token, error) {
			return &oauth2.Token{
				AccessToken: "mockAccessToken",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			}, nil
		},
	}

	mockClient := &MockHTTPClient{}
	codedInfo, err := handlers.GetUserData("random-string", "test", mockOAuthClient, mockClient)
	assert.NoError(t, err)

	assert.Equal(t, string(codedInfo[:]), "Mock response")
}

func TestSsoHandler(t *testing.T) {
	suite.Run(t, new(SsoTestSuite))
}
