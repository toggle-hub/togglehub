package middlewares

import (
	"net/http"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

const XOrganizationHeader = "X-organization"

func OrganizationMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		logger, _ := logger.GetInstance()
		organizationHeader := c.Request().Header.Get(XOrganizationHeader)

		if organizationHeader == "" {
			logger.Debug("Missing organization header")
			return apierrors.CustomError(
				c,
				http.StatusBadRequest,
				apierrors.BadRequestError,
			)
		}

		organizationID, err := primitive.ObjectIDFromHex(organizationHeader)
		if err != nil {
			logger.Debug("Client error",
				zap.Error(err))
			return apierrors.CustomError(
				c,
				http.StatusBadRequest,
				apierrors.BadRequestError,
			)
		}

		c.Set("organization", organizationID.Hex())
		return next(c)
	}
}
