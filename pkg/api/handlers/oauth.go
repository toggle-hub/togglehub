package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/config"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type OAuthHandler struct {
	oauthConfig *oauth2.Config
	db          *mongo.Database
	logger      *zap.Logger
	httpClient  apiutils.BaseHTTPClient
	oauthClient apiutils.OAuthClient
}

func NewOAuthHandler(
	db *mongo.Database,
	oauthConfig *oauth2.Config,
	logger *zap.Logger,
	httpClient apiutils.BaseHTTPClient,
	oauthClient apiutils.OAuthClient,
) *OAuthHandler {
	return &OAuthHandler{
		oauthConfig: oauthConfig,
		db:          db,
		logger:      logger,
		httpClient:  httpClient,
		oauthClient: oauthClient,
	}
}

func (sh *OAuthHandler) SignIn(c echo.Context) error {
	randomString := os.Getenv("OAUTH_RANDOM_STRING")

	if randomString == "" {
		randomString = "random-string"
	}

	url := sh.oauthConfig.AuthCodeURL(randomString)
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func (sh *OAuthHandler) Callback(c echo.Context) error {
	state := c.QueryParam("state")
	code := c.QueryParam("code")

	userDataBytes, err := sh.getUserOAuthData(state, code)

	if err != nil {
		sh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	userData := new(usermodel.UserRecord)
	err = json.Unmarshal(userDataBytes, userData)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	model := usermodel.New(sh.db)
	foundRecord, err := model.FindByEmail(context.Background(), userData.Email)
	if err == nil {
		token, err := apiutils.CreateJWT(foundRecord.ID, config.JWTExpireTime)
		if err != nil {
			sh.logger.Debug("Server error",
				zap.Error(err),
			)
			return apierrors.CustomError(
				c,
				http.StatusInternalServerError,
				apierrors.InternalServerError,
			)
		}

		cookie := new(http.Cookie)
		cookie.Name = "Authorization"
		cookie.Value = "Bearer " + token
		cookie.Expires = time.Now().Add(config.JWTExpireTime * time.Millisecond)
		cookie.HttpOnly = true
		c.SetCookie(cookie)

		return c.JSON(http.StatusOK, common.AuthResponse{
			ID:        foundRecord.ID,
			Email:     foundRecord.Email,
			FirstName: foundRecord.FirstName,
			LastName:  foundRecord.LastName,
		})
	}

	objectID, err := model.InsertOne(context.Background(), userData)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	token, err := apiutils.CreateJWT(objectID, config.JWTExpireTime)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.Error(err),
		)
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	cookie := new(http.Cookie)
	cookie.Name = "Authorization"
	cookie.Value = "Bearer " + token
	cookie.Expires = time.Now().Add(config.JWTExpireTime * time.Millisecond)
	cookie.HttpOnly = true
	c.SetCookie(cookie)

	sh.logger.Debug("User created",
		zap.String("_id", objectID.Hex()),
	)
	return c.JSON(http.StatusCreated, common.AuthResponse{
		ID:    userData.ID,
		Email: userData.Email,
	})
}

func (sh *OAuthHandler) getUserOAuthData(
	state string,
	code string,
) ([]byte, error) {
	randomString := os.Getenv("OAUTH_RANDOM_STRING")
	if randomString == "" {
		randomString = "random-string"
	}

	if state != randomString {
		return nil, errors.New("invalid user state")
	}

	token, err := sh.oauthClient.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}

	response, err := sh.httpClient.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}
