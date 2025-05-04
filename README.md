# Online payment system with fraud monitoring
The online payment system is designed to provide a secure, scalable, and efficient platform for processing payments while integrating real-time fraud detection.

## Main Features of the Implemented System
- User registration and authentication using phone number/password with JWT token generation.
- Secure password storage using bcrypt hashing.
- Payment processing, including balance top-up and transaction handling.
- Real-time fraud monitoring for transactions using rules stored in Redis.
- Optimistic Locking to prevent race conditions during balance updates.
- Notification service for sending SMS alerts about transaction statuses using Twilio.
- API Gateway for routing requests, enforcing JWT-based authentication, and implementing Rate Limiting using the Token Bucket algorithm.
- PostgreSQL for storing user data, balances, and transaction history.
- Redis for caching fraud monitoring rules and counters.
- NATS message broker for asynchronous communication between microservices.
- Docker Compose setup for PostgreSQL, Redis, and NATS.

## Prerequisites
- Go (1.20 or later)
- Docker and Docker Compose
- PostgreSQL
- Redis
- NATS 
- Twilio Account

## Project Structure
The project is organized into separate folders for each service:
- auth-service/
- payment-service/
- fraud-service/
- notification-service/
- api-gateway/
- docker-compose.yml

## Technologies Used
- **Backend** : Go + Echo Framework
- **Database** : PostgreSQL 
- **Caching** : Redis 
- **Messaging** : NATS
- **Authentication** : JWT 
- **Notifications** : Twilio
- **Rate** **Limiting** : Token Bucket Algorithm
- **Load** **Balancing** : Nginx (API Gateway)
- **Containerization** : Docker

## Environment Setup
1. Start Docker Containers:
```bash
docker-compose up -d
```
2. Running the Services. Each service can be run independently using go run. Ensure Docker containers are running before starting the services.
```bash
go run auth-service/main.go
```
```bash
go run payment-service/main.go
```
```bash
go run fraud-service/main.go
```
```bash
go run notification-service/main.go
```
```bash
go run api-gateway/main.go
```

