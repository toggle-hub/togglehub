package api

import (
	"os"

	featureflaghandler "github.com/Roll-Play/togglelabs/pkg/api/handlers/feature_flag"
	healthzhandler "github.com/Roll-Play/togglelabs/pkg/api/handlers/healthz"
	organizationhandler "github.com/Roll-Play/togglelabs/pkg/api/handlers/organization"
	signinhandler "github.com/Roll-Play/togglelabs/pkg/api/handlers/sign_in"
	signuphandler "github.com/Roll-Play/togglelabs/pkg/api/handlers/sign_up"
	ssohandler "github.com/Roll-Play/togglelabs/pkg/api/handlers/sso"
	userhandler "github.com/Roll-Play/togglelabs/pkg/api/handlers/user"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/storage"
	api_utils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type App struct {
	port    string
	server  *echo.Echo
	storage *storage.MongoStorage
	logger  *zap.Logger
}

func (a *App) Listen() error {
	return a.server.Start(a.port)
}

func normalizePort(port string) string {
	if []byte(port)[0] != ':' {
		return string(append([]byte(":"), []byte(port)...))
	}

	return port
}

func NewApp(port string, storage *storage.MongoStorage, logger *zap.Logger) *App {
	server := echo.New()

	app := &App{
		server:  server,
		port:    normalizePort(port),
		storage: storage,
		logger:  logger,
	}
	app.server.Use(middlewares.ZapLogger(logger))

	registerRoutes(app)

	return app
}

func registerRoutes(app *App) {
	app.server.GET("/healthz", healthzhandler.HealthHandler)

	oauthConfig := &oauth2.Config{
		RedirectURL:  os.Getenv("REDIRECT_URL"),
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "openid"},
		Endpoint:     google.Endpoint,
	}

	ssoHandler := ssohandler.New(
		app.storage.DB(),
		oauthConfig,
		app.logger,
		&api_utils.HTTPClient{},
		api_utils.NewOAuthClient(oauthConfig),
	)
	app.server.POST("/oauth", ssoHandler.SignIn)
	app.server.GET("/callback", ssoHandler.Callback)

	signUpHandler := signuphandler.New(app.storage.DB(), app.logger)
	app.server.POST("/signup", signUpHandler.PostUser)

	signInHandler := signinhandler.New(app.storage.DB(), app.logger)
	app.server.POST("/signin", signInHandler.PostSignIn)

	userHandler := userhandler.New(app.storage.DB(), app.logger)
	userGroup := app.server.Group("/user", middlewares.AuthMiddleware)
	userGroup.GET("", userHandler.GetUser)
	userGroup.PATCH("", userHandler.PatchUser)

	organizationHandler := organizationhandler.New(app.storage.DB(), app.logger)
	app.server.POST("/organizations", middlewares.AuthMiddleware(organizationHandler.PostOrganization))

	featureFlagHandler := featureflaghandler.New(app.storage.DB(), app.logger)
	featureGroup := app.server.Group("/features", middlewares.AuthMiddleware, middlewares.OrganizationMiddleware)
	featureGroup.POST("", featureFlagHandler.PostFeatureFlag)
	featureGroup.GET("", featureFlagHandler.ListFeatureFlags)
	featureGroup.PATCH("/:featureFlagID", featureFlagHandler.PatchFeatureFlag)
	featureGroup.PATCH(
		"/:featureFlagID/revisions/:revisionID",
		featureFlagHandler.ApproveRevision,
	)
	featureGroup.DELETE("/:featureFlagID", featureFlagHandler.DeleteFeatureFlag)
	featureGroup.PATCH(
		"/:featureFlagID/rollback",
		featureFlagHandler.RollbackFeatureFlagVersion,
	)
	featureGroup.PATCH(
		"/:featureFlagID/toggle",
		featureFlagHandler.ToggleFeatureFlag,
	)
}
