package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierrors "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type SsoHandler struct {
	ssogolang         *oauth2.Config
	db                *mongo.Database
	httpClient        apiutils.BaseHTTPClient
	customOAuthClient apiutils.OAuthClient
}

var RandomString = "random-string"

func NewSsoHandler(ssogolang *oauth2.Config, db *mongo.Database) *SsoHandler {
	return &SsoHandler{
		ssogolang: &oauth2.Config{
			RedirectURL:  os.Getenv("REDIRECT_URL"),
			ClientID:     os.Getenv("CLIENT_ID"),
			ClientSecret: os.Getenv("CLIENT_SECRET"),
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "openid"},
			Endpoint:     google.Endpoint,
		},
		db:                db,
		httpClient:        &apiutils.HTTPClient{},
		customOAuthClient: apiutils.NewRealOAuthClient(ssogolang),
	}
}

func NewSsoHandlerForTest(
	db *mongo.Database,
	httpClient apiutils.BaseHTTPClient,
	oauthClient apiutils.OAuthClient,
) *SsoHandler {
	return &SsoHandler{
		ssogolang:         &oauth2.Config{},
		db:                db,
		httpClient:        httpClient,
		customOAuthClient: oauthClient,
	}
}

func (sh *SsoHandler) Signin(c echo.Context) error {
	url := sh.ssogolang.AuthCodeURL(RandomString)
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func (sh *SsoHandler) Callback(c echo.Context) error {
	state := c.QueryParam("state")
	code := c.QueryParam("code")
	log.Print(state, code)
	userDataBytes, err := GetUserData(state, code, sh.customOAuthClient, sh.httpClient)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}
	userData := new(models.UserRecord)
	err = json.Unmarshal(userDataBytes, userData)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	model := models.NewUserModel(sh.db)
	foundRecord, err := model.FindByEmail(context.Background(), userData.Email)
	if err == nil {
		log.Println(apiutils.HandlerErrorLogMessage(errors.New(apierrors.EmailConflictError), c))
		token, err := apiutils.CreateJWT(foundRecord.ID, config.JWTExpireTime)
		if err != nil {
			log.Println(apiutils.HandlerErrorLogMessage(err, c))
			return apierrors.CustomError(
				c,
				http.StatusInternalServerError,
				apierrors.InternalServerError,
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
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	token, err := apiutils.CreateJWT(objectID, config.JWTExpireTime)
	if err != nil {
		log.Println(apiutils.HandlerErrorLogMessage(err, c))
		return apierrors.CustomError(
			c,
			http.StatusInternalServerError,
			apierrors.InternalServerError,
		)
	}

	log.Println(apiutils.HandlerLogMessage("user", objectID, c))
	return c.JSON(http.StatusCreated, common.AuthResponse{
		ID:    userData.ID,
		Email: userData.Email,
		Token: token,
	})
}

func GetUserData(
	state string,
	code string,
	ssogolang apiutils.OAuthClient,
	httpClient apiutils.BaseHTTPClient,
) ([]byte, error) {
	if state != RandomString {
		return nil, errors.New("invalid user state")
	}

	token, err := ssogolang.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}

	response, err := httpClient.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
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
