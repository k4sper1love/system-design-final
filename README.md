# Payment System - Microservice Architecture

## Overview

A complete payment processing system built with microservices architecture, allowing for:
- User authentication with phone numbers
- Balance management and fund transfers
- Real-time fraud detection
- Transaction notifications

## System Components

- **API Gateway** (Nginx): Routes requests to appropriate services
- **Auth Service**: Handles user registration, authentication, and JWT management
- **Payment Service**: Manages user balances and payment transactions
- **Fraud Detection Service**: Real-time fraud monitoring using rules-based analysis
- **Notification Service**: Sends transaction notifications via SMS
- **Supporting Infrastructure**:
  - NATS: Event messaging between services
  - PostgreSQL: Persistent data storage
  - Redis: Caching and fraud detection rules

## Quick Start

```bash
# Clone repository and start all services
docker-compose up

# Or rebuild containers before starting
docker-compose up --build
```

## Service Endpoints

After deployment, services are available at:

- **API Gateway**: http://localhost:80 - Main entry point for all services
- **Auth Service**: http://localhost:8081 - Authentication APIs
- **Payment Service**: http://localhost:8082 - Transaction and balance APIs
- **Fraud Service**: http://localhost:8083 - Fraud detection APIs
- **Notification Service**: http://localhost:8084 - Notification APIs
- **PgAdmin**: http://localhost:5050 - PostgreSQL admin interface
- **Redis Commander**: http://localhost:8085 - Redis data browser

## API Documentation

A complete Postman collection is included in the repository (`postman_collection.json`), containing examples of all API endpoints.

### API Categories:
- **Authentication**: Registration, login, token refresh
- **Payment Operations**: Balance top-up, transfers, transaction history
- **Fraud Management**: Create/manage fraud detection rules
- **Notifications**: SMS delivery management

## Development

### Project Structure

```
├── api-gateway/        # API Gateway service
├── auth-service/       # Authentication service
├── fraud-service/      # Fraud detection service
├── notification-service/ # Notification service
├── payment-service/    # Payment service
├── shared/             # Shared proto files and generated code
├── docker-compose.yml  # Docker Compose configuration
├── nginx.conf          # Nginx configuration
├── postman_collection.json # API documentation
└── README.md           # This file
```

### Running Individual Services

For development, you can run services individually:

```bash
# Start auth service (repeat pattern for other services)
cd auth-service && go run .
```

## Infrastructure Details

### Database Schema

**PostgreSQL**:
- `users`: User accounts and credentials
- `balances`: Account balances with optimistic locking
- `transactions`: Transaction records
- `refresh_tokens`: JWT refresh tokens

**Redis**:
- Fraud monitoring rules
- Suspicious transaction cache

### Administrative Tools

- **PgAdmin**: PostgreSQL database administration
  - URL: http://localhost:5050
  - Default login: admin@example.com / admin
  
- **Redis Commander**: Redis inspection and management
  - URL: http://localhost:8085

## Technical Features

- **Authentication**: JWT-based with access/refresh token mechanism
- **Communication**: 
  - REST APIs between services and clients
  - gRPC for fraud detection (high performance)
  - NATS for event-driven notifications
- **Security**: 
  - Phone validation
  - bcrypt password hashing
  - Token blacklisting
- **Data Integrity**: Optimistic locking for transactions
