package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/elkin/system-design-final/shared/fraudpb"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupTestRedis(t *testing.T) *miniredis.Miniredis {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	rdb = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Создаем тестовое правило
	rule := FraudRule{
		ID:          "test-rule",
		Description: "Test Rule",
		Threshold:   1000.0,
		CreatedAt:   time.Now(),
	}
	ruleJSON, _ := json.Marshal(rule)
	rdb.Set(ctx, "rule:test-rule", ruleJSON, 0)

	return mr
}

func TestCheckTransaction(t *testing.T) {
	mr := setupTestRedis(t)
	defer mr.Close()

	server := &fraudServer{}

	safeReq := &fraudpb.FraudCheckRequest{
		TransactionId: 1,
		UserId:        123,
		Amount:        500.0,
	}
	safeResp, err := server.CheckTransaction(context.Background(), safeReq)
	assert.NoError(t, err)
	assert.Equal(t, "safe", safeResp.Status)
	assert.Less(t, safeResp.FraudScore, 80.0)

	suspiciousReq := &fraudpb.FraudCheckRequest{
		TransactionId: 2,
		UserId:        123,
		Amount:        1500.0,
	}
	suspiciousResp, err := server.CheckTransaction(context.Background(), suspiciousReq)
	assert.NoError(t, err)
	assert.Equal(t, "suspicious", suspiciousResp.Status)
	assert.GreaterOrEqual(t, suspiciousResp.FraudScore, 80.0)
}

func TestCreateFraudRule(t *testing.T) {
	mr := setupTestRedis(t)
	defer mr.Close()

	e := echo.New()

	rule := map[string]interface{}{
		"description": "Test API Rule",
		"threshold":   2000.0,
	}
	ruleJSON, _ := json.Marshal(rule)

	req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewReader(ruleJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert.NoError(t, createFraudRule(c))
	assert.Equal(t, http.StatusCreated, rec.Code)

	var createdRule map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &createdRule)
	assert.NoError(t, err)
	assert.Equal(t, "Test API Rule", createdRule["description"])
	assert.Equal(t, 2000.0, createdRule["threshold"])
	assert.NotEmpty(t, createdRule["id"])
}

func TestGetRule(t *testing.T) {
	mr := setupTestRedis(t)
	defer mr.Close()

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/rules/:id")
	c.SetParamNames("id")
	c.SetParamValues("test-rule")

	assert.NoError(t, getRule(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var rule map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &rule)
	assert.NoError(t, err)
	assert.Equal(t, "test-rule", rule["id"])
	assert.Equal(t, "Test Rule", rule["description"])
	assert.Equal(t, 1000.0, rule["threshold"])
}

func TestGetAllRules(t *testing.T) {
	mr := setupTestRedis(t)
	defer mr.Close()

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/rules", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert.NoError(t, getAllRules(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var rules []*FraudRule
	err := json.Unmarshal(rec.Body.Bytes(), &rules)
	assert.NoError(t, err)
	assert.Len(t, rules, 1)
	assert.Equal(t, "test-rule", rules[0].ID)
}

func TestUpdateRule(t *testing.T) {
	mr := setupTestRedis(t)
	defer mr.Close()

	e := echo.New()

	rule := map[string]interface{}{
		"description": "Updated Test Rule",
		"threshold":   1500.0,
	}
	ruleJSON, _ := json.Marshal(rule)

	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(ruleJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/rules/:id")
	c.SetParamNames("id")
	c.SetParamValues("test-rule")

	assert.NoError(t, updateRule(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var updatedRule map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &updatedRule)
	assert.NoError(t, err)
	assert.Equal(t, "test-rule", updatedRule["id"])
	assert.Equal(t, "Updated Test Rule", updatedRule["description"])
	assert.Equal(t, 1500.0, updatedRule["threshold"])
}

func TestDeleteRule(t *testing.T) {
	mr := setupTestRedis(t)
	defer mr.Close()

	e := echo.New()

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/rules/:id")
	c.SetParamNames("id")
	c.SetParamValues("test-rule")

	assert.NoError(t, deleteRule(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	exists, err := rdb.Exists(ctx, "rule:test-rule").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}
