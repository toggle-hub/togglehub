package testutils

import (
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/suite"
)

type DefaultTestSuite struct {
	suite.Suite
	Server *echo.Echo
}
