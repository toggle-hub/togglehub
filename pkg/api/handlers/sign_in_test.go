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
	"github.com/Roll-Play/togglelabs/pkg/api/handlers/fixtures"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SignInHandlerTestSuite struct {
	testutils.DefaultTestSuite
	db *mongo.Database
}

func (suite *SignInHandlerTestSuite) SetupTest() {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
	if err != nil {
		panic(err)
	}

	suite.db = client.Database(config.TestDBName)
	suite.Server = echo.New()
	logger, _ := logger.NewZapLogger()
	h := handlers.NewSignInHandler(suite.db, logger)
	suite.Server.POST("/signin", h.PostSignIn)
}

func (suite *SignInHandlerTestSuite) AfterTest(_, _ string) {
	if err := suite.db.Drop(context.Background()); err != nil {
		panic(err)
	}
}

func (suite *SignInHandlerTestSuite) TearDownSuite() {
	if err := suite.db.Client().Disconnect(context.Background()); err != nil {
		panic(err)
	}

	suite.Server.Close()
}

func (suite *SignInHandlerTestSuite) TestSignInHandlerSuccess() {
	t := suite.T()

	user := fixtures.CreateUser("fizi@gmail.com", "", "", "", suite.db)

	requestBody := []byte(`{
		"email": "fizi@gmail.com",
		"password": "big_secret_password"
	}`)

	request := httptest.NewRequest(http.MethodPost, "/signin", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)
	var response common.AuthResponse

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, user.ID, response.ID)
	assert.Equal(t, user.Email, response.Email)
	assert.Equal(t, user.FirstName, response.FirstName)
	assert.Equal(t, user.LastName, response.LastName)

	defer recorder.Result().Body.Close()
	cookie := recorder.Result().Cookies()[0]
	assert.NotNil(t, cookie)
	assert.NotEmpty(t, cookie.Value)
}

func (suite *SignInHandlerTestSuite) TestSignInHandlerNotFound() {
	t := suite.T()

	requestBody := []byte(`{
		"email": "fizi@gmail.com",
		"password": "123123123"
	}`)

	request := httptest.NewRequest(http.MethodPost, "/signin", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
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

func (suite *SignInHandlerTestSuite) TestSignInHandlerUnauthorized() {
	t := suite.T()

	fixtures.CreateUser("fizi@gmail.com", "123123123", "fizi", "valores", suite.db)

	requestBody := []byte(`{
		"email": "fizi@gmail.com",
		"password": "wrongpass"
	}`)

	request := httptest.NewRequest(http.MethodPost, "/signin", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)
	var response apierrors.Error

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	}, response)
}

func TestSignInHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(SignInHandlerTestSuite))
}
