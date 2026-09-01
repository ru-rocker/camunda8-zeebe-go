# Camunda 8 (Zeebe) Resilient Go Worker

A production-ready Golang worker application for **Camunda 8 (Zeebe)** workflow orchestration featuring advanced resilience patterns, BPMN 2.0 diagram definitions, containerization, and unit tests.

---

## Features

- **Camunda 8 Zeebe Client (v8)**: Modern, gRPC-based worker and workflow starter.
- **Resilience Patterns**:
  - **Circuit Breaker**: Prevents cascading failures when downstream payment/shipping services degrade.
  - **Exponential Backoff with Full Jitter**: Prevents thundering herds during retries.
  - **Panic Recovery Middleware**: Recovers from unexpected runtime panics and isolates job failures.
  - **Error Taxonomy**: Distinguishes between technical retries (`FailJob`) and business errors (`ThrowError` to BPMN boundary events).
- **BPMN 2.0 Model (`bpmn/order-fulfillment.bpmn`)**: Complete diagram with service tasks, sequence flows, boundary error event catch, compensation/notification flows, and visual DI coordinates.
- **Containerization**: Multi-stage `Dockerfile` and `docker-compose.yml` (Zeebe Broker + Operate + Elasticsearch + Worker).
- **Unit Testing**: 100% isolated unit tests with mock entities.

---

## Workflow Architecture (BPMN)

```
[Start: Order Placed]
         │
         ▼
[Service Task: validate-order]
         │
         ▼
<Gateway: Review Needed?>
      ├─────────── [totalAmount >= 1000] ───────────► [User Task: Manual Order Review]
      │                                                               │
      │                                                               ▼
      │                                                    <Gateway: Approved?>
      │                                                       ├────── (Yes) ──┐
      │                                                       │               │
      │                                                       ▼ (No)          │
      │                                            [End: Order Rejected]      │
      ▼ (< 1000 / Auto)                                                       │
[Service Task: process-payment] ◄─────────────────────────────────────────────┘
         │
         ├───(Boundary Error: ERR_PAYMENT_DECLINED)───► [Service Task: notify-customer-failure]
         │                                                                │
         ▼                                                                ▼
[Service Task: ship-order]                                       [End: Order Cancelled]
         │
         ▼
[End: Order Completed]
```

---

## Quick Start

### 1. Run Unit Tests
```bash
make test-race
```

### 2. Build Binaries
```bash
make build
```

### 3. Start Zeebe Stack (Optional)
```bash
make docker-up
```

### 4. Deploy BPMN & Trigger Workflows
```bash
# Deploy diagram
go run ./cmd/starter -deploy

# Start standard success workflow instance (< $1000 - auto-approved)
go run ./cmd/starter -start -scenario success

# Start high-value order workflow instance (>= $1000 - pauses at Manual Order Review User Task)
go run ./cmd/starter -start -scenario review

# Approve pending Human Review User Tasks (advances workflow to payment & shipping)
go run ./cmd/starter -approve

# Or Reject pending Human Review User Tasks (terminates workflow at Order Rejected)
go run ./cmd/starter -reject

# Start payment decline workflow instance (triggers BPMN error boundary event)
go run ./cmd/starter -start -scenario decline
```
