package handler

import (
	"github.com/gofiber/fiber/v2"
)

type HealthResponse struct {
	Alive bool `json:"alive"`
}

func HealthHandler(c *fiber.Ctx) error {
	r := HealthResponse{Alive: true}

	return c.JSON(r)
}
