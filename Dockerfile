# ==========================================
# Multi-Stage Dockerfile for Camunda 8 Go Worker
# ==========================================

# Step 1: Build Stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Cache go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/starter ./cmd/starter

# Step 2: Minimal Runtime Stage
FROM alpine:3.20

WORKDIR /app

# Add security certificates & unprivileged user
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /bin/worker /app/worker
COPY --from=builder /bin/starter /app/starter
COPY --from=builder /app/bpmn /app/bpmn

USER appuser

ENV ZEEBE_ADDRESS=zeebe:26500 \
    ZEEBE_INSECURE_CONNECTION=true

ENTRYPOINT ["/app/worker"]
