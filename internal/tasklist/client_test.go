package tasklist_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camunda8-zeebe-go/internal/tasklist"
)

func TestTasklistClient_SearchTasks_WithVariables(t *testing.T) {
	// Mock HTTP Server simulating Camunda 8 Tasklist REST API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			w.WriteHeader(http.StatusNoContent)

		case "/v1/tasks/search":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			var query tasklist.TaskSearchQuery
			_ = json.NewDecoder(r.Body).Decode(&query)

			mockTasks := []tasklist.Task{
				{
					ID:                   "2251799813685436",
					Name:                 "Manager Risk Review",
					TaskDefinitionID:     "Activity_ManagerRiskReview",
					ProcessName:          "Order Risk and Fulfillment Process",
					Assignee:             "manager_demo",
					CandidateGroups:      []string{"risk-managers"},
					TaskState:            tasklist.TaskStateCreated,
					ProcessInstanceKey:   "2251799813685420",
					ProcessDefinitionKey: "2251799813685269",
					CreationDate:         "2026-09-01T15:05:00Z",
				},
			}

			// Filter by variable if specified
			if len(query.TaskVariables) > 0 {
				filter := query.TaskVariables[0]
				if filter.Name == "customerTier" && filter.Value != "\"GOLD\"" {
					mockTasks = []tasklist.Task{}
				}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockTasks)

		case "/v1/tasks/2251799813685436/variables/search":
			mockVars := []tasklist.Variable{
				{ID: "1", Name: "customerTier", Value: "GOLD"},
				{ID: "2", Name: "totalAmount", Value: 8000.0},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockVars)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := tasklist.NewClient(server.URL, "demo", "demo")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 1. Search with variable filter customerTier = "GOLD"
	tasks, err := client.SearchTasks(context.Background(), tasklist.TaskSearchQuery{
		State: tasklist.TaskStateCreated,
		TaskVariables: []tasklist.TaskVariableFilter{
			{Name: "customerTier", Value: "\"GOLD\"", Operator: tasklist.OpEqual},
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task matching customerTier=GOLD, got %d", len(tasks))
	}

	// 2. Fetch task variables
	variables, err := client.FetchTaskVariables(context.Background(), tasks[0].ID)
	if err != nil {
		t.Fatalf("failed to fetch variables: %v", err)
	}
	if len(variables) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(variables))
	}
}
