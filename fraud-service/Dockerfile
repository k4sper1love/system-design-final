FROM golang:1.19-alpine AS builder

RUN apk add --no-cache git gcc musl-dev protoc

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fraud-service .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/fraud-service /usr/local/bin/

ENV REDIS_URL=redis://redis:6379 \
    PORT=8083 \
    GRPC_PORT=50051

EXPOSE 8083 50051

CMD ["fraud-service"]