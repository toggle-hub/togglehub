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
	"github.com/Roll-Play/togglelabs/pkg/api/handlers/tests/fixtures"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type SignUpHandlerTestSuite struct {
	testutils.DefaultTestSuite
	db *mongo.Database
}

func (suite *SignUpHandlerTestSuite) SetupTest() {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
	if err != nil {
		panic(err)
	}

	suite.db = client.Database(config.TestDBName)
	suite.Server = echo.New()

	h := handlers.NewSignUpHandler(suite.db)
	suite.Server.POST("/signup", h.PostUser)
}

func (suite *SignUpHandlerTestSuite) AfterTest(_, _ string) {
	if err := suite.db.Drop(context.Background()); err != nil {
		panic(err)
	}
}

func (suite *SignUpHandlerTestSuite) TearDownSuite() {
	if err := suite.db.Client().Disconnect(context.Background()); err != nil {
		panic(err)
	}

	suite.Server.Close()
}

func (suite *SignUpHandlerTestSuite) TestSignUpHandlerSuccess() {
	t := suite.T()

	model := models.NewUserModel(suite.db)

	requestBody := []byte(`{
		"email": "fizi@gmail.com",
		"password": "123123123"
	}`)

	request := httptest.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)
	var response common.AuthResponse

	ur, err := model.FindByEmail(context.Background(), "fizi@gmail.com")
	assert.NoError(t, err)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, ur.Email, response.Email)
	assert.Equal(t, ur.FirstName, response.FirstName)
	assert.Equal(t, ur.LastName, response.LastName)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(ur.Password), []byte("123123123")))
}

func (suite *SignUpHandlerTestSuite) TestSignUpHandlerUnsuccessful() {
	t := suite.T()

	user := fixtures.CreateUser("fizi@gmail.com", "123123123", "", "", suite.db)

	requestBody, err := json.Marshal(user)
	assert.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer(requestBody))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)
	var response apierrors.Error

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, response, apierrors.Error{
		Error:   http.StatusText(http.StatusConflict),
		Message: apierrors.EmailConflictError,
	})
}

func TestSignUpHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(SignUpHandlerTestSuite))
}
