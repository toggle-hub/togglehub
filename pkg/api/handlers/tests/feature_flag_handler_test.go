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

	logger, _ := common.NewZapLogger()
	h := handlers.NewFeatureFlagHandler(suite.db, logger)

	testGroup := suite.Server.Group("", middlewares.AuthMiddleware)
	testGroup.POST("/organizations/:organizationID/feature-flags", h.PostFeatureFlag)
	testGroup.PATCH(
		"/organizations/:organizationID/feature-flags/:featureFlagID",
		h.PatchFeatureFlag,
	)
	testGroup.GET("/organizations/:organizationID/feature-flags", h.ListFeatureFlags)
	testGroup.PATCH(
		"/organizations/:organizationID/feature-flags/:featureFlagID/revisions/:revisionID",
		h.ApproveRevision,
	)
	testGroup.DELETE("/organizations/:organizationID/feature-flags/:featureFlagID", h.DeleteFeatureFlag)
	testGroup.PATCH(
		"/organizations/:organizationID/feature-flags/:featureFlagID/rollback",
		h.RollbackFeatureFlagVersion,
	)
	testGroup.PATCH("/organizations/:organizationID/feature-flags/:featureFlagID/toggle", h.ToggleFeatureFlag)
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
		Environment: "prod",
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
		"/organizations/"+organization.ID.Hex()+"/feature-flags",
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
	assert.Equal(t, featureFlagRequest.Environment, response.Environments[0].Name)

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
		"/organizations/"+organization.ID.Hex()+"/feature-flags",
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

func (suite *FeatureFlagHandlerTestSuite) TestPatchFeatureFlagSuccess() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)

	revision := fixtures.CreateRevision(user.ID, models.Live, primitive.NilObjectID)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		models.Boolean, []models.Revision{*revision}, nil, suite.db)

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
		"/organizations/"+organization.ID.Hex()+"/feature-flags/"+featureFlagRecord.ID.Hex(),
		bytes.NewBuffer(requestBody),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	featureFlagModel := models.NewFeatureFlagModel(suite.db)
	var response models.Revision

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, user.ID, response.UserID)
	assert.Equal(t, revisionRule.DefaultValue, response.DefaultValue)
	assert.Equal(t, models.Draft, response.Status)

	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	savedRevisions := savedFeatureFlag.Revisions
	assert.Equal(t, len(savedRevisions), 2)
	// Make sure original revision is the same
	originalRevision := savedRevisions[0]
	assert.Equal(t, user.ID, originalRevision.UserID)
	assert.Equal(t, revision.DefaultValue, originalRevision.DefaultValue)
	assert.Equal(t, models.Live, originalRevision.Status)
	assert.NotEmpty(t, originalRevision.Rules)
	originalRule := revision.Rules[0]
	originalSavedRule := originalRevision.Rules[0]
	assert.Equal(t, originalRule.Predicate, originalSavedRule.Predicate)
	assert.Equal(t, originalRule.Value, originalSavedRule.Value)
	assert.Equal(t, originalRule.Env, originalSavedRule.Env)
	assert.Equal(t, originalRule.IsEnabled, originalSavedRule.IsEnabled)
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
		"/organizations/"+organization.ID.Hex()+"/feature-flags/"+primitive.NewObjectID().Hex(),
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

	featureFlag1 := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		models.Boolean, nil, nil, suite.db)
	featureFlag2 := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature 2", 1,
		models.Boolean, nil, nil, suite.db)

	request := httptest.NewRequest(
		http.MethodGet,
		"/organizations/"+organization.ID.Hex()+"/feature-flags",
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
			*featureFlag1,
			*featureFlag2,
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

	featureFlag := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		models.Boolean, nil, nil, suite.db)
	fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature 2", 1,
		models.Boolean, nil, nil, suite.db)

	request := httptest.NewRequest(
		http.MethodGet,
		"/organizations/"+organization.ID.Hex()+"/feature-flags?page=1&page_size=1",
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
			*featureFlag,
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
		"/organizations/"+organization.ID.Hex()+"/feature-flags",
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

	willBeOriginalRevision := fixtures.CreateRevision(user.ID, models.Live, primitive.NilObjectID)
	willBeLiveRevision := fixtures.CreateRevision(user.ID, models.Draft, primitive.NilObjectID)
	willBeControlRevision := fixtures.CreateRevision(user.ID, models.Draft, primitive.NilObjectID)

	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		models.Boolean, []models.Revision{
			*willBeOriginalRevision,
			*willBeLiveRevision,
			*willBeControlRevision,
		}, nil, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+featureFlagRecord.ID.Hex()+
			"/revisions/"+willBeLiveRevision.ID.Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	model := models.NewFeatureFlagModel(suite.db)
	savedFeatureFlag, err := model.FindByID(context.Background(), featureFlagRecord.ID)
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

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+primitive.NewObjectID().Hex()+
			"/revisions/"+primitive.NewObjectID().Hex(),
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
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+primitive.NewObjectID().Hex()+
			"/revisions/"+primitive.NewObjectID().Hex(),
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

	revision := fixtures.CreateRevision(user.ID, models.Archived, primitive.NilObjectID)
	wrongRevision := fixtures.CreateRevision(user.ID, models.Live, revision.ID)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 2,
		models.Boolean, []models.Revision{*revision, *wrongRevision}, nil, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+featureFlagRecord.ID.Hex()+
			"/rollback",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	featureFlagModel := models.NewFeatureFlagModel(suite.db)
	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagRecord.ID)
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
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+primitive.NewObjectID().Hex()+
			"/rollback",
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
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+primitive.NewObjectID().Hex()+
			"/rollback",
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

