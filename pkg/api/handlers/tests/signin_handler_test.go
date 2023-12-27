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
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
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

	collection := suite.db.Collection(models.UserCollectionName)

	p, err := apiutils.EncryptPassword("123123")
	assert.NoError(t, err)

	r := models.UserRecord{
		Email:     "fizi@gmail.com",
		Password:  p,
		FirstName: "fizi",
		LastName:  "valores",
	}
	_, err = collection.InsertOne(context.Background(), r)

	assert.NoError(t, err)

	requestBody := []byte(`{
		"email": "fizi@gmail.com",
		"password": "123123"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/signin", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	h := handlers.NewSignInHandler(suite.db)
	c := suite.Server.NewContext(req, rec)
	var jsonRes common.AuthResponse

	assert.NoError(t, h.PostSignIn(c))

	var ur models.UserRecord
	assert.NoError(t, collection.FindOne(context.Background(),
		bson.D{{
			Key:   "email",
			Value: "fizi@gmail.com",
		}}).Decode(&ur))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, jsonRes.ID, ur.ID)
	assert.Equal(t, jsonRes.Email, ur.Email)
	assert.Equal(t, jsonRes.FirstName, ur.FirstName)
	assert.Equal(t, jsonRes.LastName, ur.LastName)
}

func (suite *SignInHandlerTestSuite) TestSignInHandlerNotFound() {
	t := suite.T()

	requestBody := []byte(`{
		"email": "fizi@gmail.com",
		"password": "123123"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/signin", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	h := handlers.NewSignInHandler(suite.db)
	c := suite.Server.NewContext(req, rec)
	var jsonRes apierrors.Error

	assert.NoError(t, h.PostSignIn(c))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, jsonRes, apierrors.Error{
		Error:   http.StatusText(http.StatusNotFound),
		Message: apierrors.NotFoundError,
	})
}

func (suite *SignInHandlerTestSuite) TestSignInHandlerUnauthorized() {
	t := suite.T()

	collection := suite.db.Collection(models.UserCollectionName)

	p, err := apiutils.EncryptPassword("123123")
	assert.NoError(t, err)

	r := models.UserRecord{
		Email:     "fizi@gmail.com",
		Password:  p,
		FirstName: "fizi",
		LastName:  "valores",
	}
	_, err = collection.InsertOne(context.Background(), r)

	assert.NoError(t, err)

	requestBody := []byte(`{
		"email": "fizi@gmail.com",
		"password": "wrongpass"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/signin", bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	h := handlers.NewSignInHandler(suite.db)
	c := suite.Server.NewContext(req, rec)
	var jsonRes apierrors.Error

	assert.NoError(t, h.PostSignIn(c))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, jsonRes, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	})
}

func TestSignInHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(SignInHandlerTestSuite))
}
