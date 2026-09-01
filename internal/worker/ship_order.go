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

// ShipOrderHandler handles the 'ship-order' task
func ShipOrderHandler(ctx context.Context, job entities.Job) (map[string]interface{}, error) {
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

	trackingNumber := fmt.Sprintf("TRK-%d-%s", time.Now().Unix()%1000000, payload.OrderID)
	payload.TrackingNumber = trackingNumber
	payload.Status = model.OrderStatusShipped
	payload.UpdatedAt = time.Now().UTC()

	shippingDetails := model.ShippingDetails{
		TrackingNumber: trackingNumber,
		Carrier:        "FastLogistics",
		EstimatedDate:  time.Now().Add(72 * time.Hour).UTC(),
	}

	return map[string]interface{}{
		"order":           payload,
		"orderStatus":     string(model.OrderStatusShipped),
		"trackingNumber":  trackingNumber,
		"shippingDetails": shippingDetails,
	}, nil
}
