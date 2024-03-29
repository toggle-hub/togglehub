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
	"github.com/Roll-Play/togglelabs/pkg/api/handlers/fixtures"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	featureflagmodel "github.com/Roll-Play/togglelabs/pkg/models/feature_flag"
	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	timelinemodel "github.com/Roll-Play/togglelabs/pkg/models/timeline"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
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

	logger, _ := logger.NewZapLogger()
	h := handlers.NewFeatureFlagHandler(suite.db, logger)

	testGroup := suite.Server.Group("", middlewares.AuthMiddleware, middlewares.OrganizationMiddleware)
	testGroup.POST("/features", h.PostFeatureFlag)
	testGroup.PATCH(
		"/features/:featureFlagID",
		h.PatchFeatureFlag,
	)
	testGroup.GET("/features", h.ListFeatureFlags)
	testGroup.PATCH(
		"/features/:featureFlagID/revisions/:revisionID",
		h.ApproveRevision,
	)
	testGroup.DELETE("/features/:featureFlagID", h.DeleteFeatureFlag)
	testGroup.PATCH(
		"/features/:featureFlagID/rollback",
		h.RollbackFeatureFlagVersion,
	)
	testGroup.PATCH("/features/:featureFlagID/toggle", h.ToggleFeatureFlag)
	testGroup.PATCH("/features/:featureFlagID/tags", h.PatchFeatureFlagTags)
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

	rule := featureflagmodel.Rule{
		Predicate: "attr: rule",
		Value:     "false",
		Env:       "prd",
		IsEnabled: true,
	}
	featureFlagRequest := handlers.PostFeatureFlagRequest{
		Name:         "cool feature",
		Type:         featureflagmodel.Boolean,
		DefaultValue: "true",
		Rules: []featureflagmodel.Rule{
			rule,
		},
		Tags:        []string{"my_tag"},
		Environment: "prod",
	}
	requestBody, err := json.Marshal(featureFlagRequest)
	assert.NoError(t, err)

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"/features",
		bytes.NewBuffer(requestBody),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response featureflagmodel.FeatureFlagRecord

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, user.ID, response.UserID)
	assert.Equal(t, organization.ID, response.OrganizationID)
	assert.Equal(t, featureFlagRequest.Type, response.Type)
	assert.Equal(t, featureFlagRequest.Environment, response.Environments[0].Name)
	assert.NotEmpty(t, response.Tags)
	assert.Equal(t, []string{"my_tag"}, response.Tags)
	assert.Equal(t, featureFlagRequest.Project, response.Project)

	organizationModel := organizationmodel.New(suite.db)
	updatedOrganization, err := organizationModel.FindByID(context.Background(), organization.ID)
	assert.NoError(t, err)
	assert.NotEmpty(t, updatedOrganization.Tags)
	assert.Equal(t, []string{"my_tag"}, updatedOrganization.Tags)

	assert.NotEmpty(t, response.Revisions)
	responseRevision := response.Revisions[0]
	assert.Equal(t, user.ID, responseRevision.UserID)
	assert.Equal(t, featureFlagRequest.DefaultValue, responseRevision.DefaultValue)
	assert.Equal(t, featureflagmodel.Live, responseRevision.Status)

	assert.NotEmpty(t, responseRevision.Rules)
	responseRule := responseRevision.Rules[0]
	assert.Equal(t, rule.Predicate, responseRule.Predicate)
	assert.Equal(t, rule.Value, responseRule.Value)
	assert.Equal(t, rule.Env, responseRule.Env)
	assert.Equal(t, rule.IsEnabled, responseRule.IsEnabled)

	timelineModel := timelinemodel.New(suite.db)
	timelineRecord, err := timelineModel.FindByID(context.Background(), response.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(timelineRecord.Entries))
	assert.Equal(t, timelinemodel.Created, timelineRecord.Entries[0].Action)
	assert.Equal(t, user.ID, timelineRecord.Entries[0].UserID)
}

