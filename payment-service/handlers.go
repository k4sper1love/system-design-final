package main

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"net/http"

	"github.com/labstack/echo/v4"
)

func topUpBalance(c echo.Context) error {
	type TopUpRequest struct {
		UserID uint    `json:"user_id"`
		Amount float64 `json:"amount"`
	}
	var req TopUpRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var balance Balance
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&balance, "user_id = ?", req.UserID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	balance.Balance += req.Amount
	balance.Version++
	if err := tx.Save(&balance).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update balance"})
	}

	transaction := Transaction{
		SenderID:        req.UserID,
		RecipientID:     nil,
		Amount:          req.Amount,
		Status:          "completed",
		TransactionType: "top_up",
		Description:     "Balance top-up",
	}
	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create transaction"})
	}

	tx.Commit()
	return c.JSON(http.StatusOK, map[string]string{"message": "Balance updated successfully"})
}

func getBalance(c echo.Context) error {
	userID := c.Param("user_id")

	var userIDInt uint
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user_id format"})
	}

	var balance Balance
	if err := db.First(&balance, "user_id = ?", userIDInt).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found or balance does not exist"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user_id": balance.UserID,
		"balance": balance.Balance,
	})
}

func processTransaction(c echo.Context) error {
	type TransactionRequest struct {
		SenderID uint    `json:"sender_id"`
		Amount   float64 `json:"amount"`
	}
	var req TransactionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	fraudResponse, err := checkFraudWithService(req.SenderID, req.Amount)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Fraud check failed"})
	}
	if fraudResponse.Status == "suspicious" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Suspicious transaction"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Transaction processed successfully"})
}

func checkFraudWithService(senderID uint, amount float64) (*FraudResponse, error) {
	type FraudResponse struct {
		Status string `json:"status"`
	}

	resp, err := http.Post("http://localhost:8083/fraud/check", "application/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var fraudResp FraudResponse
	if err := json.NewDecoder(resp.Body).Decode(&fraudResp); err != nil {
		return nil, err
	}
	return &fraudResp, nil
}

func publishTransactionStatus(userID uint, amount float64, status, phone string) {
	event := map[string]interface{}{
		"user_id": userID,
		"amount":  amount,
		"status":  status,
		"phone":   phone,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	err = natsConn.Publish("transaction.status", jsonData)
	if err != nil {
		log.Printf("Failed to publish event: %v", err)
	} else {
		log.Printf("Published transaction status event for user %d", userID)
	}
}
