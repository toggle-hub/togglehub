package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type FeatureFlagHandlerTestSuite struct {
	testutils.DefaultTestSuite
	db *mongo.Database
}

func (suite *FeatureFlagHandlerTestSuite) SetupTest() {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://test:test@localhost:27017"))
	if err != nil {
		panic(err)
	}

	suite.db = client.Database(config.TestDBName)
	suite.Server = echo.New()

	h := handlers.NewFeatureFlagHandler(suite.db)
	suite.Server.POST("organization/:orgID/featureFlag", middlewares.AuthMiddleware(h.PostFeatureFlag))
}

func (suite *FeatureFlagHandlerTestSuite) AfterTest(_, _ string) {
	if err := suite.db.Drop(context.Background()); err != nil {
		panic(err)
	}
}

func (suite *FeatureFlagHandlerTestSuite) TearDownSuite() {
	if err := suite.db.Client().Disconnect(context.Background()); err != nil {
		panic(err)
	}

	suite.Server.Close()
}

func (suite *FeatureFlagHandlerTestSuite) TestSignUpHandlerSuccess() {
	t := suite.T()

	rule := models.Rule{
		Predicate: "attr: rule",
		Value:     "false",
		Env:       "prd",
		IsEnabled: true,
	}
	ffr := models.FeatureFlagRequest{
		Type:         models.Boolean,
		DefaultValue: "true",
		Rules: []models.Rule{
			rule,
		},
	}
	requestBody, err := json.Marshal(ffr)
	assert.NoError(t, err)

	userID, orgID, err := setupUserAndOrg("fizi@valores.com", "org", suite.db)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(userID, time.Second*120)
	assert.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodPost,
		"/organization/"+orgID.Hex()+"/featureFlag",
		bytes.NewBuffer(requestBody),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)

	var jsonRes models.FeatureFlagRecord

	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, userID, jsonRes.UserID)
	assert.Equal(t, orgID, jsonRes.OrgID)
	assert.Equal(t, ffr.Type, jsonRes.Type)

	assert.NotEmpty(t, jsonRes.Revisions)
	responseRevision := jsonRes.Revisions[0]
	assert.Equal(t, userID, responseRevision.UserID)
	assert.Equal(t, ffr.DefaultValue, responseRevision.DefaultValue)
	assert.Equal(t, models.Draft, responseRevision.Status)

	assert.NotEmpty(t, responseRevision.Rules)
	responseRule := responseRevision.Rules[0]
	assert.Equal(t, rule.Predicate, responseRule.Predicate)
	assert.Equal(t, rule.Value, responseRule.Value)
	assert.Equal(t, rule.Env, responseRule.Env)
	assert.Equal(t, rule.IsEnabled, responseRule.IsEnabled)
}

func TestFeatureFlagHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(FeatureFlagHandlerTestSuite))
}

func setupUserAndOrg(email, orgName string, db *mongo.Database) (primitive.ObjectID, primitive.ObjectID, error) {
	uModel := models.NewUserModel(db)
	userRecord, err := models.NewUserRecord(email, "default", "fizi", "valores")
	if err != nil {
		return primitive.NewObjectID(), primitive.NewObjectID(), err
	}

	userID, err := uModel.InsertOne(context.Background(), userRecord)
	if err != nil {
		return primitive.NewObjectID(), primitive.NewObjectID(), err
	}
	userRecord.ID = userID

	oModel := models.NewOrganizationModel(db)
	orgRecord := models.NewOrganizationRecord(orgName, userRecord)
	orgID, err := oModel.InsertOne(context.Background(), orgRecord)
	if err != nil {
		return primitive.NewObjectID(), primitive.NewObjectID(), err
	}

	return userID, orgID, nil
}
