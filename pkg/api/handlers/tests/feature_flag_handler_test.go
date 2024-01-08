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

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/handlers/tests/fixtures"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
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
	suite.Server.POST("organization/:organizationID/feature-flag", middlewares.AuthMiddleware(h.PostFeatureFlag))
	suite.Server.PATCH(
		"organization/:organizationID/feature-flag/:featureFlagID",
		middlewares.AuthMiddleware(h.PatchFeatureFlag),
	)
	suite.Server.GET("/organization/:organizationID/feature-flag", middlewares.AuthMiddleware(h.ListFeatureFlags))
	suite.Server.PATCH(
		"/organization/:organizationID/feature-flag/:featureFlagID/revision/:revisionID",
		middlewares.AuthMiddleware(h.ApproveRevision),
	)
	suite.Server.PATCH(
		"organization/:organizationID/feature-flag/:featureFlagID/rollback",
		middlewares.AuthMiddleware(h.RollbackFeatureFlagVersion),
	)
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
	featureFlagRequest := handlers.PostFeatureFlagRequest{
		Name:         "cool feature",
		Type:         models.Boolean,
		DefaultValue: "true",
		Rules: []models.Rule{
			rule,
		},
	}
	requestBody, err := json.Marshal(featureFlagRequest)
	assert.NoError(t, err)

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"/organization/"+organization.ID.Hex()+"/feature-flag",
		bytes.NewBuffer(requestBody),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response models.FeatureFlagRecord

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, user.ID, response.UserID)
	assert.Equal(t, organization.ID, response.OrganizationID)
	assert.Equal(t, featureFlagRequest.Type, response.Type)

	assert.NotEmpty(t, response.Revisions)
	responseRevision := response.Revisions[0]
	assert.Equal(t, user.ID, responseRevision.UserID)
	assert.Equal(t, featureFlagRequest.DefaultValue, responseRevision.DefaultValue)
	assert.Equal(t, models.Live, responseRevision.Status)

	assert.NotEmpty(t, responseRevision.Rules)
	responseRule := responseRevision.Rules[0]
	assert.Equal(t, rule.Predicate, responseRule.Predicate)
	assert.Equal(t, rule.Value, responseRule.Value)
	assert.Equal(t, rule.Env, responseRule.Env)
	assert.Equal(t, rule.IsEnabled, responseRule.IsEnabled)
}

func (suite *FeatureFlagHandlerTestSuite) TestPostFeatureFlagUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.ReadOnly,
		),
	}, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"/organization/"+organization.ID.Hex()+"/feature-flag",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response apierrors.Error

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestPatchFeatureFlagSuccess() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)

	rule := models.Rule{
		Predicate: "attr: rule",
		Value:     "false",
		Env:       "dev",
		IsEnabled: true,
	}
	featureFlagRequest := &handlers.PostFeatureFlagRequest{
		Name:         "cool feature",
		Type:         models.Boolean,
		DefaultValue: "false",
		Rules: []models.Rule{
			rule,
		},
	}
	featureFlagRecord := models.NewFeatureFlagRecord(
		featureFlagRequest.Name,
		featureFlagRequest.DefaultValue,
		featureFlagRequest.Type,
		featureFlagRequest.Rules,
		organization.ID,
		user.ID,
	)
	featureFlagModel := models.NewFeatureFlagModel(suite.db)
	featureFlagID, err := featureFlagModel.InsertOne(context.Background(), featureFlagRecord)
	assert.NoError(t, err)

	newRule := models.Rule{
		Predicate: "attr: newRule",
		Value:     "true",
		Env:       "prd",
		IsEnabled: true,
	}
	revisionRule := handlers.PatchFeatureFlagRequest{
		DefaultValue: "true",
		Rules: []models.Rule{
			newRule,
		},
	}
	requestBody, err := json.Marshal(revisionRule)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organization/"+organization.ID.Hex()+"/feature-flag/"+featureFlagID.Hex(),
		bytes.NewBuffer(requestBody),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response models.Revision

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, user.ID, response.UserID)
	assert.Equal(t, revisionRule.DefaultValue, response.DefaultValue)
	assert.Equal(t, models.Draft, response.Status)
	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagID)
	assert.NoError(t, err)
	savedRevisions := savedFeatureFlag.Revisions
	assert.Equal(t, len(savedRevisions), 2)
	// Make sure original revision is the same
	originalRevision := savedRevisions[0]
	assert.Equal(t, user.ID, originalRevision.UserID)
	assert.Equal(t, featureFlagRequest.DefaultValue, originalRevision.DefaultValue)
	assert.Equal(t, models.Live, originalRevision.Status)
	assert.NotEmpty(t, originalRevision.Rules)
	originalRule := originalRevision.Rules[0]
	assert.Equal(t, rule.Predicate, originalRule.Predicate)
	assert.Equal(t, rule.Value, originalRule.Value)
	assert.Equal(t, rule.Env, originalRule.Env)
	assert.Equal(t, rule.IsEnabled, originalRule.IsEnabled)
	// Check the new revision
	newSavedRevision := savedRevisions[1]
	assert.Equal(t, user.ID, newSavedRevision.UserID)
	assert.Equal(t, revisionRule.DefaultValue, newSavedRevision.DefaultValue)
	assert.Equal(t, models.Draft, newSavedRevision.Status)
	assert.NotEmpty(t, newSavedRevision.Rules)
	newSavedRule := newSavedRevision.Rules[0]
	assert.Equal(t, newRule.Predicate, newSavedRule.Predicate)
	assert.Equal(t, newRule.Value, newSavedRule.Value)
	assert.Equal(t, newRule.Env, newSavedRule.Env)
	assert.Equal(t, newRule.IsEnabled, newSavedRule.IsEnabled)
}

