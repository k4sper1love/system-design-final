package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/nats-io/nats.go"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

var (
	natsConn     *nats.Conn
	twilioClient *twilio.RestClient
	wsClient     *websocket.Conn
	sentNotifications = make(map[string]*Notification)
)

type Notification struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Recipient   string    `json:"recipient"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	SentAt      time.Time `json:"sent_at,omitempty"`
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
}

type TransactionEvent struct {
	TransactionID uint64    `json:"transaction_id"`
	UserID        uint64    `json:"user_id"`
	Amount        float64   `json:"amount"`
	Type          string    `json:"type"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	Phone         string    `json:"phone,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true 
	},
}

var (
	callbackClients      = make(map[*websocket.Conn]bool)
	callbackClientsMutex sync.Mutex
)

func main() {
	initNATS()
	defer natsConn.Close()
	initTwilio()
	initWebSocketClient()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	subscribeToNATS()

	e.GET("/health", healthCheck)
	e.POST("/test-sms", testSMS)
	e.GET("/notifications", getNotifications)
	e.GET("/notifications/:id", getNotificationByID)

	e.GET("/ws/callbacks", handleWebSocketCallbacks)

	port := getEnv("PORT", "8083")
	log.Printf("Starting Notification Service on :%s", port)
	log.Fatal(e.Start(":" + port))
}

func initNATS() {
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")
	var err error
	natsConn, err = nats.Connect(natsURL, nats.MaxReconnects(10))
	if err != nil {
		log.Fatalf("Failed to connect to NATS at %s: %v", natsURL, err)
	}
	log.Printf("Connected to NATS at %s", natsURL)
}

func initTwilio() {
	twilioAccountSid := getEnv("TWILIO_ACCOUNT_SID", "your_twilio_account_sid")
	twilioAuthToken := getEnv("TWILIO_AUTH_TOKEN", "your_twilio_auth_token")

	twilioClient = twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: twilioAccountSid,
		Password: twilioAuthToken,
	})
	log.Println("Twilio client initialized")
}

func initWebSocketClient() {
	smsGatewayURL := getEnv("SMS_GATEWAY_URL", "ws://localhost:8083/sms")

	var dialer websocket.Dialer
	dialer.HandshakeTimeout = time.Second * 5

	conn, _, err := dialer.Dial(smsGatewayURL, nil)
	if err != nil {
		log.Printf("Warning: Failed to connect to SMS Gateway WebSocket: %v", err)
		return
	}

	log.Printf("Connected to SMS Gateway WebSocket at %s", smsGatewayURL)
	wsClient = conn

	go readWebSocketResponses()
}

func readWebSocketResponses() {
	if wsClient == nil {
		return
	}

	for {
		_, message, err := wsClient.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			wsClient.Close()
			wsClient = nil
			return
		}

		var response map[string]interface{}
		if err := json.Unmarshal(message, &response); err != nil {
			log.Printf("Failed to parse WebSocket response: %v", err)
			continue
		}

		log.Printf("Received SMS Gateway response: %v", response)

		if id, ok := response["notification_id"].(string); ok {
			if notification, exists := sentNotifications[id]; exists {
				if status, ok := response["status"].(string); ok {
					notification.Status = status
				}
				if message, ok := response["message"].(string); ok {
					notification.ErrorMsg = message
				}
				if status == "delivered" {
					notification.DeliveredAt = time.Now()
				}
			}
		}
	}
}

func subscribeToNATS() {
	_, err := natsConn.Subscribe("transactions", handleTransactionEvent)
	if err != nil {
		log.Fatalf("Failed to subscribe to 'transactions' subject: %v", err)
	}
	log.Println("Subscribed to 'transactions' events")

	_, err = natsConn.Subscribe("transaction.status", handleTransactionStatusEvent)
	if err != nil {
		log.Fatalf("Failed to subscribe to 'transaction.status' subject: %v", err)
	}
	log.Println("Subscribed to 'transaction.status' events")
}

func handleTransactionEvent(msg *nats.Msg) {
	var event TransactionEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal transaction event: %v", err)
		return
	}

	log.Printf("Received transaction event: ID=%d, Type=%s, Amount=%.2f",
		event.TransactionID, event.Type, event.Amount)

	messageTemplate := getNotificationTemplate(event.Type, event.Status)


	phone := event.Phone
	if phone == "" {
		phone = fmt.Sprintf("+7%d", event.UserID) 
	}

	message := strings.ReplaceAll(messageTemplate, "{amount}", fmt.Sprintf("%.2f", event.Amount))
	message = strings.ReplaceAll(message, "{status}", event.Status)
	message = strings.ReplaceAll(message, "{transaction_id}", fmt.Sprintf("%d", event.TransactionID))

	notification := &Notification{
		ID:        fmt.Sprintf("tx_%d_%d", event.TransactionID, time.Now().Unix()),
		Type:      "transaction_" + event.Type,
		Recipient: phone,
		Content:   message,
		Status:    "pending",
	}

	go sendSMSAsync(notification)
}

func handleTransactionStatusEvent(msg *nats.Msg) {
	var event struct {
		UserID uint64  `json:"user_id"`
		Amount float64 `json:"amount"`
		Status string  `json:"status"`
		Phone  string  `json:"phone"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal transaction status event: %v", err)
		return
	}

	log.Printf("Received transaction status event: UserID=%d, Amount=%.2f, Status=%s",
		event.UserID, event.Amount, event.Status)

	messageTemplate := getStatusNotificationTemplate(event.Status)

	phone := event.Phone
	if phone == "" {
		phone = fmt.Sprintf("+7%d", event.UserID) 
	}

	message := strings.ReplaceAll(messageTemplate, "{amount}", fmt.Sprintf("%.2f", event.Amount))
	message = strings.ReplaceAll(message, "{status}", event.Status)

	notification := &Notification{
		ID:        fmt.Sprintf("status_%d_%d", event.UserID, time.Now().Unix()),
		Type:      "transaction_status",
		Recipient: phone,
		Content:   message,
		Status:    "pending",
	}

	go sendSMSAsync(notification)
}

