package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client
var ctx = context.Background()

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis: ", err)
	}
	fmt.Println("Connected to Redis")

	checkAndCreateDefaultRule()
}

func checkAndCreateDefaultRule() {
	keys, err := rdb.Keys(ctx, "rule:*").Result()
	if err != nil {
		log.Printf("Error checking rules: %v", err)
		return
	}

	if len(keys) == 0 {
		log.Println("No fraud rules found, creating default rule")
		defaultRule := FraudRule{
			ID:          "default",
			Description: "Default threshold rule",
			Threshold:   5000,
			CreatedAt:   time.Now(),
		}

		ruleJSON, err := json.Marshal(defaultRule)
		if err != nil {
			log.Printf("Error marshaling default rule: %v", err)
			return
		}

		err = rdb.Set(ctx, "rule:default", ruleJSON, 0).Err()
		if err != nil {
			log.Printf("Error saving default rule: %v", err)
			return
		}

		log.Println("Default rule created successfully")
	}
}

type FraudRule struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Threshold   float64   `json:"threshold"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

func getFraudRule(ruleID string) (*FraudRule, error) {
	val, err := rdb.Get(ctx, "rule:"+ruleID).Result()
	if err == redis.Nil {
		return nil, nil 
	} else if err != nil {
		return nil, err
	}

	var rule FraudRule
	err = json.Unmarshal([]byte(val), &rule)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}
