package handlers

// import (
// 	"bytes"
// 	"context"
// 	"net/http"
// 	"net/http/httptest"

// 	"github.com/Roll-Play/togglelabs/pkg/api/common"
// 	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
// 	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
// 	"github.com/Roll-Play/togglelabs/pkg/config"
// 	"github.com/Roll-Play/togglelabs/pkg/models"
// 	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
// 	"github.com/labstack/echo/v4"
// 	"github.com/stretchr/testify/assert"
// 	"go.mongodb.org/mongo-driver/mongo"
// 	"go.mongodb.org/mongo-driver/mongo/options"
// )

// type OrganizationHandlerTestSuite struct {
// 	testutils.DefaultTestSuite
// 	db *mongo.Database
// }

// func (suite *OrganizationHandlerTestSuite) SetupTest() {
// 	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
// 	if err != nil {
// 		panic(err)
// 	}

// 	suite.db = client.Database(config.TestDBName)
// 	suite.Server = echo.New()
// }

// func (suite *OrganizationHandlerTestSuite) AfterTest(_, _ string) {
// 	if err := suite.db.Drop(context.Background()); err != nil {
// 		panic(err)
// 	}
// }

// func (suite *OrganizationHandlerTestSuite) TearDownSuite() {
// 	if err := suite.db.Client().Disconnect(context.Background()); err != nil {
// 		panic(err)
// 	}

// 	suite.Server.Close()
// }

// func (suite *OrganizationHandlerTestSuite) TestPostSigninHandlerSuccess() {
// 	t := suite.T()

// 	model := models.NewOrganizationModel(suite.db)

// 	requestBody := []byte(`{
// 		"name": "the company",
// 	}`)

// 	req := httptest.NewRequest(http.MethodPost, "/organization", bytes.NewBuffer(requestBody))
// 	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
// 	rec := httptest.NewRecorder()

// 	h := handlers.NewSignInHandler(suite.db)
// 	c := suite.Server.NewContext(req, rec)

// 	var jsonRes common.AuthResponse

// 	assert.NoError(t, middlewares.AuthMiddleware(h.PostSignIn(c)))
// }
