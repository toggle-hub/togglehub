package middlewares

import (
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func ZapLogger(log *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			request := c.Request()
			response := c.Response()

			id := request.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = response.Header().Get(echo.HeaderXRequestID)
			}

			fields := []zapcore.Field{
				zap.String("id", id),
				zap.String("remote_ip", c.RealIP()),
				zap.String("host", request.Host),
				zap.String("method", request.Method),
				zap.String("uri", request.RequestURI),
				zap.String("user_agent", request.UserAgent()),
				zap.Int("status", response.Status),
				zap.String("latency", time.Since(start).String()),
				zap.Int64("bytes_in", request.ContentLength),
				zap.Int64("bytes_out", response.Size),
			}

			n := response.Status
			switch {
			case n >= 500:
				log.Error("Server error", fields...)
			case n >= 400:
				log.Warn("Client error", fields...)
			case n >= 300:
				log.Info("Redirection", fields...)
			default:
				log.Info("Success", fields...)
			}

			return nil
		}
	}
}
