package worker_test

import (
	"context"
	"strings"
	"testing"

	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/worker"
)

func TestShipOrder_Success(t *testing.T) {
	job := NewMockJob(3001, "ship-order", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":    "ORD-3001",
			"customerId": "cust_123",
		},
	})

	result, err := worker.ShipOrderHandler(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["orderStatus"] != string(model.OrderStatusShipped) {
		t.Fatalf("expected orderStatus SHIPPED, got %v", result["orderStatus"])
	}

	tracking, ok := result["trackingNumber"].(string)
	if !ok || !strings.HasPrefix(tracking, "TRK-") {
		t.Fatalf("expected valid tracking number starting with TRK-, got %v", tracking)
	}
}

func TestNotifyCustomerFailure_Success(t *testing.T) {
	job := NewMockJob(4001, "notify-customer-failure", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":    "ORD-4001",
			"customerId": "cust_fail",
		},
	})

	result, err := worker.NotifyCustomerFailureHandler(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["orderStatus"] != string(model.OrderStatusFailed) {
		t.Fatalf("expected orderStatus FAILED, got %v", result["orderStatus"])
	}
}
