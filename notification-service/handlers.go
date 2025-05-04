package main

import (
	"encoding/json"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
	"log"
)

var natsConn *nats.Conn

func initNATS() {
	var err error
	natsConn, err = nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to NATS")
}

func sendSMS(phoneNumber, message string) error {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: "your_twilio_account_sid",
		Password: "your_twilio_auth_token",
	})

	params := &twilioApi.CreateMessageParams{}
	params.SetTo(phoneNumber)
	params.SetFrom("+1234567890")
	params.SetBody(message)

	_, err := client.Api.CreateMessage(params)
	if err != nil {
		return err
	}
	return nil
}

func handleTransactionEvent(msg *nats.Msg) {
	var event struct {
		UserID uint    `json:"user_id"`
		Amount float64 `json:"amount"`
		Status string  `json:"status"`
		Phone  string  `json:"phone"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal event: %v", err)
		return
	}

	message := fmt.Sprintf("Transaction status: %s. Amount: %.2f", event.Status, event.Amount)
	if err := sendSMS(event.Phone, message); err != nil {
		log.Printf("Failed to send SMS to %s: %v", event.Phone, err)
	} else {
		log.Printf("SMS sent to %s", event.Phone)
	}
}

func subscribeToNATS() {
	_, err := natsConn.Subscribe("transaction.status", handleTransactionEvent)
	if err != nil {
		log.Fatalf("Failed to subscribe to NATS: %v", err)
	}
	fmt.Println("Subscribed to transaction.status events")
}
