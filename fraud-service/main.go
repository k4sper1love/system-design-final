package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/elkin/system-design-final/shared/fraudpb"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var rdb *redis.Client
var ctx = context.Background()

type fraudServer struct {
	fraudpb.UnimplementedFraudCheckerServer
}

func (s *fraudServer) CheckTransaction(ctx context.Context, req *fraudpb.FraudCheckRequest) (*fraudpb.FraudCheckResponse, error) {
	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "amount must be positive")
	}

	log.Printf("Fraud check: user_id=%d, amount=%.2f, transaction_id=%d",
		req.UserId, req.Amount, req.TransactionId)

	rules, err := getAllFraudRules()
	if err != nil {
		log.Printf("Error getting fraud rules: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to fetch fraud rules")
	}

	if len(rules) == 0 {
		return &fraudpb.FraudCheckResponse{
			FraudScore: 0,
			Status:     "safe",
		}, nil
	}

	maxScore := 0.0
	for _, rule := range rules {
		score := calculateRiskScore(req.Amount, rule)
		if score > maxScore {
			maxScore = score
		}
	}

	status := "safe"
	if maxScore >= 80 {
		status = "suspicious"
		saveSuspiciousTransaction(req, maxScore)
	}

	return &fraudpb.FraudCheckResponse{
		FraudScore: maxScore,
		Status:     status,
	}, nil
}

func saveSuspiciousTransaction(req *fraudpb.FraudCheckRequest, score float64) {
	key := fmt.Sprintf("suspicious:tx:%d:%d", req.UserId, time.Now().Unix())
	data := map[string]interface{}{
		"user_id":        req.UserId,
		"transaction_id": req.TransactionId,
		"amount":         req.Amount,
		"score":          score,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	_, err := rdb.HMSet(ctx, key, data).Result()
	if err != nil {
		log.Printf("Failed to save suspicious transaction: %v", err)
	}

	rdb.Expire(ctx, key, 30*24*time.Hour)
}

func getAllFraudRules() ([]*FraudRule, error) {
	keys, err := rdb.Keys(ctx, "rule:*").Result()
	if err != nil {
		return nil, err
	}

	var rules []*FraudRule
	if len(keys) == 0 {
		return rules, nil
	}

	for _, key := range keys {
		rule, err := getFraudRule(key[5:])
		if err != nil {
			log.Printf("Error getting rule %s: %v", key, err)
			continue
		}
		if rule != nil {
			rules = append(rules, rule)
		}
	}

	return rules, nil
}

func calculateRiskScore(amount float64, rule *FraudRule) float64 {
	if amount > rule.Threshold {
		return 90.0
	}

	percentage := amount / rule.Threshold
	if percentage > 0.8 {
		return 70.0 + (percentage-0.8)*100.0
	} else if percentage > 0.5 {
		return 30.0 + (percentage-0.5)*133.33
	}

	return 10.0 + percentage*40.0
}

func main() {
	initRedis()

	go func() {
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		grpcServer := grpc.NewServer()
		fraudpb.RegisterFraudCheckerServer(grpcServer, &fraudServer{})
		log.Println("Fraud gRPC server started on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/fraud/check", checkFraud)                      
	e.POST("/rules", createFraudRule)                       
	e.GET("/rules", getAllRules)                              
	e.GET("/rules/:id", getRule)                                
	e.PUT("/rules/:id", updateRule)                              
	e.DELETE("/rules/:id", deleteRule)                           
	e.GET("/transactions/suspicious", getSuspiciousTransactions) 

	log.Println("Fraud REST API server started on :8083")
	e.Logger.Fatal(e.Start(":8083"))
}
