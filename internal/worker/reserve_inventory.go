package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/resilience"
	"github.com/camunda/zeebe/clients/go/v8/pkg/entities"
)

// ReserveInventoryHandler handles the 'reserve-inventory' parallel task
func ReserveInventoryHandler(ctx context.Context, job entities.Job) (map[string]interface{}, error) {
	variablesMap, err := job.GetVariablesAsMap()
	if err != nil {
		return nil, resilience.NewRetriableError("failed to parse variables in reserve-inventory", err)
	}

	var payload model.OrderPayload
	if orderVal, exists := variablesMap["order"]; exists {
		bytes, _ := json.Marshal(orderVal)
		_ = json.Unmarshal(bytes, &payload)
	}

	// Check for simulated out-of-stock trigger
	for _, item := range payload.Items {
		if strings.Contains(strings.ToLower(item.ProductID), "out_of_stock") || item.Quantity > 100 {
			return nil, resilience.NewBusinessError(
				resilience.ErrOutOfStock,
				fmt.Sprintf("Item %s is out of stock in regional warehouse", item.ProductID),
			)
		}
	}

	payload.InventoryReserved = true
	payload.UpdatedAt = time.Now().UTC()

	log.Printf("[Worker: reserve-inventory] Successfully reserved stock for Order %s (%d items)",
		payload.OrderID, len(payload.Items))

	return map[string]interface{}{
		"order":             payload,
		"inventoryReserved": true,
	}, nil
}