func sendSMSAsync(notification *Notification) {
	sentNotifications[notification.ID] = notification

	err := sendSMS(notification.Recipient, notification.Content)

	notification.SentAt = time.Now()
	if err != nil {
		notification.Status = "failed"
		notification.ErrorMsg = err.Error()
		log.Printf("Failed to send SMS to %s: %v", notification.Recipient, err)
	} else {
		notification.Status = "sent"
		log.Printf("SMS sent to %s: %s", notification.Recipient, notification.Content)
	}

	broadcastNotificationStatus(notification)
}

func sendSMS(phoneNumber, message string) error {
	if wsClient != nil {
		notificationID := fmt.Sprintf("sms_%d", time.Now().UnixNano())

		request := map[string]interface{}{
			"notification_id": notificationID,
			"to":              phoneNumber,
			"message":         message,
			"type":            "sms",
		}

		requestJSON, err := json.Marshal(request)
		if err != nil {
			return fmt.Errorf("failed to marshal WebSocket request: %w", err)
		}

		if err := wsClient.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
			log.Printf("WebSocket write error: %v, falling back to direct API", err)
			wsClient = nil
			return sendSMSViaTwilioAPI(phoneNumber, message)
		}

		log.Printf("SMS request sent via WebSocket: %s", string(requestJSON))
		return nil
	}

	return sendSMSViaTwilioAPI(phoneNumber, message)
}

func sendSMSViaTwilioAPI(phoneNumber, message string) error {
	if !strings.HasPrefix(phoneNumber, "+") {
		phoneNumber = "+" + phoneNumber
	}

	twilioPhoneNumber := getEnv("TWILIO_PHONE_NUMBER", "+1234567890")

	params := &twilioApi.CreateMessageParams{}
	params.SetTo(phoneNumber)
	params.SetFrom(twilioPhoneNumber)
	params.SetBody(message)

	_, err := twilioClient.Api.CreateMessage(params)
	return err
}

