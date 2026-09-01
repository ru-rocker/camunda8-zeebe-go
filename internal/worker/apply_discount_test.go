package worker_test

import (
	"context"
	"testing"

	"camunda8-zeebe-go/internal/worker"
)

func TestApplyDiscount_WithDMNResult(t *testing.T) {
	job := NewMockJob(5001, "apply-discount", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":     "ORD-5001",
			"totalAmount": 200.0,
		},
		"decisionResult": map[string]interface{}{
			"discountPercent": 15.0,
			"riskLevel":       "LOW",
		},
	})

	result, err := worker.ApplyDiscountHandler(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["discountPercent"] != 15.0 {
		t.Fatalf("expected discountPercent 15.0, got %v", result["discountPercent"])
	}

	if result["discountedAmount"] != 170.0 {
		t.Fatalf("expected discountedAmount 170.0 ($200 - 15%%), got %v", result["discountedAmount"])
	}
}

func TestApplyDiscount_ZeroDiscount(t *testing.T) {
	job := NewMockJob(5002, "apply-discount", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":     "ORD-5002",
			"totalAmount": 100.0,
		},
		"decisionResult": map[string]interface{}{
			"discountPercent": 0.0,
		},
	})

	result, err := worker.ApplyDiscountHandler(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["discountedAmount"] != 100.0 {
		t.Fatalf("expected discountedAmount 100.0, got %v", result["discountedAmount"])
	}
}
