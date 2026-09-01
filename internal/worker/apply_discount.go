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

// ApplyDiscountHandler handles the 'apply-discount' parallel task
func ApplyDiscountHandler(ctx context.Context, job entities.Job) (map[string]interface{}, error) {
	variablesMap, err := job.GetVariablesAsMap()
	if err != nil {
		return nil, resilience.NewRetriableError("failed to parse variables in apply-discount", err)
	}

	var payload model.OrderPayload
	if orderVal, exists := variablesMap["order"]; exists {
		bytes, _ := json.Marshal(orderVal)
		_ = json.Unmarshal(bytes, &payload)
	}

	// Extract discount percent from DMN output if present
	var discountPercent float64
	if dmnResult, exists := variablesMap["decisionResult"].(map[string]interface{}); exists {
		if dp, ok := dmnResult["discountPercent"].(float64); ok {
			discountPercent = dp
		}
	} else if payload.DiscountPercent > 0 {
		discountPercent = payload.DiscountPercent
	}

	discountAmount := payload.TotalAmount * (discountPercent / 100.0)
	discountedTotal := payload.TotalAmount - discountAmount
	if discountedTotal < 0 {
		discountedTotal = 0
	}

	payload.DiscountPercent = discountPercent
	payload.DiscountedAmount = discountedTotal
	payload.UpdatedAt = time.Now().UTC()

	log.Printf("[Worker: apply-discount] Applied %.1f%% discount on Order %s ($%.2f -> $%.2f)",
		discountPercent, payload.OrderID, payload.TotalAmount, discountedTotal)

	return map[string]interface{}{
		"order":            payload,
		"discountPercent":  discountPercent,
		"discountedAmount": discountedTotal,
		"totalAmount":      discountedTotal, // used for payment step
	}, nil
}
