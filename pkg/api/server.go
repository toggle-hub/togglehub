package api

import (
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/Roll-Play/togglelabs/pkg/storage"
	"github.com/labstack/echo/v4"
)

type App struct {
	server  *echo.Echo
	port    string
	storage *storage.MongoStorage
}

func (a *App) Listen() error {
	return a.server.Start(a.port)
}

func (a *App) get(path string, handler echo.HandlerFunc, middlewares ...echo.MiddlewareFunc) {
	a.server.GET(path, handler, middlewares...)
}

func (a *App) post(path string, handler echo.HandlerFunc, middlewares ...echo.MiddlewareFunc) {
	a.server.POST(path, handler, middlewares...)
}

// func (a *App) put(path string, handler echo.HandlerFunc, middlewares ...echo.MiddlewareFunc) {
// 	a.server.Put(path, handlers...)
// }

// func (a *App) patch(path string, handler echo.HandlerFunc, middlewares ...echo.MiddlewareFunc) {
// 	a.server.Patch(path, handlers...)
// }

func normalizePort(port string) string {
	if []byte(port)[0] != ':' {
		return string(append([]byte(":"), []byte(port)...))
	}

	return port
}

func NewApp(port string, storage *storage.MongoStorage) *App {
	server := echo.New()

	App := &App{
		server:  server,
		port:    normalizePort(port),
		storage: storage,
	}

	registerRoutes(App)
	return App
}

func registerRoutes(app *App) {
	app.get("/healthz", handlers.HealthHandler)

	signUpHandler := handlers.NewSignUpHandler(app.storage.DB())
	app.post("/signup", signUpHandler.PostUser)

	signInHandler := handlers.NewSignInHandler(app.storage.DB())
	app.post("/signin", signInHandler.PostSignIn)
}
