package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() {
	var err error
	db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&Balance{}, &Transaction{})

	balance := Balance{
		UserID:  1,
		Balance: 1000,
		Version: 1,
	}
	db.Create(&balance)
}

func TestGetBalance(t *testing.T) {
	setupTestDB()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/balance/:user_id")
	c.SetParamNames("user_id")
	c.SetParamValues("1")

	if err := getBalance(c); err != nil {
		t.Errorf("getBalance failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if resp["balance"] != float64(1000) {
		t.Errorf("Expected balance 1000, got %v", resp["balance"])
	}
}

func TestTopUpBalance(t *testing.T) {
	setupTestDB()
	e := echo.New()

	payload := map[string]interface{}{
		"user_id": 1,
		"amount":  500,
	}
	jsonBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(jsonBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := topUpBalance(c); err != nil {
		t.Errorf("topUpBalance failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	var balance Balance
	db.First(&balance, "user_id = ?", 1)
	if balance.Balance != 1500 {
		t.Errorf("Expected balance 1500 after top-up, got %v", balance.Balance)
	}

	var transaction Transaction
	db.First(&transaction, "sender_id = ? AND transaction_type = ?", 1, "top_up")
	if transaction.Amount != 500 {
		t.Errorf("Expected transaction amount 500, got %v", transaction.Amount)
	}
}

func TestTransferFunds(t *testing.T) {
	setupTestDB()

	recipient := Balance{
		UserID:  2,
		Balance: 0,
		Version: 1,
	}
	db.Create(&recipient)

	e := echo.New()

	payload := map[string]interface{}{
		"sender_id":    1,
		"recipient_id": 2,
		"amount":       300,
		"description":  "Test transfer",
	}
	jsonBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(jsonBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set("user_id", uint(1))

	if err := transferFunds(c); err != nil {
		t.Errorf("transferFunds failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	var senderBalance, recipientBalance Balance
	db.First(&senderBalance, "user_id = ?", 1)
	db.First(&recipientBalance, "user_id = ?", 2)

	if senderBalance.Balance != 700 {
		t.Errorf("Expected sender balance 700 after transfer, got %v", senderBalance.Balance)
	}

	if recipientBalance.Balance != 300 {
		t.Errorf("Expected recipient balance 300 after transfer, got %v", recipientBalance.Balance)
	}

	var transaction Transaction
	db.First(&transaction, "sender_id = ? AND transaction_type = ?", 1, "transfer")
	if transaction.Amount != 300 {
		t.Errorf("Expected transaction amount 300, got %v", transaction.Amount)
	}
	if *transaction.RecipientID != 2 {
		t.Errorf("Expected recipient ID 2, got %v", *transaction.RecipientID)
	}
}
