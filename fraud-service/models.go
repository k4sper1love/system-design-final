package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
		log.Fatal(err)
	}
	fmt.Println("Connected to Redis")
}

type FraudRule struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Threshold   float64 `json:"threshold"`
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
