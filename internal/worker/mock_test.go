package worker_test

import (
	"encoding/json"

	"github.com/camunda/zeebe/clients/go/v8/pkg/entities"
	"github.com/camunda/zeebe/clients/go/v8/pkg/pb"
)

// NewMockJob creates an entities.Job for unit testing
func NewMockJob(key int64, jobType string, vars map[string]interface{}) entities.Job {
	varsJSON, _ := json.Marshal(vars)
	return entities.Job{
		ActivatedJob: &pb.ActivatedJob{
			Key:                key,
			Type:               jobType,
			ProcessInstanceKey: 100000 + key,
			BpmnProcessId:      "order-fulfillment-process",
			ElementId:          "test_activity",
			Retries:            3,
			Variables:          string(varsJSON),
		},
	}
}
