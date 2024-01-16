package healthzhandler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type HealthResponse struct {
	Alive bool `json:"alive"`
}

func HealthHandler(c echo.Context) error {
	r := HealthResponse{Alive: true}

	return c.JSON(http.StatusOK, r)
}
