package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ExampleTestSuite struct {
	suite.Suite
	server *echo.Echo
	db     *mongo.Database
}

func (suite *ExampleTestSuite) SetupTest() {
	testCtx := context.Background()

	client, err := mongo.Connect(testCtx, options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
	if err != nil {
		panic(err)
	}
	suite.db = client.Database("togglelabs_test")
	suite.server = echo.New()
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
	suite.server.Close()
}

func (suite *ExampleTestSuite) TestExample() {
	t := suite.T()
	collection := suite.db.Collection("example")
	r := handlers.ExampleRecord{
		Name: "fizi",
	}

	_, err := collection.InsertOne(context.Background(), r)

	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/example", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	h := handlers.NewExampleRouter(suite.db)
	c := suite.server.NewContext(req, rec)
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

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(ExampleTestSuite))
}
