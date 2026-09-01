# AGENT.md - Camunda 8 Zeebe Go Project Guidelines

This document outlines architectural principles, coding conventions, resilience patterns, and operational guidelines for AI agents working in this repository.

---

## 1. Project Overview & Architecture

This repository contains a production-grade **Camunda 8 (Zeebe) Go Worker and Workflow** application for an e-commerce order fulfillment lifecycle.

### Tech Stack
- **Language**: Go 1.23+
- **Orchestration Engine**: Camunda 8 (Zeebe Client Go `v8`)
- **Workflow Format**: BPMN 2.0 (`bpmn/order-fulfillment.bpmn`)
- **Containerization**: Multi-stage Dockerfile & Docker Compose

### Directory Structure
```
.
├── AGENT.md                       # Agent guidelines and architectural rules
├── Dockerfile                     # Multi-stage container definition
├── docker-compose.yml             # Local environment (Zeebe + Operate + Elasticsearch + Worker)
├── Makefile                       # Project development tasks
├── bpmn/
│   ├── order-fulfillment.bpmn          # Standard BPMN 2.0 workflow model
│   ├── order-risk-fulfillment.bpmn     # Complex BPMN 2.0 with DMN & parallel fork/join
│   └── order-risk-rules.dmn            # DMN 1.3 Decision Table for risk evaluation
├── cmd/
│   ├── worker/main.go                  # Main Zeebe job worker daemon
│   └── starter/main.go                 # Workflow/DMN deployer & instance initiator
└── internal/
    ├── config/config.go                # Environment variables configuration
    ├── model/order.go                  # Domain types and workflow payloads
    ├── resilience/
    │   ├── backoff.go                  # Exponential backoff with full jitter
    │   ├── circuitbreaker.go           # Resilient circuit breaker state machine
    │   ├── errors.go                   # Business (BPMN) vs Retriable error types
    │   └── middleware.go               # Resilient decorator for Zeebe worker handlers
    └── worker/
        ├── handler.go                  # Worker registration & lifecycle manager
        ├── validate_order.go           # Task handler: 'validate-order'
        ├── process_payment.go          # Task handler: 'process-payment' (CB protected)
        ├── ship_order.go               # Task handler: 'ship-order'
        ├── notify_failure.go           # Task handler: 'notify-customer-failure'
        ├── apply_discount.go           # Task handler: 'apply-discount' (DMN output consumer)
        └── reserve_inventory.go        # Task handler: 'reserve-inventory' (inventory stock checks)
```

---

## 2. Core Resilience Guidelines

Resilience is a primary design requirement in this repository. Every job worker **must** be wrapped using the resilience middleware.

### A. Error Taxonomy
1. **Business Errors (`*resilience.BusinessError`)**:
   - Non-retriable domain errors (e.g., `ERR_PAYMENT_DECLINED`, `ERR_INVALID_ORDER`).
   - Action: Dispatch `client.NewThrowErrorCommand()` so Zeebe catches it with a **BPMN Boundary Error Event**.
2. **Retriable / Technical Errors (`*resilience.RetriableError` or generic errors)**:
   - Transient failures (network timeout, downstream service degradation).
   - Action: Dispatch `client.NewFailJobCommand()` with decremented retries and jittered backoff.
   - When retries reach `0`, Zeebe automatically raises an **Incident** in Camunda Operate for operator review.

### B. Circuit Breaker (`internal/resilience/circuitbreaker.go`)
- Apply to any external downstream API call (e.g., payment gateways, shipping providers).
- States: `CLOSED` -> `OPEN` (after `FailureThreshold`) -> `HALF-OPEN` (after `RecoveryTimeout`) -> `CLOSED` (after `SuccessThreshold`).
- Rejects calls immediately with `ErrCircuitOpen` during outages, preventing cascading thread starvation.

### C. Exponential Backoff with Full Jitter (`internal/resilience/backoff.go`)
- Formula: `sleep = rand() * min(MaxInterval, InitialInterval * (Multiplier ^ attempt))`
- Prevents thundering herd on downstream recoveries.

### D. Panic Recovery
- All worker routines must recover from panics, log the stack trace, and fail the job gracefully rather than crashing the daemon.

---

## 3. Testing Policy & Instructions

> [!IMPORTANT]
> **Strict Testing Rule**:
> - **ONLY run UNIT TESTS (`go test ./...`)** during routine changes and validation.
> - **DO NOT run End-to-End (E2E) tests or boot live broker containers** unless explicitly asked by the user.

### Automated Unit Testing Commands
```bash
# Run all unit tests with race detection
go test -v -race ./...

# Run resilience package tests
go test -v -race ./internal/resilience/...

# Run worker package unit tests
go test -v -race ./internal/worker/...
```

---

## 4. Local Build & Run Instructions (When Requested)

### Build Binaries
```bash
go build -o bin/worker ./cmd/worker
go build -o bin/starter ./cmd/starter
```

### Run with Docker Compose
```bash
docker compose up --build -d
```

### Deploy BPMN and Start Sample Workflow
```bash
# Deploy diagram
go run ./cmd/starter -deploy

# Start success workflow
go run ./cmd/starter -start -scenario success

# Start payment decline simulation (triggers BPMN error boundary event)
go run ./cmd/starter -start -scenario decline
```
