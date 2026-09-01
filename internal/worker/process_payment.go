package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/resilience"
	"github.com/camunda/zeebe/clients/go/v8/pkg/entities"
)

// PaymentService is an interface for processing payments
type PaymentService interface {
	Charge(ctx context.Context, req model.PaymentRequest) (*model.PaymentResponse, error)
}

// DefaultPaymentService simulates a payment gateway with realistic edge cases
type DefaultPaymentService struct{}

func (s *DefaultPaymentService) Charge(ctx context.Context, req model.PaymentRequest) (*model.PaymentResponse, error) {
	// 1. Simulate Business Error: Declines for specific customer test prefixes or extreme amounts
	if strings.HasPrefix(req.CustomerID, "decline_") || req.TotalAmount > 100000 {
		return nil, resilience.NewBusinessError(
			resilience.ErrPaymentDeclined,
			fmt.Sprintf("credit card declined for customer %s (amount: $%.2f)", req.CustomerID, req.TotalAmount),
		)
	}

	// 2. Simulate Transient Technical Error: Gateway timeout for test prefix
	if strings.HasPrefix(req.CustomerID, "fail_transient_") {
		return nil, resilience.NewRetriableError(
			"payment gateway HTTP 504 Gateway Timeout",
			fmt.Errorf("upstream gateway unreachable"),
		)
	}

	// 3. Normal Success Path
	paymentID := fmt.Sprintf("PAY-%d-%s", time.Now().UnixNano()%1000000, req.OrderID)
	return &model.PaymentResponse{
		PaymentID: paymentID,
		Success:   true,
		Message:   "Payment captured successfully",
	}, nil
}

// NewProcessPaymentHandler creates a resilient handler for the 'process-payment' task
func NewProcessPaymentHandler(paymentService PaymentService) resilience.ResilientJobHandler {
	if paymentService == nil {
		paymentService = &DefaultPaymentService{}
	}

	return func(ctx context.Context, job entities.Job) (map[string]interface{}, error) {
		variablesMap, err := job.GetVariablesAsMap()
		if err != nil {
			return nil, resilience.NewRetriableError("failed to parse job variables", err)
		}

		var payload model.OrderPayload
		if orderVal, exists := variablesMap["order"]; exists {
			bytes, mErr := json.Marshal(orderVal)
			if mErr != nil {
				return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "malformed order payload")
			}
			if uErr := json.Unmarshal(bytes, &payload); uErr != nil {
				return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "cannot unmarshal order payload")
			}
		} else {
			bytes, mErr := json.Marshal(variablesMap)
			if mErr != nil {
				return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "malformed variables")
			}
			if uErr := json.Unmarshal(bytes, &payload); uErr != nil {
				return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "cannot unmarshal variables")
			}
		}

		req := model.PaymentRequest{
			OrderID:     payload.OrderID,
			CustomerID:  payload.CustomerID,
			TotalAmount: payload.TotalAmount,
		}

		resp, chargeErr := paymentService.Charge(ctx, req)
		if chargeErr != nil {
			return nil, chargeErr
		}

		payload.PaymentID = resp.PaymentID
		payload.Status = model.OrderStatusPaid
		payload.UpdatedAt = time.Now().UTC()

		return map[string]interface{}{
			"order":       payload,
			"orderStatus": string(model.OrderStatusPaid),
			"paymentId":   resp.PaymentID,
		}, nil
	}
}
