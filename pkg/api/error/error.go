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
<<<<<<< HEAD
	BadRequestError     ErrorMessage = "malformed request"
=======
	BadRequestError     ErrorMessage = "bad request"
>>>>>>> 1d564d630e53fba521f547b28edeac0b185a31b6
)

type Error struct {
	Error   string       `json:"error"`
	Message ErrorMessage `json:"message"`
}

func CustomError(c echo.Context, httpStatus int, message ErrorMessage) error {
	newError := Error{
		Error:   http.StatusText(httpStatus),
		Message: message,
	}
	return c.JSON(httpStatus, newError)
}
