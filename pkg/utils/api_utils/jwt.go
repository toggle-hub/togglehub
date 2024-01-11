package api_utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func CreateJWT(id primitive.ObjectID, expireAt time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "togglelabs",
		"sub": id.Hex(),
		"exp": time.Now().Add(expireAt * time.Millisecond).Unix(),
	})

	key := os.Getenv("JWT_SECRET")
	if key == "" {
		key = "your-secret-key"
	}
	signedToken, err := token.SignedString([]byte(key))

	if err != nil {
		return "", err
	}

	return signedToken, nil
}
