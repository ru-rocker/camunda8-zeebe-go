package worker

import (
	"log"
	"sync"

	"camunda8-zeebe-go/internal/config"
	"camunda8-zeebe-go/internal/resilience"
	"github.com/camunda/zeebe/clients/go/v8/pkg/worker"
	"github.com/camunda/zeebe/clients/go/v8/pkg/zbc"
)

// JobWorkerSubscription represents an active Zeebe job worker subscription
type JobWorkerSubscription struct {
	JobType string
	Worker  worker.JobWorker
}

// Manager manages lifecycle and resilient registration of Zeebe job workers
type Manager struct {
	client         zbc.Client
	cfg            *config.Config
	workers        []JobWorkerSubscription
	paymentCB      *resilience.CircuitBreaker
	backoff        *resilience.ExponentialBackoff
	mu             sync.Mutex
	paymentService PaymentService
}

// NewManager creates a new worker Manager
func NewManager(client zbc.Client, cfg *config.Config, paymentService PaymentService) *Manager {
	if paymentService == nil {
		paymentService = &DefaultPaymentService{}
	}

	cbConfig := resilience.CircuitBreakerConfig{
		Name:             "payment-gateway-cb",
		FailureThreshold: cfg.CBFailureThreshold,
		RecoveryTimeout:  cfg.CBRecoveryTimeout,
		SuccessThreshold: 2,
	}

	backoffConfig := resilience.BackoffConfig{
		InitialInterval: cfg.BackoffInitialInterval,
		MaxInterval:     cfg.BackoffMaxInterval,
		Multiplier:      2.0,
		MaxRetries:      cfg.BackoffMaxRetries,
	}

	return &Manager{
		client:         client,
		cfg:            cfg,
		workers:        make([]JobWorkerSubscription, 0),
		paymentCB:      resilience.NewCircuitBreaker(cbConfig),
		backoff:        resilience.NewExponentialBackoff(backoffConfig),
		paymentService: paymentService,
	}
}

// RegisterWorkers subscribes all workflow job workers with resilience middleware
func (m *Manager) RegisterWorkers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	defaultCfg := resilience.ResilienceMiddlewareConfig{
		Timeout:        m.cfg.WorkerTimeout,
		Backoff:        m.backoff,
		InitialRetries: int32(m.cfg.BackoffMaxRetries),
	}

	paymentMiddlewareCfg := resilience.ResilienceMiddlewareConfig{
		Timeout:        m.cfg.WorkerTimeout,
		Backoff:        m.backoff,
		CircuitBreaker: m.paymentCB,
		InitialRetries: int32(m.cfg.BackoffMaxRetries),
	}

	// 1. validate-order worker
	m.startWorker("validate-order", resilience.WrapResilientHandler(ValidateOrderHandler, defaultCfg))

	// 2. process-payment worker (protected with Circuit Breaker)
	m.startWorker("process-payment", resilience.WrapResilientHandler(NewProcessPaymentHandler(m.paymentService), paymentMiddlewareCfg))

	// 3. ship-order worker
	m.startWorker("ship-order", resilience.WrapResilientHandler(ShipOrderHandler, defaultCfg))

	// 4. notify-customer-failure worker
	m.startWorker("notify-customer-failure", resilience.WrapResilientHandler(NotifyCustomerFailureHandler, defaultCfg))

	// 5. apply-discount worker (for DMN workflow)
	m.startWorker("apply-discount", resilience.WrapResilientHandler(ApplyDiscountHandler, defaultCfg))

	// 6. reserve-inventory worker (for DMN workflow)
	m.startWorker("reserve-inventory", resilience.WrapResilientHandler(ReserveInventoryHandler, defaultCfg))
}

func (m *Manager) startWorker(jobType string, handler worker.JobHandler) {
	w := m.client.NewJobWorker().
		JobType(jobType).
		Handler(handler).
		Name(m.cfg.WorkerName + "-" + jobType).
		Timeout(m.cfg.WorkerTimeout).
		MaxJobsActive(m.cfg.WorkerMaxJobsActive).
		Concurrency(m.cfg.WorkerConcurrency).
		Open()

	m.workers = append(m.workers, JobWorkerSubscription{
		JobType: jobType,
		Worker:  w,
	})

	log.Printf("[Worker Manager] Successfully registered resilient worker for job type: '%s'", jobType)
}

// Close gracefully closes all worker subscriptions
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("[Worker Manager] Initiating graceful shutdown for %d active job workers...", len(m.workers))
	for _, sub := range m.workers {
		log.Printf("[Worker Manager] Closing worker for job type '%s'...", sub.JobType)
		sub.Worker.Close()
		sub.Worker.AwaitClose()
	}
	m.workers = nil
	log.Println("[Worker Manager] All workers closed gracefully.")
}