func (suite *FeatureFlagHandlerTestSuite) TestPatchFeatureFlagUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.ReadOnly,
		),
	}, suite.db)
	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organization/"+organization.ID.Hex()+"/feature-flag/"+primitive.NewObjectID().Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response apierrors.Error

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestListFeatureFlagsAuthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)
	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	featureFlagModel := models.NewFeatureFlagModel(suite.db)

	rule := models.Rule{
		Predicate: "attr: rule",
		Value:     "false",
		Env:       "dev",
		IsEnabled: true,
	}

	featureFlags := []*models.FeatureFlagRecord{
		models.NewFeatureFlagRecord(
			"cool feature",
			"false",
			models.Boolean,
			[]models.Rule{rule},
			organization.ID,
			user.ID,
		),
		models.NewFeatureFlagRecord(
			"cool feature",
			"false",
			models.Boolean,
			[]models.Rule{rule},
			organization.ID,
			user.ID,
		),
	}

	for _, featureFlag := range featureFlags {
		_, err := featureFlagModel.InsertOne(context.Background(), featureFlag)
		assert.NoError(t, err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/organization/"+organization.ID.Hex()+"/feature-flag",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response handlers.ListFeatureFlagResponse

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, handlers.ListFeatureFlagResponse{
		Data: []models.FeatureFlagRecord{
			*featureFlags[0],
			*featureFlags[1],
		},
		Page:     1,
		PageSize: 10,
		Total:    2,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestListFeatureFlagsPagination() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)
	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	featureFlagModel := models.NewFeatureFlagModel(suite.db)

	rule := models.Rule{
		Predicate: "attr: rule",
		Value:     "false",
		Env:       "dev",
		IsEnabled: true,
	}

	featureFlags := []*models.FeatureFlagRecord{
		models.NewFeatureFlagRecord(
			"cool feature",
			"false",
			models.Boolean,
			[]models.Rule{rule},
			organization.ID,
			user.ID,
		),
		models.NewFeatureFlagRecord(
			"cool feature 2",
			"false",
			models.Boolean,
			[]models.Rule{rule},
			organization.ID,
			user.ID,
		),
	}

	for _, featureFlag := range featureFlags {
		_, err := featureFlagModel.InsertOne(context.Background(), featureFlag)
		assert.NoError(t, err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/organization/"+organization.ID.Hex()+"/feature-flag?page=1&page_size=1",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response handlers.ListFeatureFlagResponse

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, handlers.ListFeatureFlagResponse{
		Data: []models.FeatureFlagRecord{
			*featureFlags[0],
		},
		Page:     1,
		PageSize: 1,
		Total:    1,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestListFeatureFlagsUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)
	user, err := models.NewUserRecord("evildoear97@gmail.com", "trying_to_steal_info", "Evil", "Doer")
	assert.NoError(t, err)

	userModel := models.NewUserModel(suite.db)
	userID, err := userModel.InsertOne(context.Background(), user)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(userID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodGet,
		"/organization/"+organization.ID.Hex()+"/feature-flag",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response apierrors.Error

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusForbidden),
		Message: apierrors.ForbiddenError,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestRevisionStatusUpdateSuccess() {
	t := suite.T()
	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)

	rule := models.Rule{
		Predicate: "attr: rule",
		Value:     "false",
		Env:       "dev",
		IsEnabled: true,
	}
	featureFlagRequest := &handlers.PostFeatureFlagRequest{
		Name:         "cool feature",
		Type:         models.Boolean,
		DefaultValue: "false",
		Rules: []models.Rule{
			rule,
		},
	}
	featureFlagRecord := models.NewFeatureFlagRecord(
		featureFlagRequest.Name,
		featureFlagRequest.DefaultValue,
		featureFlagRequest.Type,
		featureFlagRequest.Rules,
		organization.ID,
		user.ID,
	)
	featureFlagRecord.Revisions[0].Status = models.Live
	featureFlagModel := models.NewFeatureFlagModel(suite.db)
	featureFlagID, err := featureFlagModel.InsertOne(context.Background(), featureFlagRecord)
	assert.NoError(t, err)

	willBeLiveRevision := models.NewRevisionRecord("value", []models.Rule{rule}, user.ID)
	willBeControlRevision := models.NewRevisionRecord("value", []models.Rule{rule}, user.ID)

	_, err = featureFlagModel.UpdateOne(
		context.Background(),
		bson.D{{Key: "_id", Value: featureFlagID}},
		bson.D{{Key: "$push", Value: bson.M{"revisions": willBeLiveRevision}}},
	)
	assert.NoError(t, err)

	_, err = featureFlagModel.UpdateOne(
		context.Background(),
		bson.D{{Key: "_id", Value: featureFlagID}},
		bson.D{{Key: "$push", Value: bson.M{"revisions": willBeControlRevision}}},
	)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodPatch,
		"/organization/"+organization.ID.Hex()+
			"/feature-flag/"+featureFlagID.Hex()+
			"/revision/"+willBeLiveRevision.ID.Hex(),
		nil,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagID)
	assert.NoError(t, err)

	savedRevisions := savedFeatureFlag.Revisions
	assert.Equal(t, len(savedRevisions), 3)
	assert.Equal(t, savedFeatureFlag.Version, 2)

	originalRevision := savedRevisions[0]
	assert.Equal(t, models.Archived, originalRevision.Status)
	updatedRevision := savedRevisions[1]
	assert.Equal(t, models.Live, updatedRevision.Status)
	controlRevision := savedRevisions[2]
	assert.Equal(t, models.Draft, controlRevision.Status)
}

func (suite *FeatureFlagHandlerTestSuite) TestRevisionUpdateUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)

	unauthorizedUser := fixtures.CreateUser("", "", "", "", suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodPatch,
		"/organization/"+organization.ID.Hex()+
			"/feature-flag/"+primitive.NewObjectID().Hex()+
			"/revision/"+primitive.NewObjectID().Hex(),
		nil,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	rec := httptest.NewRecorder()

	suite.Server.ServeHTTP(rec, req)

	var response apierrors.Error

	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestRevisionUpdateUnauthorizedMissingPermission() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	unauthorizedUser := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			unauthorizedUser,
			models.ReadOnly,
		),
	}, suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organization/"+organization.ID.Hex()+
			"/feature-flag/"+primitive.NewObjectID().Hex()+
			"/revision/"+primitive.NewObjectID().Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response apierrors.Error

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestRollbackSuccess() {
	t := suite.T()
	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Collaborator,
		),
	}, suite.db)

	rule := models.Rule{
		Predicate: "attr: rule",
		Value:     "false",
		Env:       "dev",
		IsEnabled: true,
	}
	featureFlagRequest := &handlers.PostFeatureFlagRequest{
		Name:         "cool feature",
		Type:         models.Boolean,
		DefaultValue: "false",
		Rules: []models.Rule{
			rule,
		},
	}
	featureFlagRecord := models.NewFeatureFlagRecord(
		featureFlagRequest.Name,
		featureFlagRequest.DefaultValue,
		featureFlagRequest.Type,
		featureFlagRequest.Rules,
		organization.ID,
		user.ID,
	)
	featureFlagRecord.Revisions[0].Status = models.Archived
	featureFlagRecord.Version = 2
	featureFlagModel := models.NewFeatureFlagModel(suite.db)
	featureFlagID, err := featureFlagModel.InsertOne(context.Background(), featureFlagRecord)
	assert.NoError(t, err)

	wrongRevision := models.NewRevisionRecord("value", []models.Rule{rule}, user.ID)
	wrongRevision.LastRevisionID = featureFlagRecord.Revisions[0].ID
	wrongRevision.Status = models.Live

	_, err = featureFlagModel.UpdateOne(
		context.Background(),
		bson.D{{Key: "_id", Value: featureFlagID}},
		bson.D{{Key: "$push", Value: bson.M{"revisions": wrongRevision}}},
	)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organization/"+organization.ID.Hex()+
			"/feature-flag/"+featureFlagID.Hex()+
			"/rollback",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagID)
	assert.NoError(t, err)

	savedRevisions := savedFeatureFlag.Revisions
	assert.Equal(t, 2, len(savedRevisions))
	assert.Equal(t, 1, savedFeatureFlag.Version)

	liveRevision := savedRevisions[0]
	assert.Equal(t, models.Live, liveRevision.Status)
	rolledBackRevision := savedRevisions[1]
	assert.Equal(t, models.Draft, rolledBackRevision.Status)
}

func (suite *FeatureFlagHandlerTestSuite) TestRollbackUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)

	unauthorizedUser := fixtures.CreateUser("", "", "", "", suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organization/"+organization.ID.Hex()+
			"/feature-flag/"+primitive.NewObjectID().Hex()+
			"/rollback",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response apierrors.Error

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestRollbackUnauthorizedMissingPermission() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	unauthorizedUser := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.ReadOnly,
		),
	}, suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organization/"+organization.ID.Hex()+
			"/feature-flag/"+primitive.NewObjectID().Hex()+
			"/rollback",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response apierrors.Error

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, apierrors.Error{
		Error:   http.StatusText(http.StatusUnauthorized),
		Message: apierrors.UnauthorizedError,
	}, response)
}

func TestFeatureFlagHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(FeatureFlagHandlerTestSuite))
}
