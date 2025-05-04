package main

import (
	"github.com/labstack/echo/v4"
)

func main() {
	initDB()

	e := echo.New()
	e.POST("/balance/top-up", topUpBalance)
	e.GET("/balance/:user_id", getBalance)

	e.Logger.Fatal(e.Start(":8082"))
}
