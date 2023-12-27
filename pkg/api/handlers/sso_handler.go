package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	apierror "github.com/Roll-Play/togglelabs/pkg/api/error"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/models"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const USER_COLLECTION = "user"

type SsoHandler struct {
	ssogolang         *oauth2.Config
	db                *mongo.Database
	httpClient        apiutils.HTTPClient
	customOAuthClient apiutils.OAuthClient
}

type UserInfo struct {
	ID    primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	SsoId string             `json:"sso_id" bson:"sso_id"`
	Email string             `json:"email" bson:"email"`
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
		httpClient:        &apiutils.RealHttpClient{},
		customOAuthClient: apiutils.NewRealOAuthClient(ssogolang),
	}
}

func NewSsoHandlerForTest(db *mongo.Database, httpClient apiutils.HTTPClient, oauthClient apiutils.OAuthClient) *SsoHandler {
	return &SsoHandler{
		ssogolang:         &oauth2.Config{},
		db:                db,
		httpClient:        httpClient,
		customOAuthClient: oauthClient,
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
	log.Print(state, code)
	userDataBytes, err := GetUserData(state, code, sh.customOAuthClient, sh.httpClient)
	if err != nil {
		return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
	}
	var userData = new(UserInfo)
	err = json.Unmarshal(userDataBytes, userData)
	if err != nil {
		return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
	}
	collection := sh.db.Collection(USER_COLLECTION)
	ctx, cancel := context.WithTimeout(context.Background(), config.DBFetchTimeout*time.Second)
	defer cancel()

	var foundRecord models.UserRecord
	err = collection.FindOne(context.Background(), bson.D{{Key: "email", Value: userData.Email}}).Decode(&foundRecord)
	if err == nil {
		token, err := apiutils.CreateJWT(foundRecord.ID, config.JWTExpireTime)
		if err != nil {
			return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
		}
		return c.JSON(http.StatusOK, common.AuthResponse{
			ID:        foundRecord.ID,
			Email:     foundRecord.Email,
			FirstName: foundRecord.FirstName,
			LastName:  foundRecord.LastName,
			Token:     token,
		})
	}

	result, err := collection.InsertOne(ctx, userData)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	objectID := result.InsertedID
	oID, ok := objectID.(primitive.ObjectID)
	if !ok {
		return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
	}

	token, err := apiutils.CreateJWT(oID, config.JWTExpireTime)
	if err != nil {
		return apierror.CustomError(c, http.StatusInternalServerError, apierror.InternalServerError)
	}

	return c.JSON(http.StatusCreated, common.AuthResponse{
		ID:    userData.ID,
		Email: userData.Email,
		Token: token,
	})
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
