package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func checkFraud(c echo.Context) error {
	type FraudRequest struct {
		TransactionID int     `json:"transaction_id"`
		UserID        uint64  `json:"user_id"`
		Amount        float64 `json:"amount"`
	}
	var req FraudRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	rules, err := getAllFraudRules()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch fraud rules"})
	}

	var maxScore float64
	for _, rule := range rules {
		score := calculateRiskScore(req.Amount, rule)
		if score > maxScore {
			maxScore = score
		}
	}

	status := "safe"
	if maxScore >= 80 {
		status = "suspicious"
	}

	response := map[string]interface{}{
		"transaction_id": req.TransactionID,
		"fraud_score":    maxScore,
		"status":         status,
	}

	return c.JSON(http.StatusOK, response)
}

func createFraudRule(c echo.Context) error {
	var rule FraudRule
	if err := c.Bind(&rule); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if rule.Description == "" || rule.Threshold <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Description and threshold are required"})
	}

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to marshal rule"})
	}

	if err := rdb.Set(ctx, "rule:"+rule.ID, ruleJSON, 0).Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save rule"})
	}

	return c.JSON(http.StatusCreated, rule)
}

func getAllRules(c echo.Context) error {
	rules, err := getAllFraudRules()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch rules"})
	}
	return c.JSON(http.StatusOK, rules)
}

func getRule(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Rule ID is required"})
	}

	rule, err := getFraudRule(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch rule"})
	}
	if rule == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Rule not found"})
	}

	return c.JSON(http.StatusOK, rule)
}

func updateRule(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Rule ID is required"})
	}

	existingRule, err := getFraudRule(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch rule"})
	}
	if existingRule == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Rule not found"})
	}

	var rule FraudRule
	if err := c.Bind(&rule); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	rule.ID = id

	if rule.Description == "" || rule.Threshold <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Description and threshold are required"})
	}

	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to marshal rule"})
	}

	if err := rdb.Set(ctx, "rule:"+id, ruleJSON, 0).Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update rule"})
	}

	return c.JSON(http.StatusOK, rule)
}

func deleteRule(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Rule ID is required"})
	}

	existingRule, err := getFraudRule(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch rule"})
	}
	if existingRule == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Rule not found"})
	}

	if err := rdb.Del(ctx, "rule:"+id).Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete rule"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Rule deleted successfully"})
}

func getSuspiciousTransactions(c echo.Context) error {
	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.QueryParam("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	keys, err := rdb.Keys(ctx, "suspicious:tx:*").Result()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch transactions"})
	}

	startIdx := (page - 1) * limit
	endIdx := startIdx + limit
	if startIdx >= len(keys) {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"transactions": []interface{}{},
			"total":        len(keys),
			"page":         page,
			"limit":        limit,
		})
	}
	if endIdx > len(keys) {
		endIdx = len(keys)
	}
	keys = keys[startIdx:endIdx]

	transactions := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		data, err := rdb.HGetAll(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch transaction data"})
		}

		tx := make(map[string]interface{})
		for k, v := range data {
			switch k {
			case "user_id", "transaction_id":
				val, _ := strconv.ParseUint(v, 10, 64)
				tx[k] = val
			case "amount", "score":
				val, _ := strconv.ParseFloat(v, 64)
				tx[k] = val
			default:
				tx[k] = v
			}
		}
		tx["id"] = key[len("suspicious:tx:"):]
		transactions = append(transactions, tx)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"transactions": transactions,
		"total":        len(keys),
		"page":         page,
		"limit":        limit,
	})
}
