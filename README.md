# Camunda 8 (Zeebe) Resilient Go Worker

A production-ready Golang worker application for **Camunda 8 (Zeebe)** workflow orchestration featuring advanced resilience patterns, DMN 1.3 Decision Tables, complex BPMN 2.0 diagrams (Parallel Gateways, Business Rule Tasks, User Tasks, Boundary Events), containerization, and unit tests.

---

## Key Features

- **Camunda 8 Zeebe Client (v8)**: Modern, gRPC-based worker daemon and workflow starter.
- **DMN 1.3 Decision Tables**: Native Business Rule Task execution evaluating customer tier, order amount, and fraud score.
- **Complex BPMN Patterns**:
  - **Business Rule Task (`<bpmn:businessRuleTask>`)**: Evaluates risk policies using embedded DMN engine.
  - **Parallel Gateway (`<bpmn:parallelGateway>`)**: Fork and Join concurrent execution for discounting and inventory reservation.
  - **Human Review (`<bpmn:userTask>`)**: User task assignment for high-risk / high-value orders.
  - **Boundary Error Events (`<bpmn:boundaryEvent>`)**: Catches `ERR_PAYMENT_DECLINED` and `ERR_OUT_OF_STOCK` for graceful compensation.
- **Resilience Patterns**:
  - **Circuit Breaker**: Prevents cascading failures when downstream payment/shipping services degrade.
  - **Exponential Backoff with Full Jitter**: Avoids thundering herds during worker retries.
  - **Panic Recovery Middleware**: Recovers from runtime panics and isolates job failures.
  - **Error Taxonomy**: Distinguishes between technical retries (`FailJob`) and business errors (`ThrowError` to BPMN boundary events).
- **Containerization**: Multi-stage `Dockerfile` and `docker-compose.yml` (Zeebe Broker + Operate + Elasticsearch + Worker).
- **Unit Testing**: 100% isolated unit tests with mock entities (`go test -v -race ./...`).

---

## Workflows & Architecture

