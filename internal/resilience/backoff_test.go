package resilience_test

import (
	"testing"
	"time"

	"camunda8-zeebe-go/internal/resilience"
)

func TestExponentialBackoff_CalculatesWithinBounds(t *testing.T) {
	cfg := resilience.BackoffConfig{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     2 * time.Second,
		Multiplier:      2.0,
		MaxRetries:      5,
	}

	bo := resilience.NewExponentialBackoff(cfg)

	// Attempt 0: max raw = 100ms. Jittered must be in [0, 100ms]
	for i := 0; i < 50; i++ {
		d := bo.CalculateBackoff(0)
		if d < 0 || d > 100*time.Millisecond {
			t.Fatalf("attempt 0 backoff %v out of range [0, 100ms]", d)
		}
	}

	// Attempt 1: max raw = 200ms. Jittered in [0, 200ms]
	for i := 0; i < 50; i++ {
		d := bo.CalculateBackoff(1)
		if d < 0 || d > 200*time.Millisecond {
			t.Fatalf("attempt 1 backoff %v out of range [0, 200ms]", d)
		}
	}

	// Attempt 10: raw would be huge, but clamped at MaxInterval (2s). Jittered in [0, 2s]
	for i := 0; i < 50; i++ {
		d := bo.CalculateBackoff(10)
		if d < 0 || d > 2*time.Second {
			t.Fatalf("attempt 10 backoff %v exceeded MaxInterval 2s", d)
		}
	}
}

func TestExponentialBackoff_DefaultConfig(t *testing.T) {
	bo := resilience.NewExponentialBackoff(resilience.DefaultBackoffConfig())
	if bo.MaxRetries() != 5 {
		t.Fatalf("expected default max retries 5, got %d", bo.MaxRetries())
	}
}
