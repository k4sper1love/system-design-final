package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	initNATS()
	defer natsConn.Close()

	subscribeToNATS()

	http.HandleFunc("/test-sms", func(w http.ResponseWriter, r *http.Request) {
		phone := r.URL.Query().Get("phone")
		message := r.URL.Query().Get("message")
		if phone == "" || message == "" {
			http.Error(w, "Missing phone or message", http.StatusBadRequest)
			return
		}

		if err := sendSMS(phone, message); err != nil {
			http.Error(w, fmt.Sprintf("Failed to send SMS: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("SMS sent successfully"))
	})

	log.Println("Starting Notification Service on :8083")
	log.Fatal(http.ListenAndServe(":8083", nil))
}
