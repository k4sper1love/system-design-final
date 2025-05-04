package main

import (
	"github.com/labstack/echo/v4"
)

func main() {
	initDB()

	e := echo.New()
	e.POST("/register", register)
	e.POST("/login", login)

	e.Logger.Fatal(e.Start(":8081"))
}
