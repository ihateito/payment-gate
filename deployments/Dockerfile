# Multi-stage Dockerfile for PaymentGate microservices
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Cache dependency layers
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build target microservice binary
ARG SERVICE=api-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/app ./cmd/${SERVICE}

# Lightweight minimal runtime container
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/app /app/app
COPY --from=builder /app/web /app/web

EXPOSE 8080

ENTRYPOINT ["/app/app"]
