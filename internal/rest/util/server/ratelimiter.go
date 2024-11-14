package server

import (
	tollbooth "github.com/didip/tollbooth/v6"
	limiter "github.com/didip/tollbooth/v6/limiter"
	"github.com/labstack/echo/v4"
)

func LimitMiddleware(limiter *limiter.Limiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return echo.HandlerFunc(func(c echo.Context) error {
			if err := tollbooth.LimitByRequest(limiter, c.Response(), c.Request()); err != nil {
				return c.String(err.StatusCode, err.Message)
			}
			return next(c)
		})
	}
}

func LimitHandler(limiter *limiter.Limiter) echo.MiddlewareFunc {
	return LimitMiddleware(limiter)
}