func (suite *FeatureFlagHandlerTestSuite) TestPostFeatureFlagUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.ReadOnly,
		),
	}, nil, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"/features",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)

	revision := fixtures.CreateRevision(user.ID, featureflagmodel.Live, nil)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		featureflagmodel.Boolean, []featureflagmodel.Revision{*revision}, nil, nil, nil, suite.db)

	newRule := featureflagmodel.Rule{
		Predicate: "attr: newRule",
		Value:     "true",
		Env:       "prd",
		IsEnabled: true,
	}
	revisionRule := handlers.PatchFeatureFlagRequest{
		DefaultValue: "true",
		Rules: []featureflagmodel.Rule{
			newRule,
		},
	}

	timelineModel := timelinemodel.New(suite.db)
	timelineRecord := &timelinemodel.TimelineRecord{
		FeatureFlagID: featureFlagRecord.ID,
		Entries:       []timelinemodel.TimelineEntry{},
	}
	_, err := timelineModel.InsertOne(context.Background(), timelineRecord)
	assert.NoError(t, err)

	requestBody, err := json.Marshal(revisionRule)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+featureFlagRecord.ID.Hex(),
		bytes.NewBuffer(requestBody),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	featureFlagModel := featureflagmodel.New(suite.db)
	var response featureflagmodel.Revision

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, user.ID, response.UserID)
	assert.Equal(t, revisionRule.DefaultValue, response.DefaultValue)
	assert.Equal(t, featureflagmodel.Draft, response.Status)

	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.NotEqual(t, featureFlagRecord.UpdatedAt, savedFeatureFlag.UpdatedAt)
	savedRevisions := savedFeatureFlag.Revisions
	assert.Equal(t, len(savedRevisions), 2)
	// Make sure original revision is the same
	originalRevision := savedRevisions[0]
	assert.Equal(t, user.ID, originalRevision.UserID)
	assert.Equal(t, revision.DefaultValue, originalRevision.DefaultValue)
	assert.Equal(t, featureflagmodel.Live, originalRevision.Status)
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
	assert.Equal(t, featureflagmodel.Draft, newSavedRevision.Status)
	assert.NotEmpty(t, newSavedRevision.Rules)
	newSavedRule := newSavedRevision.Rules[0]
	assert.Equal(t, newRule.Predicate, newSavedRule.Predicate)
	assert.Equal(t, newRule.Value, newSavedRule.Value)
	assert.Equal(t, newRule.Env, newSavedRule.Env)
	assert.Equal(t, newRule.IsEnabled, newSavedRule.IsEnabled)

	savedTimeline, err := timelineModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(savedTimeline.Entries))
	assert.Equal(t, timelinemodel.RevisionCreated, savedTimeline.Entries[0].Action)
	assert.Equal(t, user.ID, savedTimeline.Entries[0].UserID)
}

func (suite *FeatureFlagHandlerTestSuite) TestPatchFeatureFlagUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.ReadOnly,
		),
	}, nil, suite.db)
	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+primitive.NewObjectID().Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)
	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	featureFlag1 := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		featureflagmodel.Boolean, nil, nil, nil, nil, suite.db)
	featureFlag2 := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature 2", 1,
		featureflagmodel.Boolean, nil, nil, nil, nil, suite.db)

	request := httptest.NewRequest(
		http.MethodGet,
		"/features",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response handlers.ListFeatureFlagResponse

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, handlers.ListFeatureFlagResponse{
		Data: []featureflagmodel.FeatureFlagRecord{
			*featureFlag2,
			*featureFlag1,
		},
		Page:     1,
		PageSize: 10,
		Total:    2,
	}, response)
}

