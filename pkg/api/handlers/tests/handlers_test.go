package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	testutils "github.com/Roll-Play/togglelabs/pkg/utils/test_utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type HandlersSuite struct {
	testutils.DefaultTestSuite
}

func (suite *HandlersSuite) SetupTest() {
	suite.Server = echo.New()
}

func (suite *HandlersSuite) TestHealthHanlderHealthy() {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := suite.Server.NewContext(req, rec)
	var jsonRes handlers.HealthResponse

	assert.NoError(suite.T(), handlers.HealthHandler(c))
	assert.NoError(suite.T(), json.Unmarshal(rec.Body.Bytes(), &jsonRes))
	assert.Equal(suite.T(), http.StatusOK, rec.Code)
	assert.Equal(suite.T(), handlers.HealthResponse{
		Alive: true,
	}, jsonRes)
}

func TestHandlers(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
}
