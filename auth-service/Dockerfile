FROM golang:1.19-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o auth-service .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/auth-service /usr/local/bin/

ENV DB_HOST=postgres \
    DB_PORT=5432 \
    DB_USER=postgres \
    DB_PASSWORD=postgres \
    DB_NAME=payment_system \
    JWT_SECRET=your_jwt_secret_key_here \
    PORT=8081

EXPOSE 8081

CMD ["auth-service"]