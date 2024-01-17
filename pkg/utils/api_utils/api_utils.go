package api_utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/oauth2"
)

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

type OAuthClientProduction struct {
	config *oauth2.Config
}

func NewOAuthClient(config *oauth2.Config) *OAuthClientProduction {
	return &OAuthClientProduction{config: config}
}

func (c *OAuthClientProduction) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
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
var ErrContextUserTypeAssertion = errors.New("unable to assert type of user id in context")
var ErrContextOrganizationTypeAssertion = errors.New("unable to assert type of organization id in context")
var ErrReadPermissionDenied = errors.New("user does not have read permission")
var ErrNoOrganization = errors.New("organization not set in context")

func GetUserFromContext(c echo.Context) (primitive.ObjectID, error) {
	ctxUser := c.Get("user")
	if ctxUser == nil {
		return primitive.NilObjectID, ErrNotAuthenticated
	}

	objectIDHex, ok := ctxUser.(string)
	if !ok {
		return primitive.NilObjectID, ErrContextUserTypeAssertion
	}

	userID, err := primitive.ObjectIDFromHex(objectIDHex)
	if err != nil {
		return primitive.NilObjectID, err
	}
	return userID, nil
}

func GetOrganizationFromContext(c echo.Context) (primitive.ObjectID, error) {
	ctxOrganization := c.Get("organization")
	if ctxOrganization == nil {
		return primitive.NilObjectID, ErrNoOrganization
	}

	objectIDHex, ok := ctxOrganization.(string)
	if !ok {
		return primitive.NilObjectID, ErrContextOrganizationTypeAssertion
	}

	organizationID, err := primitive.ObjectIDFromHex(objectIDHex)
	if err != nil {
		return primitive.NilObjectID, err
	}

	return organizationID, nil
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
	organization *organizationmodel.OrganizationRecord,
	permission organizationmodel.PermissionLevelEnum,
) bool {
	for _, member := range organization.Members {
		if member.User.ID == userID {
			switch permission {
			case organizationmodel.Admin:
				return member.PermissionLevel == permission
			case organizationmodel.Collaborator:
				return member.PermissionLevel == permission || member.PermissionLevel == organizationmodel.Admin
			case organizationmodel.ReadOnly:
				return member.PermissionLevel == permission ||
					member.PermissionLevel == organizationmodel.Collaborator ||
					member.PermissionLevel == organizationmodel.Admin
			}
		}
	}

	return false
}
