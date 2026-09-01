package resilience_test

import (
	"errors"
	"strings"
	"testing"

	"camunda8-zeebe-go/internal/resilience"
)

func TestBusinessError(t *testing.T) {
	err := resilience.NewBusinessError(resilience.ErrPaymentDeclined, "insufficient funds")
	if err.Code != resilience.ErrPaymentDeclined {
		t.Fatalf("expected code %s, got %s", resilience.ErrPaymentDeclined, err.Code)
	}
	if !strings.Contains(err.Error(), "insufficient funds") {
		t.Fatalf("expected error message to contain text, got: %s", err.Error())
	}
}

func TestRetriableError(t *testing.T) {
	rootCause := errors.New("connection reset by peer")
	err := resilience.NewRetriableError("gateway unreachable", rootCause)

	if !errors.Is(err, rootCause) {
		t.Fatalf("expected errors.Is to match rootCause via Unwrap()")
	}
	if !strings.Contains(err.Error(), "connection reset by peer") {
		t.Fatalf("expected formatted error to contain root cause")
	}
}
