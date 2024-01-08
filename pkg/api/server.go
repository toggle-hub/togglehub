package api

import (
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/api/middlewares"
	"github.com/Roll-Play/togglelabs/pkg/storage"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

type App struct {
	server  *echo.Echo
	port    string
	storage *storage.MongoStorage
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

func NewApp(port string, storage *storage.MongoStorage) *App {
	server := echo.New()

	app := &App{
		server:  server,
		port:    normalizePort(port),
		storage: storage,
	}

	registerRoutes(app)

	return app
}

func registerRoutes(app *App) {
	app.server.GET("/healthz", handlers.HealthHandler)

	ssoHandler := handlers.NewSsoHandler(&oauth2.Config{}, app.storage.DB())
	app.server.POST("/oauth", ssoHandler.Signin)
	app.server.GET("/callback", ssoHandler.Callback)

	signUpHandler := handlers.NewSignUpHandler(app.storage.DB())
	app.server.POST("/signup", signUpHandler.PostUser)

	signInHandler := handlers.NewSignInHandler(app.storage.DB())
	app.server.POST("/signin", signInHandler.PostSignIn)

	userHandler := handlers.NewUserHandler(app.storage.DB())
	userGroup := app.server.Group("/user", middlewares.AuthMiddleware)
	userGroup.PATCH("", userHandler.PatchUser)

	organizationHandler := handlers.NewOrganizationHandler(app.storage.DB())
	organizationGroup := app.server.Group("/organization", middlewares.AuthMiddleware)
	organizationGroup.POST("", organizationHandler.PostOrganization)

	featureFlagHandler := handlers.NewFeatureFlagHandler(app.storage.DB())
	organizationGroup.POST("/:organizationID/feature-flag", featureFlagHandler.PostFeatureFlag)
	organizationGroup.PATCH("/:organizationID/feature-flag/:featureFlagID", featureFlagHandler.PatchFeatureFlag)
	organizationGroup.GET("/:organizationID/feature-flag", featureFlagHandler.ListFeatureFlags)
	organizationGroup.PATCH(
		"/:organizationID/feature-flag/:featureFlagID/revision/:revisionID",
		featureFlagHandler.ApproveRevision,
	)
	organizationGroup.PATCH("/:organizationID/feature-flag/:featureFlagID/rollback", featureFlagHandler.RollbackFeatureFlagVersion)
}