func (suite *FeatureFlagHandlerTestSuite) TestListFeatureFlagsPagination() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)
	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		featureflagmodel.Boolean, nil, nil, nil, nil, suite.db)
	featureFlag := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature 2", 1,
		featureflagmodel.Boolean, nil, nil, nil, nil, suite.db)

	request := httptest.NewRequest(
		http.MethodGet,
		"/features?page=1&page_size=1",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	var response handlers.ListFeatureFlagResponse

	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, handlers.ListFeatureFlagResponse{
		Data: []featureflagmodel.FeatureFlagRecord{
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)
	user, err := usermodel.NewUserRecord("evildoear97@gmail.com", "trying_to_steal_info", "Evil", "Doer")
	assert.NoError(t, err)

	userModel := usermodel.New(suite.db)
	userID, err := userModel.InsertOne(context.Background(), user)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(userID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodGet,
		"/features",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)

	willBeOriginalRevision := fixtures.CreateRevision(user.ID, featureflagmodel.Live, nil)
	willBeLiveRevision := fixtures.CreateRevision(user.ID, featureflagmodel.Draft, nil)
	willBeControlRevision := fixtures.CreateRevision(user.ID, featureflagmodel.Draft, nil)

	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		featureflagmodel.Boolean, []featureflagmodel.Revision{
			*willBeOriginalRevision,
			*willBeLiveRevision,
			*willBeControlRevision,
		}, nil, nil, nil, suite.db)

	timelineModel := timelinemodel.New(suite.db)
	timelineRecord := &timelinemodel.TimelineRecord{
		FeatureFlagID: featureFlagRecord.ID,
		Entries:       []timelinemodel.TimelineEntry{},
	}
	_, err := timelineModel.InsertOne(context.Background(), timelineRecord)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+featureFlagRecord.ID.Hex()+
			"/revisions/"+willBeLiveRevision.ID.Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	model := featureflagmodel.New(suite.db)
	savedFeatureFlag, err := model.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)

	savedRevisions := savedFeatureFlag.Revisions
	assert.Equal(t, len(savedRevisions), 3)
	assert.Equal(t, savedFeatureFlag.Version, 2)

	originalRevision := savedRevisions[0]
	assert.Equal(t, featureflagmodel.Archived, originalRevision.Status)
	updatedRevision := savedRevisions[1]
	assert.Equal(t, featureflagmodel.Live, updatedRevision.Status)
	controlRevision := savedRevisions[2]
	assert.Equal(t, featureflagmodel.Draft, controlRevision.Status)

	savedTimeline, err := timelineModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(savedTimeline.Entries))
	assert.Equal(t, timelinemodel.RevisionApproved, savedTimeline.Entries[0].Action)
	assert.Equal(t, user.ID, savedTimeline.Entries[0].UserID)
}

func (suite *FeatureFlagHandlerTestSuite) TestRevisionUpdateUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)

	unauthorizedUser := fixtures.CreateUser("", "", "", "", suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+primitive.NewObjectID().Hex()+
			"/revisions/"+primitive.NewObjectID().Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			unauthorizedUser,
			organizationmodel.ReadOnly,
		),
	}, nil, suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+primitive.NewObjectID().Hex()+
			"/revisions/"+primitive.NewObjectID().Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Collaborator,
		),
	}, nil, suite.db)

	revision := fixtures.CreateRevision(user.ID, featureflagmodel.Archived, nil)
	wrongRevision := fixtures.CreateRevision(user.ID, featureflagmodel.Live, &revision.ID)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 2,
		featureflagmodel.Boolean, []featureflagmodel.Revision{*revision, *wrongRevision}, nil, nil, nil, suite.db)

	timelineModel := timelinemodel.New(suite.db)
	timelineRecord := &timelinemodel.TimelineRecord{
		FeatureFlagID: featureFlagRecord.ID,
		Entries:       []timelinemodel.TimelineEntry{},
	}
	_, err := timelineModel.InsertOne(context.Background(), timelineRecord)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+featureFlagRecord.ID.Hex()+
			"/rollback",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	featureFlagModel := featureflagmodel.New(suite.db)
	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)

	savedRevisions := savedFeatureFlag.Revisions
	assert.Equal(t, 2, len(savedRevisions))
	assert.Equal(t, 1, savedFeatureFlag.Version)

	liveRevision := savedRevisions[0]
	assert.Equal(t, featureflagmodel.Live, liveRevision.Status)
	rolledBackRevision := savedRevisions[1]
	assert.Equal(t, featureflagmodel.Draft, rolledBackRevision.Status)

	savedTimeline, err := timelineModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(savedTimeline.Entries))
	assert.Equal(t, timelinemodel.FeatureFlagRollback, savedTimeline.Entries[0].Action)
	assert.Equal(t, user.ID, savedTimeline.Entries[0].UserID)
}

