FROM golang:1.19-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o payment-service .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/payment-service /usr/local/bin/

ENV DB_HOST=postgres \
    DB_PORT=5432 \
    DB_USER=postgres \
    DB_PASSWORD=postgres \
    DB_NAME=payment_system \
    NATS_URL=nats://nats:4222 \
    AUTH_SERVICE_URL=http://auth-service:8081 \
    FRAUD_SERVICE_URL=fraud-service:50051 \
    PORT=8082

EXPOSE 8082

CMD ["payment-service"]