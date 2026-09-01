package worker_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/resilience"
	"camunda8-zeebe-go/internal/worker"
)

func TestProcessPayment_Success(t *testing.T) {
	handler := worker.NewProcessPaymentHandler(nil) // uses DefaultPaymentService

	job := NewMockJob(2001, "process-payment", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":     "ORD-2001",
			"customerId":  "cust_john_doe",
			"totalAmount": 150.00,
		},
	})

	result, err := handler(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["orderStatus"] != string(model.OrderStatusPaid) {
		t.Fatalf("expected orderStatus PAID, got %v", result["orderStatus"])
	}

	paymentId, ok := result["paymentId"].(string)
	if !ok || !strings.HasPrefix(paymentId, "PAY-") {
		t.Fatalf("expected valid paymentId starting with PAY-, got %v", paymentId)
	}
}

func TestProcessPayment_DeclineTriggersBusinessError(t *testing.T) {
	handler := worker.NewProcessPaymentHandler(nil)

	job := NewMockJob(2002, "process-payment", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":     "ORD-2002",
			"customerId":  "decline_card_expired",
			"totalAmount": 50.00,
		},
	})

	_, err := handler(context.Background(), job)
	if err == nil {
		t.Fatalf("expected payment decline error, got nil")
	}

	var bizErr *resilience.BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected BusinessError, got %T: %v", err, err)
	}

	if bizErr.Code != resilience.ErrPaymentDeclined {
		t.Fatalf("expected code %s, got %s", resilience.ErrPaymentDeclined, bizErr.Code)
	}
}

func TestProcessPayment_TransientFailureTriggersRetriableError(t *testing.T) {
	handler := worker.NewProcessPaymentHandler(nil)

	job := NewMockJob(2003, "process-payment", map[string]interface{}{
		"order": map[string]interface{}{
			"orderId":     "ORD-2003",
			"customerId":  "fail_transient_upstream_down",
			"totalAmount": 75.00,
		},
	})

	_, err := handler(context.Background(), job)
	if err == nil {
		t.Fatalf("expected transient error, got nil")
	}

	var retErr *resilience.RetriableError
	if !errors.As(err, &retErr) {
		t.Fatalf("expected RetriableError, got %T: %v", err, err)
	}
}
