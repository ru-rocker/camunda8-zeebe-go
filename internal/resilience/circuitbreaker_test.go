package resilience_test

import (
	"errors"
	"testing"
	"time"

	"camunda8-zeebe-go/internal/resilience"
)

func TestCircuitBreaker_InitialStateIsClosed(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:             "test-cb",
		FailureThreshold: 2,
		RecoveryTimeout:  50 * time.Millisecond,
		SuccessThreshold: 2,
	})

	if cb.State() != resilience.StateClosed {
		t.Fatalf("expected state CLOSED, got %s", cb.State())
	}
}

func TestCircuitBreaker_TripsToOpenAfterThresholdFailures(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:             "test-cb",
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
		SuccessThreshold: 2,
	})

	testErr := errors.New("service down")

	// 1st failure
	_ = cb.Execute(func() error { return testErr })
	if cb.State() != resilience.StateClosed {
		t.Fatalf("expected CLOSED after 1 failure, got %s", cb.State())
	}

	// 2nd failure
	_ = cb.Execute(func() error { return testErr })
	if cb.State() != resilience.StateClosed {
		t.Fatalf("expected CLOSED after 2 failures, got %s", cb.State())
	}

	// 3rd failure -> should trip to OPEN
	_ = cb.Execute(func() error { return testErr })
	if cb.State() != resilience.StateOpen {
		t.Fatalf("expected OPEN after 3 failures, got %s", cb.State())
	}

	// Immediate next call should fail fast with ErrCircuitOpen without executing the func
	executed := false
	err := cb.Execute(func() error {
		executed = true
		return nil
	})

	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if executed {
		t.Fatalf("expected action NOT to execute when circuit is OPEN")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpenAndRecovers(t *testing.T) {
	recoveryTimeout := 40 * time.Millisecond
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:             "test-cb",
		FailureThreshold: 2,
		RecoveryTimeout:  recoveryTimeout,
		SuccessThreshold: 2,
	})

	testErr := errors.New("transient issue")

	// Trip to OPEN
	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })

	if cb.State() != resilience.StateOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	// Wait for recovery timeout
	time.Sleep(recoveryTimeout + 10*time.Millisecond)

	// State check should now report HALF-OPEN
	if cb.State() != resilience.StateHalfOpen {
		t.Fatalf("expected HALF-OPEN after recovery timeout, got %s", cb.State())
	}

	// 1st success in HALF-OPEN
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.State() != resilience.StateHalfOpen {
		t.Fatalf("expected still HALF-OPEN after 1 success, got %s", cb.State())
	}

	// 2nd success in HALF-OPEN -> should close
	err = cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.State() != resilience.StateClosed {
		t.Fatalf("expected CLOSED after reaching success threshold, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReOpensCircuit(t *testing.T) {
	recoveryTimeout := 40 * time.Millisecond
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:             "test-cb",
		FailureThreshold: 1,
		RecoveryTimeout:  recoveryTimeout,
		SuccessThreshold: 2,
	})

	testErr := errors.New("outage")

	// Trip to OPEN
	_ = cb.Execute(func() error { return testErr })
	if cb.State() != resilience.StateOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	// Wait for recovery timeout
	time.Sleep(recoveryTimeout + 10*time.Millisecond)

	// Fail in HALF-OPEN
	_ = cb.Execute(func() error { return testErr })
	if cb.State() != resilience.StateOpen {
		t.Fatalf("expected OPEN after failure in HALF-OPEN, got %s", cb.State())
	}
}

func TestCircuitBreaker_ManualReset(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:             "test-cb",
		FailureThreshold: 1,
		RecoveryTimeout:  10 * time.Second,
		SuccessThreshold: 1,
	})

	_ = cb.Execute(func() error { return errors.New("err") })
	if cb.State() != resilience.StateOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	cb.Reset()
	if cb.State() != resilience.StateClosed {
		t.Fatalf("expected CLOSED after Reset(), got %s", cb.State())
	}
}
