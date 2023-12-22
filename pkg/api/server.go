package api

import (
	handler "github.com/Roll-Play/togglelabs/pkg/api/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
)

type FiberHandlerFunc = func(c *fiber.Ctx) error

type Server struct {
	app  *fiber.App
	port string
	conn *sqlx.DB
}

func (s *Server) Listen() error {
	return s.app.Listen(s.port)
}

func (s *Server) get(path string, handlers ...FiberHandlerFunc) {
	s.app.Get(path, handlers...)
}

// func (s *Server) post(path string, handlers ...FiberHandlerFunc) {
// 	s.app.Post(path, handlers...)
// }

// func (s *Server) put(path string, handlers ...FiberHandlerFunc) {
// 	s.app.Put(path, handlers...)
// }

// func (s *Server) patch(path string, handlers ...FiberHandlerFunc) {
// 	s.app.Patch(path, handlers...)
// }

func normalizePort(port string) string {
	if []byte(port)[0] != ':' {
		return string(append([]byte(":"), []byte(port)...))
	}

	return port
}

func NewServer(port string, conn *sqlx.DB) *Server {
	app := fiber.New(fiber.Config{
		AppName:       "togglelabs-api",
		CaseSensitive: true,
	})

	server := &Server{
		app:  app,
		port: normalizePort(port),
		conn: conn,
	}

	registerRoutes(server)
	return server
}

func registerRoutes(server *Server) {
	server.get("/healthz", handler.HealthHandler)
}

func NewHandler(path string, fn FiberHandlerFunc) map[string]FiberHandlerFunc {
	return map[string]FiberHandlerFunc{path: fn}
}
