package apierror

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type ErrorMessage = string

const (
	NotFoundError       ErrorMessage = "record not found"
	InternalServerError ErrorMessage = "internal server error"
	EmailConflictError  ErrorMessage = "email already in use"
	// PermissionError  ErrorMessage   = "The user %s doesn't have the necessary permissions"
)

type Error struct {
	Error   string       `json:"error"`
	Message ErrorMessage `json:"message"`
}

func CustomError(context echo.Context, https int, message ErrorMessage) error {
	newError := Error{
		Error:   http.StatusText(https),
		Message: message,
	}
	return context.JSON(https, newError)
}
