package apiutils

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Roll-Play/togglelabs/pkg/config"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

type RealHttpClient struct{}

func (c *RealHttpClient) Get(url string) (*http.Response, error) {
	return http.Get(url)
}

type OAuthClient interface {
	Exchange(context.Context, string) (*oauth2.Token, error)
}

type RealOAuthClient struct {
	config *oauth2.Config
}

func NewRealOAuthClient(config *oauth2.Config) *RealOAuthClient {
	return &RealOAuthClient{config: config}
}

func (c *RealOAuthClient) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return c.config.Exchange(ctx, code)
}

func GetPaginationParams(page, limit string) (int, int) {
	if page == "" {
		page = "1"
	}

	if limit == "" {
		limit = "10"
	}

	// Atoi can return an error if the string size is < 1
	// but because we check it before converting it'll probably
	// never happen
	pageNumber, _ := strconv.Atoi(page)
	limitNumber, _ := strconv.Atoi(limit)

	return pageNumber, limitNumber
}

func EncryptPassword(password string) (string, error) {
	encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), config.BCryptCost)
	if err != nil {
		return "", err
	}

	return string(encryptedPassword), nil
}
