package worker_test

import (
	"context"
	"errors"
	"testing"

	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/resilience"
	"camunda8-zeebe-go/internal/worker"
)

func TestValidateOrder_Success_StandardOrder(t *testing.T) {
	job := NewMockJob(1001, "validate-order", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":    "ORD-12345",
			"customerId": "CUST-999",
			"items": []map[string]interface{}{
				{"productId": "P1", "name": "Item 1", "quantity": 2, "unitPrice": 10.50},
				{"productId": "P2", "name": "Item 2", "quantity": 1, "unitPrice": 5.00},
			},
		},
	})

	result, err := worker.ValidateOrderHandler(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["orderStatus"] != string(model.OrderStatusValidated) {
		t.Fatalf("expected orderStatus VALIDATED, got %v", result["orderStatus"])
	}

	if result["totalAmount"] != 26.0 {
		t.Fatalf("expected totalAmount 26.0, got %v", result["totalAmount"])
	}

	if result["requiresManualReview"] != false {
		t.Fatalf("expected requiresManualReview to be false for small amount, got %v", result["requiresManualReview"])
	}
}

func TestValidateOrder_Success_HighValueRequiresReview(t *testing.T) {
	job := NewMockJob(1004, "validate-order", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":    "ORD-99999",
			"customerId": "CUST-VIP",
			"items": []map[string]interface{}{
				{"productId": "P-LAPTOP", "name": "Developer Laptop", "quantity": 1, "unitPrice": 2500.00},
			},
		},
	})

	result, err := worker.ValidateOrderHandler(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["requiresManualReview"] != true {
		t.Fatalf("expected requiresManualReview to be true for >= $1000, got %v", result["requiresManualReview"])
	}
}

func TestValidateOrder_MissingOrderID(t *testing.T) {
	job := NewMockJob(1002, "validate-order", map[string]interface{}{
		"order": map[string]interface{}{
			"customerId": "CUST-999",
			"items": []map[string]interface{}{
				{"productId": "P1", "quantity": 1, "unitPrice": 10.0},
			},
		},
	})

	_, err := worker.ValidateOrderHandler(context.Background(), job)
	if err == nil {
		t.Fatalf("expected error for missing orderId, got nil")
	}

	var bizErr *resilience.BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected BusinessError, got %T: %v", err, err)
	}

	if bizErr.Code != resilience.ErrInvalidOrder {
		t.Fatalf("expected error code %s, got %s", resilience.ErrInvalidOrder, bizErr.Code)
	}
}

func TestValidateOrder_InvalidQuantity(t *testing.T) {
	job := NewMockJob(1003, "validate-order", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":    "ORD-123",
			"customerId": "CUST-1",
			"items": []map[string]interface{}{
				{"productId": "P1", "quantity": 0, "unitPrice": 10.0},
			},
		},
	})

	_, err := worker.ValidateOrderHandler(context.Background(), job)
	var bizErr *resilience.BusinessError
	if !errors.As(err, &bizErr) || bizErr.Code != resilience.ErrInvalidOrder {
		t.Fatalf("expected ErrInvalidOrder BusinessError, got: %v", err)
	}
}
