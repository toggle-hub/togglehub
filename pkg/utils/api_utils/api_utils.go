package apiutils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Roll-Play/togglelabs/pkg/models"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/oauth2"
)

type ContextUser struct {
	ID primitive.ObjectID
}

type BaseHTTPClient interface {
	Get(url string) (*http.Response, error)
}

type HTTPClient struct{}

func (c *HTTPClient) Get(url string) (*http.Response, error) {
	return http.Get(url) //nolint
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

var ErrNotAuthenticated = errors.New("user not authenticated")
var ErrContextUserTypeAssertion = errors.New("unable to assert type of user in context")
var ErrReadPermissionDenied = errors.New("user does not have read permission")

func GetObjectIDFromContext(c echo.Context) (primitive.ObjectID, error) {
	ctxUser := c.Get("user")
	if ctxUser == nil {
		return primitive.NilObjectID, ErrNotAuthenticated
	}

	user, ok := c.Get("user").(ContextUser)

	if !ok {
		return primitive.NilObjectID, ErrContextUserTypeAssertion
	}

	return user.ID, nil
}

func HandlerErrorLogMessage(err error, c echo.Context) string {
	return fmt.Sprintf(
		"[Error]: {\"error\": \"%s\", \"ip\": \"%s\", \"location\": \"%s\"}",
		err.Error(),
		c.RealIP(),
		c.Request().URL.Path,
	)
}

func HandlerLogMessage(resource string, id primitive.ObjectID, c echo.Context) string {
	return fmt.Sprintf(
		"[Log]: {\"resource\": \"%s\", \"action\":\"%s\", \"_id\":\"%s\", \"ip\": \"%s\", \"location\": \"%s\"}",
		resource,
		c.Request().Method,
		id,
		c.RealIP(),
		c.Request().URL.Path,
	)
}

func UserHasPermission(
	userID primitive.ObjectID,
	organization *models.OrganizationRecord,
	permission models.PermissionLevelEnum,
) bool {
	for _, member := range organization.Members {
		if member.User.ID == userID {
			switch permission {
			case models.Admin:
				return member.PermissionLevel == permission
			case models.Collaborator:
				return member.PermissionLevel == permission || member.PermissionLevel == models.Admin
			case models.ReadOnly:
				return member.PermissionLevel == permission ||
					member.PermissionLevel == models.Collaborator ||
					member.PermissionLevel == models.Admin
			}
		}
	}

	return false
}
