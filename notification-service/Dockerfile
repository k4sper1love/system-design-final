FROM golang:1.19-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o notification-service .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/notification-service /usr/local/bin/

ENV NATS_URL=nats://nats:4222 \
    PORT=8084 \
    TWILIO_ACCOUNT_SID=your_twilio_account_sid \
    TWILIO_AUTH_TOKEN=your_twilio_auth_token \
    TWILIO_PHONE_NUMBER=your_twilio_phone_number \
    SMS_GATEWAY_URL=ws://localhost:8084/ws/sms

EXPOSE 8084

CMD ["notification-service"]