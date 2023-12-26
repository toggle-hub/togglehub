package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/config"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ExampleTestSuite struct {
	testutils.DefaultTestSuite
	db *mongo.Database
}

func (suite *ExampleTestSuite) SetupTest() {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
	if err != nil {
		panic(err)
	}
	suite.db = client.Database(config.TestDBName)
	suite.Server = echo.New()
}

func (suite *ExampleTestSuite) AfterTest(_, _ string) {
	if err := suite.db.Drop(context.Background()); err != nil {
		panic(err)
	}
}

func (suite *ExampleTestSuite) TearDownSuite() {
	if err := suite.db.Client().Disconnect(context.Background()); err != nil {
		panic(err)
	}
	suite.Server.Close()
}

func (suite *ExampleTestSuite) TestExampleHandlerReturnsList() {
	t := suite.T()
	collection := suite.db.Collection(handlers.ExampleCollectionName)
	r := handlers.ExampleRecord{
		Name: "fizi",
	}

	_, err := collection.InsertOne(context.Background(), r)

	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/example", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	h := handlers.NewExampleRouter(suite.db)
	c := suite.Server.NewContext(req, rec)
	var jsonRes handlers.ExampleListResponse

	assert.NoError(t, h.GetExamples(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, handlers.ExampleListResponse{
		Data: []handlers.ExampleRecord{
			r,
		},
		Page:     1,
		PageSize: 10,
		Total:    1,
	}, jsonRes)
}

func (suite *ExampleTestSuite) TestExampleHandlerReturnsEmptyList() {
	t := suite.T()

	collection := suite.db.Collection(handlers.ExampleCollectionName)
	r := handlers.ExampleRecord{
		Name: "fizi",
	}

	_, err := collection.InsertOne(context.Background(), r)

	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/example?page=2", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	h := handlers.NewExampleRouter(suite.db)
	c := suite.Server.NewContext(req, rec)
	var jsonRes handlers.ExampleListResponse

	assert.NoError(t, h.GetExamples(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, handlers.ExampleListResponse{
		Data:     []handlers.ExampleRecord{},
		Page:     2,
		PageSize: 10,
		Total:    0,
	}, jsonRes)
}

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(ExampleTestSuite))
}
