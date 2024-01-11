package middlewares

import (
	"errors"
	"log"
	"net/http"
	"strings"

	api_errors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	api_utils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

var ErrMissingAuthHeader = errors.New("missing authorization header")
var ErrInvalidSignMethod = errors.New("invalid signing method")
var ErrInvalidToken = errors.New("invalid token")

func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		logger, _ := logger.GetInstance()
		authHeader := c.Request().Header.Get(echo.HeaderAuthorization)

		if authHeader == "" {
			logger.Debug("Client error",
				zap.String("cause", "missing Authorization header"))
			return c.JSON(http.StatusUnauthorized, api_errors.Error{
				Error:   "missing Authorization header",
				Message: http.StatusText(http.StatusUnauthorized),
			})
		}

		tokenString := strings.TrimSpace(strings.Replace(authHeader, "Bearer ", "", 1))
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			secretKey := []byte("your-secret-key")

			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				logger.Debug("Client error",
					zap.String("cause", ErrInvalidSignMethod.Error()))
				return nil, ErrInvalidSignMethod
			}
			return secretKey, nil
		})

		if err != nil {
			logger.Debug("Client error",
				zap.String("cause", err.Error()))
			return c.JSON(http.StatusUnauthorized, api_errors.Error{
				Error:   ErrInvalidToken.Error(),
				Message: http.StatusText(http.StatusUnauthorized),
			})
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			sub, _ := claims["sub"].(string)
			userID, err := primitive.ObjectIDFromHex(sub)
			if err != nil {
				log.Println(api_utils.HandlerErrorLogMessage(err, c))
				return c.JSON(http.StatusUnauthorized, api_errors.Error{
					Error:   "invalid token sub",
					Message: http.StatusText(http.StatusUnauthorized),
				})
			}

			c.Set("user", userID.Hex())
			return next(c)
		}

		log.Println(api_utils.HandlerErrorLogMessage(ErrInvalidToken, c))
		return c.JSON(http.StatusUnauthorized, api_errors.Error{
			Error:   ErrInvalidToken.Error(),
			Message: http.StatusText(http.StatusUnauthorized),
		})
	}
}
