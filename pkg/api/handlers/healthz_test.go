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
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()

	c := suite.Server.NewContext(request, recorder)
	var response handlers.HealthResponse

	assert.NoError(suite.T(), handlers.HealthHandler(c))
	assert.NoError(suite.T(), json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(suite.T(), http.StatusOK, recorder.Code)
	assert.Equal(suite.T(), handlers.HealthResponse{
		Alive: true,
	}, response)
}

func TestHandlers(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
}