func (suite *FeatureFlagHandlerTestSuite) TestRollbackUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)

	unauthorizedUser := fixtures.CreateUser("", "", "", "", suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+primitive.NewObjectID().Hex()+
			"/rollback",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.ReadOnly,
		),
	}, nil, suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+primitive.NewObjectID().Hex()+
			"/rollback",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)

	revision := fixtures.CreateRevision(user.ID, featureflagmodel.Archived, nil)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 2,
		featureflagmodel.Boolean, []featureflagmodel.Revision{*revision}, nil, nil, nil, suite.db)

	timelineModel := timelinemodel.New(suite.db)
	timelineRecord := &timelinemodel.TimelineRecord{
		FeatureFlagID: featureFlagRecord.ID,
		Entries:       []timelinemodel.TimelineEntry{},
	}
	_, err := timelineModel.InsertOne(context.Background(), timelineRecord)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/features/"+featureFlagRecord.ID.Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	model := featureflagmodel.New(suite.db)

	deletedRecord, err := model.FindOne(context.Background(), bson.D{
		{Key: "_id", Value: featureFlagRecord.ID},
		{Key: "deleted_at", Value: bson.M{
			"$exists": true},
		}})

	assert.NoError(t, err)

	assert.Equal(t, featureFlagRecord.ID, deletedRecord.ID)
	assert.Equal(t, http.StatusNoContent, recorder.Code)

	savedTimeline, err := timelineModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(savedTimeline.Entries))
	assert.Equal(t, timelinemodel.FeatureFlagDeleted, savedTimeline.Entries[0].Action)
	assert.Equal(t, user.ID, savedTimeline.Entries[0].UserID)
}

func (suite *FeatureFlagHandlerTestSuite) TestFeatureFlagDeletionForbidden() {
	t := suite.T()
	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.ReadOnly,
		),
	}, nil, suite.db)
	revision := fixtures.CreateRevision(user.ID, featureflagmodel.Archived, nil)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 2,
		featureflagmodel.Boolean, []featureflagmodel.Revision{*revision}, nil, nil, nil, suite.db)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/features/"+featureFlagRecord.ID.Hex(),
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Collaborator,
		),
	}, nil, suite.db)

	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 2,
		featureflagmodel.Boolean, nil, []featureflagmodel.FeatureFlagEnvironment{
			{
				Name:      "prod",
				IsEnabled: true,
			},
			{
				Name:      "dev",
				IsEnabled: true,
			},
		}, nil, nil, suite.db)

	timelineModel := timelinemodel.New(suite.db)
	timelineRecord := &timelinemodel.TimelineRecord{
		FeatureFlagID: featureFlagRecord.ID,
		Entries:       []timelinemodel.TimelineEntry{},
	}
	_, err := timelineModel.InsertOne(context.Background(), timelineRecord)
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+featureFlagRecord.ID.Hex()+
			"/toggle?env=prod",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	featureFlagModel := featureflagmodel.New(suite.db)
	savedFeatureFlag, err := featureFlagModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.Equal(t, featureFlagRecord.Environments[0].Name, savedFeatureFlag.Environments[0].Name)
	assert.Equal(t, false, savedFeatureFlag.Environments[0].IsEnabled)
	assert.Equal(t, featureFlagRecord.Environments[1].Name, savedFeatureFlag.Environments[1].Name)
	assert.Equal(t, featureFlagRecord.Environments[1].IsEnabled, savedFeatureFlag.Environments[1].IsEnabled)

	savedTimeline, err := timelineModel.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(savedTimeline.Entries))
	assert.Equal(t, fmt.Sprintf(timelinemodel.FeatureFlagToggle, "prod"), savedTimeline.Entries[0].Action)
	assert.Equal(t, user.ID, savedTimeline.Entries[0].UserID)
}

