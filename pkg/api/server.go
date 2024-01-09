package api

import (
	"os"

	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/storage"
	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
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
	app.server.GET("/healthz", handlers.HealthHandler)

	oauthConfig := &oauth2.Config{
		RedirectURL:  os.Getenv("REDIRECT_URL"),
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "openid"},
		Endpoint:     google.Endpoint,
	}

	ssoHandler := handlers.NewSsoHandler(
		app.storage.DB(),
		oauthConfig,
		app.logger,
		&apiutils.HTTPClient{},
		apiutils.NewOAuthClient(oauthConfig),
	)
	app.server.POST("/oauth", ssoHandler.Signin)
	app.server.GET("/callback", ssoHandler.Callback)

	signUpHandler := handlers.NewSignUpHandler(app.storage.DB(), app.logger)
	app.server.POST("/signup", signUpHandler.PostUser)

	signInHandler := handlers.NewSignInHandler(app.storage.DB(), app.logger)
	app.server.POST("/signin", signInHandler.PostSignIn)

	userHandler := handlers.NewUserHandler(app.storage.DB(), app.logger)
	userGroup := app.server.Group("/user", middlewares.AuthMiddleware)
	userGroup.PATCH("", userHandler.PatchUser)

	organizationHandler := handlers.NewOrganizationHandler(app.storage.DB(), app.logger)
	organizationGroup := app.server.Group("/organizations", middlewares.AuthMiddleware)
	organizationGroup.POST("", organizationHandler.PostOrganization)

	featureFlagHandler := handlers.NewFeatureFlagHandler(app.storage.DB(), app.logger)
	organizationGroup.POST("/:organizationID/feature-flags", featureFlagHandler.PostFeatureFlag)
	organizationGroup.PATCH("/:organizationID/feature-flags/featureFlagID", featureFlagHandler.PatchFeatureFlag)
	organizationGroup.GET("/:organizationID/feature-flags", featureFlagHandler.ListFeatureFlags)
	organizationGroup.PATCH(
		"/:organizationID/feature-flags/featureFlagID/revisions/:revisionID",
		featureFlagHandler.ApproveRevision,
	)
	organizationGroup.DELETE("/:organizationID/feature-flags/featureFlagID", featureFlagHandler.DeleteFeatureFlag)
}
