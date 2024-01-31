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

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
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

	oauthHandler := handlers.NewOAuthHandler(
		app.storage.DB(),
		oauthConfig,
		app.logger,
		&apiutils.HTTPClient{},
		apiutils.NewOAuthClient(oauthConfig),
	)

	app.server.POST("/oauth", oauthHandler.SignIn)
	app.server.GET("/callback", oauthHandler.Callback)

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := sqs.New(sess)
	queueName := os.Getenv("SQS_QUEUE_NAME")
	result, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		panic(err)
	}

	signUpHandler := handlers.NewSignUpHandler(app.storage.DB(), app.logger, sess, result.QueueUrl)
	app.server.POST("/signup", signUpHandler.PostUser)

	signInHandler := handlers.NewSignInHandler(app.storage.DB(), app.logger)
	app.server.POST("/signin", signInHandler.PostSignIn)

	userHandler := handlers.NewUserHandler(app.storage.DB(), app.logger)
	userGroup := app.server.Group("/user", middlewares.AuthMiddleware)
	userGroup.GET("", userHandler.GetUser)
	userGroup.PATCH("", userHandler.PatchUser)

	organizationHandler := handlers.NewOrganizationHandler(app.storage.DB(), app.logger)
	app.server.POST("/organizations", middlewares.AuthMiddleware(organizationHandler.PostOrganization))
	app.server.GET("/organizations", middlewares.AuthMiddleware(organizationHandler.GetOrganization), middlewares.OrganizationMiddleware)
	app.server.POST("/projects", middlewares.AuthMiddleware(organizationHandler.PostProject), middlewares.OrganizationMiddleware)
	app.server.DELETE("/projects/:projectID", middlewares.AuthMiddleware(organizationHandler.DeleteProject), middlewares.OrganizationMiddleware)

	featureFlagHandler := handlers.NewFeatureFlagHandler(app.storage.DB(), app.logger)
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
	featureGroup.PATCH("/:featureFlagID/tags", featureFlagHandler.PatchFeatureFlagTags)
}
