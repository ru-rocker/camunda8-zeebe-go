package resilience

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// State represents the state of the circuit breaker
type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateHalfOpen:
		return "HALF-OPEN"
	case StateOpen:
		return "OPEN"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

var (
	// ErrCircuitOpen is returned when the circuit breaker rejects execution
	ErrCircuitOpen = errors.New("circuit breaker is open; requests are rejected")
)

// CircuitBreakerConfig holds settings for the circuit breaker
type CircuitBreakerConfig struct {
	Name             string
	FailureThreshold int           // consecutive failures to trip open
	RecoveryTimeout  time.Duration // time in OPEN before attempting HALF-OPEN
	SuccessThreshold int           // consecutive successes in HALF-OPEN to close
}

// DefaultCircuitBreakerConfig returns standard default settings
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:             name,
		FailureThreshold: 3,
		RecoveryTimeout:  10 * time.Second,
		SuccessThreshold: 2,
	}
}

// CircuitBreaker wraps execution and prevents repeated calls to failing downstream services
type CircuitBreaker struct {
	config            CircuitBreakerConfig
	state             State
	consecutiveFails  int
	consecutivePasses int
	lastStateChange   time.Time
	mu                sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.RecoveryTimeout <= 0 {
		cfg.RecoveryTimeout = 10 * time.Second
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}

	return &CircuitBreaker{
		config:          cfg,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// State returns the current circuit breaker state, evaluating recovery timeout if OPEN
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.evaluateState()
	return cb.state
}

// evaluateState must be called with lock held
func (cb *CircuitBreaker) evaluateState() {
	if cb.state == StateOpen && time.Since(cb.lastStateChange) >= cb.config.RecoveryTimeout {
		cb.state = StateHalfOpen
		cb.consecutivePasses = 0
		cb.consecutiveFails = 0
		cb.lastStateChange = time.Now()
	}
}

// Execute runs the provided action function if allowed by the circuit state
func (cb *CircuitBreaker) Execute(action func() error) error {
	cb.mu.Lock()
	cb.evaluateState()

	if cb.state == StateOpen {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}
	cb.mu.Unlock()

	// Execute action outside lock
	err := action()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// onSuccess updates internal state upon successful call (lock held)
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		cb.consecutiveFails = 0
	case StateHalfOpen:
		cb.consecutivePasses++
		if cb.consecutivePasses >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.consecutiveFails = 0
			cb.consecutivePasses = 0
			cb.lastStateChange = time.Now()
		}
	}
}

// onFailure updates internal state upon failed call (lock held)
func (cb *CircuitBreaker) onFailure() {
	switch cb.state {
	case StateClosed:
		cb.consecutiveFails++
		if cb.consecutiveFails >= cb.config.FailureThreshold {
			cb.state = StateOpen
			cb.consecutivePasses = 0
			cb.lastStateChange = time.Now()
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.consecutivePasses = 0
		cb.consecutiveFails = 0
		cb.lastStateChange = time.Now()
	}
}

// Reset manually sets circuit back to Closed
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.consecutiveFails = 0
	cb.consecutivePasses = 0
	cb.lastStateChange = time.Now()
}
