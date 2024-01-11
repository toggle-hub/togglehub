package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	api_errors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	api_utils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type SsoHandler struct {
	oauthConfig *oauth2.Config
	db          *mongo.Database
	logger      *zap.Logger
	httpClient  api_utils.BaseHTTPClient
	oauthClient api_utils.OAuthClient
}

func NewSsoHandler(
	db *mongo.Database,
	oauthConfig *oauth2.Config,
	logger *zap.Logger,
	httpClient api_utils.BaseHTTPClient,
	oauthClient api_utils.OAuthClient,
) *SsoHandler {
	return &SsoHandler{
		oauthConfig: oauthConfig,
		db:          db,
		logger:      logger,
		httpClient:  httpClient,
		oauthClient: oauthClient,
	}
}

func (sh *SsoHandler) Signin(c echo.Context) error {
	randomString := os.Getenv("OAUTH_RANDOM_STRING")

	if randomString == "" {
		randomString = "random-string"
	}

	url := sh.oauthConfig.AuthCodeURL(randomString)
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func (sh *SsoHandler) Callback(c echo.Context) error {
	state := c.QueryParam("state")
	code := c.QueryParam("code")

	userDataBytes, err := sh.getUserOAuthData(state, code)

	if err != nil {
		sh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}
	userData := new(models.UserRecord)
	err = json.Unmarshal(userDataBytes, userData)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	model := models.NewUserModel(sh.db)
	foundRecord, err := model.FindByEmail(context.Background(), userData.Email)
	if err == nil {
		token, err := api_utils.CreateJWT(foundRecord.ID, config.JWTExpireTime)
		if err != nil {
			sh.logger.Debug("Server error",
				zap.String("cause", err.Error()),
			)
			return api_errors.CustomError(
				c,
				http.StatusInternalServerError,
				api_errors.InternalServerError,
			)
		}

		return c.JSON(http.StatusOK, common.AuthResponse{
			ID:        foundRecord.ID,
			Email:     foundRecord.Email,
			FirstName: foundRecord.FirstName,
			LastName:  foundRecord.LastName,
			Token:     token,
		})
	}

	objectID, err := model.InsertOne(context.Background(), userData)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	token, err := api_utils.CreateJWT(objectID, config.JWTExpireTime)
	if err != nil {
		sh.logger.Debug("Server error",
			zap.String("cause", err.Error()),
		)
		return api_errors.CustomError(
			c,
			http.StatusInternalServerError,
			api_errors.InternalServerError,
		)
	}

	sh.logger.Debug("User created",
		zap.String("_id", objectID.Hex()),
	)
	return c.JSON(http.StatusCreated, common.AuthResponse{
		ID:    userData.ID,
		Email: userData.Email,
		Token: token,
	})
}

func (sh *SsoHandler) getUserOAuthData(
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
