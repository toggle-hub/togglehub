package apierrors

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type ErrorMessage = string

const (
	NotFoundError       ErrorMessage = "record not found"
	InternalServerError ErrorMessage = "internal server error"
	EmailConflictError  ErrorMessage = "email already in use"
	UnauthorizedError   ErrorMessage = "user lacks valid authentication credentials"
	BadRequestError     ErrorMessage = "malformed request"
	ForbiddenError      ErrorMessage = "forbidden action"
)

type Error struct {
	Error   string       `json:"error"`
	Message ErrorMessage `json:"message"`
}

func CustomError(c echo.Context, httpStatus int, message ErrorMessage) error {
	return c.JSON(httpStatus, Error{
		Error:   http.StatusText(httpStatus),
		Message: message,
	})
}
