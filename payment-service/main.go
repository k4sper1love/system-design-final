package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	initDB()
	initFraudClient()
	initNATS()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/balance/:user_id", getBalance)

	protected := e.Group("")
	protected.Use(JWTMiddleware)

	protected.POST("/balance/top-up", topUpBalance)

	protected.POST("/transactions/transfer", transferFunds)
	protected.POST("/transactions/process", processTransaction)
	protected.GET("/transactions/history/:user_id", getTransactionHistory)

	e.Logger.Fatal(e.Start(":8082"))
}
