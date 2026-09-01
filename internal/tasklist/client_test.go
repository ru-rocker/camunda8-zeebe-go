package tasklist_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camunda8-zeebe-go/internal/tasklist"
)

func TestTasklistClient_SearchTasks(t *testing.T) {
	// Mock HTTP Server simulating Camunda 8 Tasklist REST API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			w.Header().Set("Set-Cookie", "SESSION=mock-session-cookie; Path=/; HttpOnly")
			w.WriteHeader(http.StatusOK)

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

			// Filter by assignee if specified in mock
			if query.Assignee != "" && query.Assignee != "manager_demo" {
				mockTasks = []tasklist.Task{}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockTasks)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := tasklist.NewClient(server.URL, "demo", "demo")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 1. Search for manager_demo
	tasks, err := client.SearchTasks(context.Background(), tasklist.TaskSearchQuery{
		Assignee: "manager_demo",
		State:    tasklist.TaskStateCreated,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Assignee != "manager_demo" {
		t.Fatalf("expected assignee 'manager_demo', got '%s'", tasks[0].Assignee)
	}
	if tasks[0].Name != "Manager Risk Review" {
		t.Fatalf("expected name 'Manager Risk Review', got '%s'", tasks[0].Name)
	}

	// 2. Search for non-existing user
	emptyTasks, err := client.SearchTasks(context.Background(), tasklist.TaskSearchQuery{
		Assignee: "unknown_user",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(emptyTasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(emptyTasks))
	}
}
