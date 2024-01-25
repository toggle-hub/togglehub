package middlewares

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

var ErrMissingAuthCookie = errors.New("missing authorization cookie")
var ErrInvalidSignMethod = errors.New("invalid signing method")
var ErrInvalidToken = errors.New("invalid token")

func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		logger, _ := logger.GetInstance()
		authCookie, err := c.Cookie("Authorization")
		if err != nil {
			logger.Debug("Client error",
				zap.Error(ErrMissingAuthCookie))
			return c.JSON(http.StatusUnauthorized, apierrors.Error{
				Error:   "missing Authorization cookie",
				Message: http.StatusText(http.StatusUnauthorized),
			})
		}

		if authCookie.Name == "" {
			logger.Debug("Client error",
				zap.Error(ErrMissingAuthCookie))
			return c.JSON(http.StatusUnauthorized, apierrors.Error{
				Error:   "missing Authorization cookie",
				Message: http.StatusText(http.StatusUnauthorized),
			})
		}

		tokenString := strings.TrimSpace(strings.Replace(authCookie.Value, "Bearer ", "", 1))
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			secretKey := os.Getenv("JWT_KEY")
			if secretKey == "" {
				secretKey = "your-secret-key"
			}

			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				logger.Debug("Client error",
					zap.Error(ErrInvalidSignMethod))
				return nil, ErrInvalidSignMethod
			}
			return []byte(secretKey), nil
		})

		if err != nil {
			logger.Debug("Client error",
				zap.Error(err))
			return c.JSON(http.StatusUnauthorized, apierrors.Error{
				Error:   ErrInvalidToken.Error(),
				Message: http.StatusText(http.StatusUnauthorized),
			})
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			sub, _ := claims["sub"].(string)
			userID, err := primitive.ObjectIDFromHex(sub)
			if err != nil {
				log.Println(apiutils.HandlerErrorLogMessage(err, c))
				return c.JSON(http.StatusUnauthorized, apierrors.Error{
					Error:   "invalid token sub",
					Message: http.StatusText(http.StatusUnauthorized),
				})
			}

			c.Set("user", userID.Hex())
			return next(c)
		}

		log.Println(apiutils.HandlerErrorLogMessage(ErrInvalidToken, c))
		return c.JSON(http.StatusUnauthorized, apierrors.Error{
			Error:   ErrInvalidToken.Error(),
			Message: http.StatusText(http.StatusUnauthorized),
		})
	}
}
