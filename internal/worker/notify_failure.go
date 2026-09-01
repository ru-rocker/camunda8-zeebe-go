package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/resilience"
	"github.com/camunda/zeebe/clients/go/v8/pkg/entities"
)

// NotifyCustomerFailureHandler handles 'notify-customer-failure' task
func NotifyCustomerFailureHandler(ctx context.Context, job entities.Job) (map[string]interface{}, error) {
	variablesMap, err := job.GetVariablesAsMap()
	if err != nil {
		return nil, resilience.NewRetriableError("failed to parse job variables", err)
	}

	var payload model.OrderPayload
	if orderVal, exists := variablesMap["order"]; exists {
		bytes, mErr := json.Marshal(orderVal)
		if mErr == nil {
			_ = json.Unmarshal(bytes, &payload)
		}
	}

	log.Printf("[Notification] Alerted customer %s: Order %s was cancelled due to payment failure.",
		payload.CustomerID, payload.OrderID)

	payload.Status = model.OrderStatusFailed
	payload.FailureReason = "Payment Declined"
	payload.UpdatedAt = time.Now().UTC()

	return map[string]interface{}{
		"order":         payload,
		"orderStatus":   string(model.OrderStatusFailed),
		"failureReason": "Payment Declined",
	}, nil
}
