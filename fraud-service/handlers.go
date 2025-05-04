package main

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

func checkFraud(c echo.Context) error {
	type FraudRequest struct {
		TransactionID int     `json:"transaction_id"`
		Amount        float64 `json:"amount"`
	}
	var req FraudRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	rule, err := getFraudRule("1")
	if err != nil || rule == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch fraud rule"})
	}

	fraudScore := calculateFraudScore(req.Amount, rule.Threshold)

	response := map[string]interface{}{
		"transaction_id": req.TransactionID,
		"fraud_score":    fraudScore,
		"status":         "safe",
	}
	if fraudScore > rule.Threshold {
		response["status"] = "suspicious"
	}

	return c.JSON(http.StatusOK, response)
}

func calculateFraudScore(amount, threshold float64) float64 {
	if amount > threshold {
		return 90
	}
	return 50
}
