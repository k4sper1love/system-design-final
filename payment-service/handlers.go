package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/elkin/system-design-final/shared/fraudpb"
	"github.com/labstack/echo/v4"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

var fraudClient fraudpb.FraudCheckerClient
var natsConn *nats.Conn

func initFraudClient() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		log.Fatalf("Failed to connect to fraud-service: %v", err)
	}
	fraudClient = fraudpb.NewFraudCheckerClient(conn)
}

func initNATS() {
	var err error
	natsConn, err = nats.Connect("nats://localhost:4222", nats.MaxReconnects(10))
	if err != nil {
		log.Fatal("Failed to connect to NATS: ", err)
	}
	log.Println("Connected to NATS")
}

func JWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing or invalid Authorization header"})
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		valid, userID, err := validateTokenWithAuthService(tokenString)
		if err != nil || !valid {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
		}

		c.Set("user_id", userID)
		return next(c)
	}
}

func validateTokenWithAuthService(tokenString string) (bool, uint, error) {
	req, err := http.NewRequest("GET", "http://localhost:8081/check", nil)
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tokenString)

	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, 0, errors.New("token validation failed")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, 0, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, 0, err
	}

	valid, ok := result["valid"].(bool)
	if !ok || !valid {
		return false, 0, errors.New("invalid token response")
	}

	userIDFloat, ok := result["user_id"].(float64)
	if !ok {
		return false, 0, errors.New("user_id not found")
	}

	return true, uint(userIDFloat), nil
}

func topUpBalance(c echo.Context) error {
	type TopUpRequest struct {
		UserID uint    `json:"user_id"`
		Amount float64 `json:"amount"`
	}
	var req TopUpRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.UserID == 0 || req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user_id or amount"})
	}

	if req.Amount > 10000 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		fraudResp, err := fraudClient.CheckTransaction(ctx, &fraudpb.FraudCheckRequest{
			TransactionId: 0,
			UserId:        uint64(req.UserID),
			Amount:        req.Amount,
		})
		if err != nil {
			log.Printf("Fraud check failed: %v", err)
		} else if fraudResp.Status == "suspicious" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Suspicious transaction"})
		}
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var balance Balance
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&balance, "user_id = ?", req.UserID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			balance = Balance{
				UserID:  req.UserID,
				Balance: 0,
				Version: 1,
			}
			if err := tx.Create(&balance).Error; err != nil {
				tx.Rollback()
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create balance"})
			}
		} else {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
		}
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

	publishTransactionEvent(transaction.ID, req.UserID, req.Amount, "top_up", "completed")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":        "Balance updated successfully",
		"balance":        balance.Balance,
		"transaction_id": transaction.ID,
	})
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	fraudResp, err := fraudClient.CheckTransaction(ctx, &fraudpb.FraudCheckRequest{
		TransactionId: 0,
		UserId:        uint64(req.SenderID),
		Amount:        req.Amount,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Fraud check failed"})
	}
	if fraudResp.Status == "suspicious" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Suspicious transaction"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Transaction processed successfully"})
}

func transferFunds(c echo.Context) error {
	type TransferRequest struct {
		SenderID    uint    `json:"sender_id"`
		RecipientID uint    `json:"recipient_id"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description,omitempty"`
	}
	var req TransferRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Amount must be positive"})
	}
	if req.SenderID == 0 || req.RecipientID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid sender or recipient"})
	}
	if req.SenderID == req.RecipientID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Sender and recipient must be different"})
	}

	userID, ok := c.Get("user_id").(uint)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	if userID != req.SenderID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only transfer from your own account"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	fraudResp, err := fraudClient.CheckTransaction(ctx, &fraudpb.FraudCheckRequest{
		TransactionId: 0,
		UserId:        uint64(req.SenderID),
		Amount:        req.Amount,
	})
	if err != nil {
		log.Printf("Fraud check error: %v", err)
	} else if fraudResp != nil && fraudResp.Status == "suspicious" {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":       "Transaction flagged as suspicious",
			"fraud_score": fraudResp.FraudScore,
		})
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sender, recipient Balance
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&sender, "user_id = ?", req.SenderID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Sender balance not found"})
	}
	if err := tx.Set("gorm:query_option", "FOR UPDATE").FirstOrCreate(&recipient, Balance{UserID: req.RecipientID}).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get recipient balance"})
	}

	if sender.Balance < req.Amount {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":    "Insufficient funds",
			"balance":  sender.Balance,
			"required": req.Amount,
		})
	}

	sender.Balance -= req.Amount
	recipient.Balance += req.Amount
	sender.Version++
	recipient.Version++

	if err := tx.Save(&sender).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update sender balance"})
	}
	if err := tx.Save(&recipient).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update recipient balance"})
	}

	description := "Transfer between users"
	if req.Description != "" {
		description = req.Description
	}

	transaction := Transaction{
		SenderID:        req.SenderID,
		RecipientID:     &req.RecipientID,
		Amount:          req.Amount,
		Status:          "completed",
		TransactionType: "transfer",
		Description:     description,
	}

	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create transaction record"})
	}

	if err := tx.Commit().Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to commit transaction"})
	}

	publishTransactionEvent(transaction.ID, req.SenderID, req.Amount, "transfer_sent", "completed")
	publishTransactionEvent(transaction.ID, req.RecipientID, req.Amount, "transfer_received", "completed")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":        "Transfer successful",
		"transaction_id": transaction.ID,
		"sender_balance": sender.Balance,
	})
}

func getTransactionHistory(c echo.Context) error {
	userID := c.Param("user_id")
	var userIDInt uint
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user_id format"})
	}
	var transactions []Transaction
	if err := db.Where("sender_id = ? OR recipient_id = ?", userIDInt, userIDInt).Order("created_at desc").Find(&transactions).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch transactions"})
	}
	return c.JSON(http.StatusOK, transactions)
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

func publishTransactionEvent(transactionID uint, userID uint, amount float64, transactionType, status string) {
	if natsConn == nil {
		log.Println("NATS not initialized")
		return
	}

	event := map[string]interface{}{
		"transaction_id": transactionID,
		"user_id":        userID,
		"amount":         amount,
		"type":           transactionType,
		"status":         status,
		"timestamp":      time.Now(),
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal transaction event: %v", err)
		return
	}

	err = natsConn.Publish("transactions", jsonData)
	if err != nil {
		log.Printf("Failed to publish transaction event: %v", err)
	} else {
		log.Printf("Published transaction event: %s", string(jsonData))
	}
}
