package api

import (
	"github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
)

type Server struct {
	app  *echo.Echo
	port string
	db   *mongo.Database
}

func (s *Server) Listen() error {
	return s.app.Start(s.port)
}

func (s *Server) get(path string, handler echo.HandlerFunc, middlewares ...echo.MiddlewareFunc) {
	s.app.GET(path, handler, middlewares...)
}

func (s *Server) post(path string, handler echo.HandlerFunc, middlewares ...echo.MiddlewareFunc) {
	s.app.POST(path, handler, middlewares...)
}

// func (s *Server) put(path string, handler echo.HandlerFunc, middlewares ...echo.MiddlewareFunc) {
// 	s.app.Put(path, handlers...)
// }

// func (s *Server) patch(path string, handler echo.HandlerFunc, middlewares ...echo.MiddlewareFunc) {
// 	s.app.Patch(path, handlers...)
// }

func normalizePort(port string) string {
	if []byte(port)[0] != ':' {
		return string(append([]byte(":"), []byte(port)...))
	}

	return port
}

func NewServer(port string, db *mongo.Database) *Server {
	app := echo.New()

	server := &Server{
		app:  app,
		port: normalizePort(port),
		db:   db,
	}

	registerRoutes(server)
	return server
}

func registerRoutes(server *Server) {
	server.get("/healthz", handlers.HealthHandler)

	exampleHandler := handlers.NewExampleRouter(server.db)
	server.get("/example", exampleHandler.GetExamples)
	signUpHandler := handlers.NewSignUpHandler(server.db)
	server.post("/signup", signUpHandler.PostUser)

	ssoHandler := handlers.NewSsoHandler(&oauth2.Config{}, server.db)
	server.get("/signin", ssoHandler.Signin)
	server.get("/callback", ssoHandler.Callback)
}
