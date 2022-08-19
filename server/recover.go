package server

import (
	"github.com/bytebase/bytebase/common/log"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func recoverMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = errors.Errorf("%v", r)
				}
				log.Error("Middleware PANIC RECOVER", zap.Error(err))

				c.Error(err)
			}
		}()
		return next(c)
	}
}