func getNotificationTemplate(eventType, status string) string {
	switch eventType {
	case "top_up":
		return "Your account has been topped up with {amount}."
	case "transfer_sent":
		return "You have sent {amount} to another user."
	case "transfer_received":
		return "You have received {amount} from another user."
	default:
		return "Transaction {transaction_id}: {status}. Amount: {amount}"
	}
}

func getStatusNotificationTemplate(status string) string {
	switch status {
	case "completed":
		return "Your transaction for {amount} has been completed successfully."
	case "pending":
		return "Your transaction for {amount} is being processed."
	case "failed":
		return "Your transaction for {amount} has failed."
	default:
		return "Transaction status: {status}. Amount: {amount}"
	}
}

func healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":         "Notification service is healthy",
		"nats_connected": fmt.Sprintf("%t", natsConn.IsConnected()),
	})
}

func testSMS(c echo.Context) error {
	type TestSMSRequest struct {
		Phone   string `json:"phone"`
		Message string `json:"message"`
	}
	var req TestSMSRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}
	if req.Phone == "" || req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing phone or message"})
	}

	notification := &Notification{
		ID:        fmt.Sprintf("test_%d", time.Now().Unix()),
		Type:      "test",
		Recipient: req.Phone,
		Content:   req.Message,
		Status:    "pending",
	}

	go sendSMSAsync(notification)

	return c.JSON(http.StatusOK, map[string]string{
		"message":         "SMS sending initiated",
		"notification_id": notification.ID,
	})
}

func getNotifications(c echo.Context) error {
	notifications := make([]*Notification, 0, len(sentNotifications))
	for _, notification := range sentNotifications {
		notifications = append(notifications, notification)
	}
	return c.JSON(http.StatusOK, notifications)
}

func getNotificationByID(c echo.Context) error {
	id := c.Param("id")
	notification, exists := sentNotifications[id]
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification not found"})
	}
	return c.JSON(http.StatusOK, notification)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func handleWebSocketCallbacks(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	callbackClientsMutex.Lock()
	callbackClients[ws] = true
	callbackClientsMutex.Unlock()

	defer func() {
		ws.Close()
		callbackClientsMutex.Lock()
		delete(callbackClients, ws)
		callbackClientsMutex.Unlock()
	}()

	log.Printf("New SMS Gateway callback connection established")

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket callback error: %v", err)
			}
			break
		}

		var callback map[string]interface{}
		if err := json.Unmarshal(message, &callback); err != nil {
			log.Printf("Error parsing callback message: %v", err)
			continue
		}

		log.Printf("Received SMS delivery callback: %v", callback)

		if notificationID, ok := callback["notification_id"].(string); ok {
			if notification, exists := sentNotifications[notificationID]; exists {
				if status, ok := callback["status"].(string); ok {
					notification.Status = status
				}
				if errorMsg, ok := callback["error"].(string); ok {
					notification.ErrorMsg = errorMsg
				}
				if status == "delivered" {
					notification.DeliveredAt = time.Now()
				}

				log.Printf("Updated notification %s status to %s",
					notificationID, notification.Status)
			}
		}
	}

	return nil
}

func broadcastNotificationStatus(notification *Notification) {
	callbackClientsMutex.Lock()
	defer callbackClientsMutex.Unlock()

	if len(callbackClients) == 0 {
		return
	}

	statusUpdate := map[string]interface{}{
		"event":           "status_update",
		"notification_id": notification.ID,
		"type":            notification.Type,
		"recipient":       notification.Recipient,
		"status":          notification.Status,
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	statusJSON, err := json.Marshal(statusUpdate)
	if err != nil {
		log.Printf("Error marshaling status update: %v", err)
		return
	}

	for client := range callbackClients {
		err := client.WriteMessage(websocket.TextMessage, statusJSON)
		if err != nil {
			log.Printf("Error sending to WebSocket client: %v", err)
			client.Close()
			delete(callbackClients, client)
		}
	}
}
