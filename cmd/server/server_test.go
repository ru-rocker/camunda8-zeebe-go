package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_HealthEndpoint_MethodNotAllowed(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405 MethodNotAllowed, got %d", w.Code)
	}
}

func TestServer_StartInstance_InvalidJSON(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/instances", bytes.NewBufferString("invalid-json"))
	w := httptest.NewRecorder()

	srv.handleStartInstance(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 BadRequest for invalid JSON, got %d", w.Code)
	}
}

func TestServer_TaskAction_InvalidEndpoint(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/notanumber/invalid", nil)
	w := httptest.NewRecorder()

	srv.handleTaskAction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid action path, got %d", w.Code)
	}
}

func TestStartProcessRequest_Serialization(t *testing.T) {
	req := StartProcessRequest{
		ProcessID:    "order-risk-fulfillment-process",
		CustomerID:   "cust_test_1",
		CustomerTier: "PLATINUM",
		TotalAmount:  5000.0,
		FraudScore:   12.5,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed StartProcessRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.CustomerTier != "PLATINUM" {
		t.Fatalf("expected PLATINUM, got %s", parsed.CustomerTier)
	}
	if parsed.TotalAmount != 5000.0 {
		t.Fatalf("expected 5000.0, got %v", parsed.TotalAmount)
	}
}
