package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"time"
)

func main() {
	e := echo.New()
	e.Use(middleware.Logger())

	rateLimiter := NewRateLimiter(10, 1*time.Second)
	e.Use(RateLimitMiddleware(rateLimiter))

	e.Use(JWTMiddleware)

	e.POST("/auth/register", func(c echo.Context) error {
		return c.String(http.StatusOK, "Register endpoint")
	})
	e.POST("/auth/login", func(c echo.Context) error {
		return c.String(http.StatusOK, "Login endpoint")
	})
	e.POST("/payment/top-up", func(c echo.Context) error {
		return c.String(http.StatusOK, "Top-up endpoint")
	})
	e.GET("/payment/balance/:user_id", func(c echo.Context) error {
		return c.String(http.StatusOK, "Balance endpoint")
	})

	e.Logger.Fatal(e.Start(":8080"))
}
