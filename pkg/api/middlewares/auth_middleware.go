package middlewares

import (
	"errors"
	"net/http"
	"strings"

	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ContextUser struct {
	ID primitive.ObjectID
}

func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(context echo.Context) error {
		authHeader := context.Request().Header.Get("Authorization")

		if authHeader == "" {
			return context.JSON(http.StatusUnauthorized, apierrors.Error{
				Error:   "Missing Authorization header",
				Message: http.StatusText(http.StatusUnauthorized),
			})
		}

		tokenString := strings.TrimSpace(strings.Replace(authHeader, "Bearer ", "", 1))
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			secretKey := []byte("your-secret-key")

			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}

			return secretKey, nil
		})

		if err != nil {
			return context.JSON(http.StatusUnauthorized, apierrors.Error{
				Error:   "Invalid token",
				Message: http.StatusText(http.StatusUnauthorized),
			})
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID, ok := claims["sub"].(primitive.ObjectID)
			if !ok {
				return context.JSON(http.StatusUnauthorized, apierrors.Error{
					Error:   "Invalid token",
					Message: http.StatusText(http.StatusUnauthorized),
				})
			}

			context.Set("user", ContextUser{
				ID: userID,
			})
			return next(context)
		}

		return context.JSON(http.StatusUnauthorized, apierrors.Error{
			Error:   "Invalid token",
			Message: http.StatusText(http.StatusUnauthorized),
		})
	}
}
