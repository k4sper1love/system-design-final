package main

import (
	"github.com/labstack/echo/v4"
)

func main() {
	initRedis()

	e := echo.New()
	e.POST("/fraud/check", checkFraud)

	e.Logger.Fatal(e.Start(":8083"))
}
