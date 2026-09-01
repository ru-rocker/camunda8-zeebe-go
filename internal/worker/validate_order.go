package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/resilience"
	"github.com/camunda/zeebe/clients/go/v8/pkg/entities"
)

// ValidateOrderHandler handles the 'validate-order' task
func ValidateOrderHandler(ctx context.Context, job entities.Job) (map[string]interface{}, error) {
	variablesMap, err := job.GetVariablesAsMap()
	if err != nil {
		return nil, resilience.NewRetriableError("failed to parse job variables", err)
	}

	var payload model.OrderPayload

	// Try extracting from "order" nested key, or direct root mapping
	if orderVal, exists := variablesMap["order"]; exists {
		bytes, mErr := json.Marshal(orderVal)
		if mErr != nil {
			return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "malformed order payload structure")
		}
		if uErr := json.Unmarshal(bytes, &payload); uErr != nil {
			return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "cannot unmarshal order payload")
		}
	} else {
		bytes, mErr := json.Marshal(variablesMap)
		if mErr != nil {
			return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "malformed root variables")
		}
		if uErr := json.Unmarshal(bytes, &payload); uErr != nil {
			return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "cannot unmarshal root variables into order")
		}
	}

	// Validate order constraints
	if payload.OrderID == "" {
		return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "orderId is required")
	}

	if payload.CustomerID == "" {
		return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "customerId is required")
	}

	if len(payload.Items) == 0 {
		return nil, resilience.NewBusinessError(resilience.ErrInvalidOrder, "order must contain at least one item")
	}

	var calculatedTotal float64
	for i, item := range payload.Items {
		if item.Quantity <= 0 {
			return nil, resilience.NewBusinessError(
				resilience.ErrInvalidOrder,
				fmt.Sprintf("item at index %d has invalid quantity (%d)", i, item.Quantity),
			)
		}
		if item.UnitPrice <= 0 {
			return nil, resilience.NewBusinessError(
				resilience.ErrInvalidOrder,
				fmt.Sprintf("item %s has non-positive price (%.2f)", item.ProductID, item.UnitPrice),
			)
		}
		calculatedTotal += float64(item.Quantity) * item.UnitPrice
	}

	payload.TotalAmount = calculatedTotal
	payload.RequiresManualReview = (calculatedTotal >= 1000.0)
	payload.Status = model.OrderStatusValidated
	payload.UpdatedAt = time.Now().UTC()

	return map[string]interface{}{
		"order":                payload,
		"orderStatus":          string(model.OrderStatusValidated),
		"totalAmount":          calculatedTotal,
		"requiresManualReview": payload.RequiresManualReview,
	}, nil
}