func (suite *FeatureFlagHandlerTestSuite) TestEnvironmentToggleUnauthorized() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)

	unauthorizedUser := fixtures.CreateUser("", "", "", "", suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+primitive.NewObjectID().Hex()+
			"/toggle?env=prod",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.ReadOnly,
		),
	}, nil, suite.db)

	token, err := apiutils.CreateJWT(unauthorizedUser.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+primitive.NewObjectID().Hex()+
			"/toggle?env=prod",
		nil,
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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

func (suite *FeatureFlagHandlerTestSuite) TestPatchTagSuccess() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.Admin,
		),
	}, nil, suite.db)

	revision := fixtures.CreateRevision(user.ID, featureflagmodel.Live, nil)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		featureflagmodel.Boolean, []featureflagmodel.Revision{*revision}, nil, nil, nil, suite.db)

	timelineModel := timelinemodel.New(suite.db)
	timelineRecord := &timelinemodel.TimelineRecord{
		FeatureFlagID: featureFlagRecord.ID,
		Entries:       []timelinemodel.TimelineEntry{},
	}
	_, err := timelineModel.InsertOne(context.Background(), timelineRecord)
	assert.NoError(t, err)
	expected := []string{"fizi", "valores"}
	requestBody, err := json.Marshal(handlers.PatchFeatureFlagTagsRequest{
		Tags: expected,
	})
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+featureFlagRecord.ID.Hex()+"/tags",
		bytes.NewBuffer(requestBody),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
	recorder := httptest.NewRecorder()

	suite.Server.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)

	model := featureflagmodel.New(suite.db)
	updatedFlag, err := model.FindByID(context.Background(), featureFlagRecord.ID)
	assert.NoError(t, err)
	assert.Equal(t, expected, updatedFlag.Tags)
	assert.NotEqual(t, featureFlagRecord.UpdatedAt, updatedFlag.UpdatedAt)

	organizationModel := organizationmodel.New(suite.db)
	updatedOrganization, err := organizationModel.FindByID(context.Background(), organization.ID)
	assert.NoError(t, err)
	assert.NotEmpty(t, updatedOrganization.Tags)
	assert.Equal(t, expected, updatedOrganization.Tags)
}

func (suite *FeatureFlagHandlerTestSuite) TestPatchTagForbidden() {
	t := suite.T()

	user := fixtures.CreateUser("", "", "", "", suite.db)
	organization := fixtures.CreateOrganization("the company", []common.Tuple[*usermodel.UserRecord, string]{
		common.NewTuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum](
			user,
			organizationmodel.ReadOnly,
		),
	}, nil, suite.db)

	revision := fixtures.CreateRevision(user.ID, featureflagmodel.Live, nil)
	featureFlagRecord := fixtures.CreateFeatureFlag(user.ID, organization.ID, "cool feature", 1,
		featureflagmodel.Boolean, []featureflagmodel.Revision{*revision}, nil, nil, nil, suite.db)

	timelineModel := timelinemodel.New(suite.db)
	timelineRecord := &timelinemodel.TimelineRecord{
		FeatureFlagID: featureFlagRecord.ID,
		Entries:       []timelinemodel.TimelineEntry{},
	}
	_, err := timelineModel.InsertOne(context.Background(), timelineRecord)
	assert.NoError(t, err)

	requestBody, err := json.Marshal(handlers.PatchFeatureFlagTagsRequest{
		Tags: []string{"fizi", "valores"},
	})
	assert.NoError(t, err)

	token, err := apiutils.CreateJWT(user.ID, time.Second*120)
	assert.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/features/"+featureFlagRecord.ID.Hex()+"/tags",
		bytes.NewBuffer(requestBody),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set(middlewares.XOrganizationHeader, organization.ID.Hex())
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
