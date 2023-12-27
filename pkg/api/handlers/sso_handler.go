package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/config"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const USER_COLLECTION = "user"

type SsoHandler struct {
	ssogolang *oauth2.Config
	db        *mongo.Database
}

type UserInfo struct {
	SsoId string `json:"id" bson:"sso_id"`
	Email string `json:"email" bson:"id"`
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
		db: db,
	}
}

func (sh *SsoHandler) Signin(c echo.Context) error {
	url := sh.ssogolang.AuthCodeURL(RandomString)
	fmt.Println(url)
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func (sh *SsoHandler) Callback(c echo.Context) error {
	state := c.QueryParam("state")
	code := c.QueryParam("code")
	httpClient := &apiutils.RealHttpClient{}
	customOAuthClient := apiutils.NewRealOAuthClient(sh.ssogolang)

	userDataBytes, err := GetUserData(state, code, customOAuthClient, httpClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	var userData = new(UserInfo)
	err = json.Unmarshal(userDataBytes, userData)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	collection := sh.db.Collection(USER_COLLECTION)
	ctx, cancel := context.WithTimeout(context.Background(), config.DBFetchTimeout*time.Second)
	defer cancel()

	_, err = collection.InsertOne(ctx, userData)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, userData)
}

func GetUserData(state, code string, ssogolang apiutils.OAuthClient, httpClient apiutils.HTTPClient) ([]byte, error) {

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

	return data, nil
}