func (suite *FeatureFlagHandlerTestSuite) TestFeatureFlagDeletionSuccess() {
	t := suite.T()
	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Admin,
		),
	}, suite.db)

	revision := fixtures.CreateRevision(user.ID, models.Archived, primitive.NilObjectID)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 2,
		models.Boolean, []models.Revision{*revision}, nil, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+featureFlagRecord.ID.Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	model := models.NewFeatureFlagModel(suite.db)

	deletedRecord, err := model.FindOne(context.Background(), bson.D{
		{Key: "_id", Value: featureFlagRecord.ID},
		{Key: "deleted_at", Value: bson.M{
			"$exists": true},
		}})

	assert.NoError(t, err)

	assert.Equal(t, featureFlagRecord.ID, deletedRecord.ID)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func (suite *FeatureFlagHandlerTestSuite) TestFeatureFlagDeletionForbidden() {
	t := suite.T()
	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.ReadOnly,
		),
	}, suite.db)
	revision := fixtures.CreateRevision(user.ID, models.Archived, primitive.NilObjectID)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 2,
		models.Boolean, []models.Revision{*revision}, nil, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+featureFlagRecord.ID.Hex(),
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

func (suite *FeatureFlagHandlerTestSuite) TestEnvironmentToggleSuccess() {
	t := suite.T()
	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*models.UserRecord, string]{
		common.NewTuple[*models.UserRecord, models.PermissionLevelEnum](
			user,
			models.Collaborator,
		),
	}, suite.db)

	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 2,
		models.Boolean, nil, []models.FeatureFlagEnvironment{
			{
				Name:      "prod",
				IsEnabled: true,
			},
			{
				Name:      "dev",
				IsEnabled: true,
			},
		}, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+featureFlagRecord.ID.Hex()+
			"/toggle?env=prod",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	featureFlagModel := models.NewFeatureFlagModel(suite.db)
	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.Equal(t, featureFlagRecord.Environments[0].Name, savedFeatureFlag.Environments[0].Name)
	assert.Equal(t, false, savedFeatureFlag.Environments[0].IsEnabled)
	assert.Equal(t, featureFlagRecord.Environments[1].Name, savedFeatureFlag.Environments[1].Name)
	assert.Equal(t, featureFlagRecord.Environments[1].IsEnabled, savedFeatureFlag.Environments[1].IsEnabled)
}

func (suite *FeatureFlagHandlerTestSuite) TestEnvironmentToggleUnauthorized() {
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
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+primitive.NewObjectID().Hex()+
			"/toggle?env=prod",
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

func (suite *FeatureFlagHandlerTestSuite) TestEnvironmentToggleMissingPermission() {
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
		"/organizations/"+organization.ID.Hex()+
			"/feature-flags/"+primitive.NewObjectID().Hex()+
			"/toggle?env=prod",
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

func TestFeatureFlagHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(FeatureFlagHandlerTestSuite))
}
