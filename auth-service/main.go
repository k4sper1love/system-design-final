package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	initDB()

	go cleanupBlacklist()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/register", register)
	e.POST("/login", login)
	e.POST("/refresh", refreshToken)
	e.POST("/logout", logout)
	e.GET("/check", checkToken)

	protectedGroup := e.Group("")
	protectedGroup.Use(JWTMiddleware)
	protectedGroup.GET("/profile", getProfileProtected)

	e.Logger.Fatal(e.Start(":8081"))
}
