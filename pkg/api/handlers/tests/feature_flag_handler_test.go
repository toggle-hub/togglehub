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

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
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
	suite.Server.POST("organization/:orgID/featureFlag/:ffID", middlewares.AuthMiddleware(h.PostRevision))
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

func (suite *FeatureFlagHandlerTestSuite) TestPostFeatureFlagSuccess() {
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

	userID, orgID, err := setupUserAndOrg("fizi@valores.com", "org", models.Admin, suite.db)
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

func (suite *FeatureFlagHandlerTestSuite) TestPostFeatureFlagUnauthorized() {
	t := suite.T()

	userID, orgID, err := setupUserAndOrg("fizi@valores.com", "org", models.ReadOnly, suite.db)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(userID, time.Second*120)
	assert.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodPost,
		"/organization/"+orgID.Hex()+"/featureFlag",
		nil,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)

	var jsonRes apierrors.Error

	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, jsonRes, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	})
}

func (suite *FeatureFlagHandlerTestSuite) TestPostRevisionSuccess() {
	t := suite.T()
	userID, orgID, err := setupUserAndOrg("fizi@valores.com", "org", models.Admin, suite.db)
	assert.NoError(t, err)

	r := models.Rule{
		Predicate: "attr: rule",
		Value:     "false",
		Env:       "dev",
		IsEnabled: true,
	}
	ff := &models.FeatureFlagRequest{
		Type:         models.Boolean,
		DefaultValue: "false",
		Rules: []models.Rule{
			r,
		},
	}
	ffr := models.NewFeatureFlagRecord(ff, orgID, userID)
	ffm := models.NewFeatureFlagModel(suite.db)
	ffID, err := ffm.InsertOne(context.Background(), ffr)
	assert.NoError(t, err)

	nr := models.Rule{
		Predicate: "attr: newRule",
		Value:     "true",
		Env:       "prd",
		IsEnabled: true,
	}
	rr := models.RevisionRequest{
		DefaultValue: "true",
		Rules: []models.Rule{
			nr,
		},
	}
	requestBody, err := json.Marshal(rr)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(userID, time.Second*120)
	assert.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodPost,
		"/organization/"+orgID.Hex()+"/featureFlag/"+ffID.Hex(),
		bytes.NewBuffer(requestBody),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)

	var jsonRes models.Revision

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, userID, jsonRes.UserID)
	assert.Equal(t, rr.DefaultValue, jsonRes.DefaultValue)
	assert.Equal(t, models.Draft, jsonRes.Status)

	savedFF, err := ffm.FindByID(context.Background(), ffID)
	assert.NoError(t, err)

	savedRevisions := savedFF.Revisions
	assert.Equal(t, len(savedRevisions), 2)

	//Make sure original revision is the same
	originalRevision := savedRevisions[0]
	assert.Equal(t, userID, originalRevision.UserID)
	assert.Equal(t, ff.DefaultValue, originalRevision.DefaultValue)
	assert.Equal(t, models.Draft, originalRevision.Status)
	assert.NotEmpty(t, originalRevision.Rules)
	originalRule := originalRevision.Rules[0]
	assert.Equal(t, r.Predicate, originalRule.Predicate)
	assert.Equal(t, r.Value, originalRule.Value)
	assert.Equal(t, r.Env, originalRule.Env)
	assert.Equal(t, r.IsEnabled, originalRule.IsEnabled)

	//Check the new revision
	newRevision := savedRevisions[1]
	assert.Equal(t, userID, newRevision.UserID)
	assert.Equal(t, rr.DefaultValue, newRevision.DefaultValue)
	assert.Equal(t, models.Draft, newRevision.Status)
	assert.NotEmpty(t, newRevision.Rules)
	newRule := newRevision.Rules[0]
	assert.Equal(t, nr.Predicate, newRule.Predicate)
	assert.Equal(t, nr.Value, newRule.Value)
	assert.Equal(t, nr.Env, newRule.Env)
	assert.Equal(t, nr.IsEnabled, newRule.IsEnabled)
}

func (suite *FeatureFlagHandlerTestSuite) TestPostRevisionUnauthorized() {
	t := suite.T()

	userID, orgID, err := setupUserAndOrg("fizi@valores.com", "org", models.ReadOnly, suite.db)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(userID, time.Second*120)
	assert.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodPost,
		"/organization/"+orgID.Hex()+"/featureFlag/"+primitive.NewObjectID().Hex(),
		nil,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)

	var jsonRes apierrors.Error

	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, jsonRes, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	})
}

func TestFeatureFlagHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(FeatureFlagHandlerTestSuite))
}

func setupUserAndOrg(
	email, orgName string,
	permission models.PermissionLevelEnum,
	db *mongo.Database) (primitive.ObjectID, primitive.ObjectID, error) {
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
	orgRecord.Members = []models.OrganizationMember{
		models.OrganizationMember{
			User:            *userRecord,
			PermissionLevel: permission,
		},
	}
	orgID, err := oModel.InsertOne(context.Background(), orgRecord)
	if err != nil {
		return primitive.NewObjectID(), primitive.NewObjectID(), err
	}

	return userID, orgID, nil
}
