package fixtures

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/Roll-Play/togglelabs/pkg/models"
	"golang.org/x/oauth2"
)

type MockHTTPClient struct{}

func (c *MockHTTPClient) Get(_ string) (*http.Response, error) {
	response := httptest.NewRecorder()
	userInfo := models.UserRecord{
		SsoID: "12345",
		Email: "test@test.com",
	}
	body, err := json.Marshal(userInfo)
	if err != nil {
		panic(err)
	}

	response.Body.Write(body)
	return response.Result(), nil
}

type MockOAuthClient struct {
	ExchangeFunc func(ctx context.Context, code string) (*oauth2.Token, error)
}

func (m *MockOAuthClient) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return m.ExchangeFunc(ctx, code)
}
