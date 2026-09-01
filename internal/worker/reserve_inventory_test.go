package worker_test

import (
	"context"
	"errors"
	"testing"

	"camunda8-zeebe-go/internal/resilience"
	"camunda8-zeebe-go/internal/worker"
)

func TestReserveInventory_Success(t *testing.T) {
	job := NewMockJob(6001, "reserve-inventory", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId": "ORD-6001",
			"items": []map[string]interface{}{
				{"productId": "PROD-SERVER-1", "quantity": 2},
			},
		},
	})

	result, err := worker.ReserveInventoryHandler(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["inventoryReserved"] != true {
		t.Fatalf("expected inventoryReserved to be true, got %v", result["inventoryReserved"])
	}
}

func TestReserveInventory_OutOfStock_TriggersBPMNError(t *testing.T) {
	job := NewMockJob(6002, "reserve-inventory", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId": "ORD-6002",
			"items": []map[string]interface{}{
				{"productId": "PROD-OUT_OF_STOCK-99", "quantity": 1},
			},
		},
	})

	_, err := worker.ReserveInventoryHandler(context.Background(), job)
	if err == nil {
		t.Fatalf("expected OutOfStock error, got nil")
	}

	var bizErr *resilience.BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected BusinessError, got %T: %v", err, err)
	}

	if bizErr.Code != resilience.ErrOutOfStock {
		t.Fatalf("expected error code %s, got %s", resilience.ErrOutOfStock, bizErr.Code)
	}
}