### 1. Complex Workflow with DMN: Order Risk & Fulfillment (`order-risk-fulfillment-process`)
* **BPMN**: [bpmn/order-risk-fulfillment.bpmn](file:///Users/ricky/Documents/workspaces/workspace-poc/camunda8-zeebe-go/bpmn/order-risk-fulfillment.bpmn)
* **DMN Decision Table**: [bpmn/order-risk-rules.dmn](file:///Users/ricky/Documents/workspaces/workspace-poc/camunda8-zeebe-go/bpmn/order-risk-rules.dmn)

```
[Start Event: Order Submitted]
      │
      ▼
[Business Rule Task: Evaluate Risk & Discount (DMN)]
      │
      ▼
<Gateway: Risk Level?>
   ├──── (CRITICAL Fraud >= 80) ──────────────────────────────────────────────────► [End: Order Rejected (Fraud)]
   ├──── (HIGH Risk / Requires Manager) ──► [User Task: Manager Risk Review]
   │                                                    │
   │                                                    ▼
   │                                         <Gateway: Manager Approved?>
   │                                            ├──── (No) ───────────────────────► [End: Order Rejected by Manager]
   ▼ (LOW / MEDIUM / Approved)                  ▼ (Yes)
═════════════════════════[ Parallel Fork (AND) ]═════════════════════════
      │                                                     │
      ▼                                                     ▼
[Service Task: apply-discount]                [Service Task: reserve-inventory]
      │                                          (Boundary Error: ERR_OUT_OF_STOCK)
      │                                                     │
═════════════════════════[ Parallel Join (AND) ]═════════════════════════
      │
      ▼
[Service Task: process-payment] ───(Boundary Error: ERR_PAYMENT_DECLINED)──► [Service Task: notify-customer-failure]
      │                                                                                  │
      ▼                                                                                  ▼
[Service Task: ship-order]                                                     [End: Order Cancelled]
      │
      ▼
[End: Order Completed]
```

#### DMN Decision Table Logic (`Decision_CalculateRiskAndDiscount`):
| Customer Tier | Order Amount | Fraud Score | Output: Risk Level | Output: Discount | Output: Manager Approval |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Any | Any | $\ge 80$ | **`CRITICAL`** | 0% | No (Auto-Rejected) |
| Any | $\ge \$5,000$ | $\ge 40$ | **`HIGH`** | 5% | **Yes (User Task)** |
| **`PLATINUM`** | Any | $< 40$ | **`LOW`** | **20%** | No (Auto-Approved) |
| **`GOLD`** | Any | $< 40$ | **`LOW`** | **15%** | No (Auto-Approved) |
| **`SILVER`** | Any | $< 40$ | **`MEDIUM`** | **10%** | No (Auto-Approved) |
| Default | Any | Any | **`MEDIUM`** | 0% | No (Auto-Approved) |

---

### 2. Standard Workflow with Human Review (`order-fulfillment-process`)
* **BPMN**: [bpmn/order-fulfillment.bpmn](file:///Users/ricky/Documents/workspaces/workspace-poc/camunda8-zeebe-go/bpmn/order-fulfillment.bpmn)

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

### 1. Run Unit Tests (Strict Unit Test Policy)
```bash
make test-race
```

### 2. Build Binaries
```bash
make build
```

### 3. Start Local Zeebe Stack
```bash
make docker-up
```
* **Camunda Operate Web UI**: [http://localhost:8081](http://localhost:8081) (Credentials: `demo` / `demo`)
* **Camunda Tasklist Web UI**: [http://localhost:8082](http://localhost:8082) (Credentials: `demo` / `demo`)

### 4. Deploy All BPMN Workflows & DMN Tables
```bash
make deploy-all
```

---

## Querying User Tasks (Production Tasklist API vs Broker Polling)

### A. Production Read Model Search (Zero Broker Locks)
Queries the Camunda Tasklist REST API (`POST /v1/tasks/search`) backed by Elasticsearch index:
```bash
# Query all pending CREATED tasks
make tasklist-query
# or: go run ./cmd/starter -tasklist-query

# Query tasks assigned to a specific user (e.g. manager_demo)
make tasklist-query-manager
# or: go run ./cmd/starter -tasklist-query -user manager_demo -state CREATED
```

### B. Broker Polling (Zeebe Job Worker Prototype)
```bash
# Polls active userTask jobs from Zeebe engine queue
make list-tasks
# or: go run ./cmd/starter -list-tasks -user manager_demo
```

---

## Running Test Scenarios

### A. Complex DMN Workflow Scenarios (`order-risk-fulfillment-process`)

1. **Platinum VIP Customer (Low Risk + 20% DMN Discount + Parallel Processing)**:
   ```bash
   make start-risk-platinum
   ```
2. **Critical Fraud Detection (Auto-Rejected by DMN without Human Intervention)**:
   ```bash
   make start-risk-fraud
   ```
3. **High-Value Order with Moderate Risk (DMN routes to Manager Risk Review User Task)**:
   ```bash
   make start-risk-high
   # Then approve via:
   make approve-review
   # Or reject via:
   make reject-review
   ```
4. **Out-of-Stock Simulation (Triggers Inventory Boundary Error Event)**:
   ```bash
   make start-risk-stock
   ```

---

### B. Standard Workflow Scenarios (`order-fulfillment-process`)

1. **Standard Success Order (< $1,000 auto-approved)**:
   ```bash
   make start-instance
   ```
2. **High-Value Order (>= $1,000 pauses at User Task)**:
   ```bash
   make start-review
   # Then approve or reject:
   make approve-review
   ```
3. **Payment Decline (Triggers Payment Boundary Error Event)**:
   ```bash
   make start-decline
   ```

---

## Stopping the Environment
```bash
make docker-down
```
