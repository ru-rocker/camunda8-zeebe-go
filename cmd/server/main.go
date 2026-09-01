package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"camunda8-zeebe-go/internal/config"
	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/tasklist"
	"github.com/camunda/zeebe/clients/go/v8/pkg/zbc"
)

type Server struct {
	cfg            *config.Config
	zeebeClient    zbc.Client
	tasklistClient *tasklist.Client
}

func NewServer(cfg *config.Config) (*Server, error) {
	clientConfig := &zbc.ClientConfig{
		GatewayAddress:         cfg.ZeebeAddress,
		UsePlaintextConnection: cfg.ZeebeInsecure,
	}

	if cfg.ZeebeClientID != "" && cfg.ZeebeClientSecret != "" {
		clientConfig.UsePlaintextConnection = false
		provider, credErr := zbc.NewOAuthCredentialsProvider(&zbc.OAuthProviderConfig{
			ClientID:               cfg.ZeebeClientID,
			ClientSecret:           cfg.ZeebeClientSecret,
			Audience:               cfg.ZeebeAudience,
			AuthorizationServerURL: cfg.ZeebeAuthServerURL,
		})
		if credErr == nil {
			clientConfig.CredentialsProvider = provider
		}
	}

	zClient, err := zbc.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Zeebe client: %w", err)
	}

	tlClient, _ := tasklist.NewClient(cfg.TasklistURL, cfg.TasklistUsername, cfg.TasklistPassword)

	return &Server{
		cfg:            cfg,
		zeebeClient:    zClient,
		tasklistClient: tlClient,
	}, nil
}

func main() {
	cfg := config.LoadConfig()
	srv, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("[Server] Failed to initialize server: %v", err)
	}
	defer srv.zeebeClient.Close()

	mux := http.NewServeMux()

	// Static Files
	fileServer := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fileServer)

	// API Routes
	mux.HandleFunc("/api/health", srv.handleHealth)
	mux.HandleFunc("/api/deploy", srv.handleDeploy)
	mux.HandleFunc("/api/instances", srv.handleStartInstance)
	mux.HandleFunc("/api/tasks/search", srv.handleTaskSearch)
	mux.HandleFunc("/api/tasks/", srv.handleTaskAction)

	addr := ":3000"
	log.Printf("[Server] Camunda 8 Web UI running at http://localhost%s", addr)
	log.Printf("[Server] Proxying Zeebe (%s) & Tasklist (%s)", cfg.ZeebeAddress, cfg.TasklistURL)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[Server] Server error: %v", err)
	}
}

// GET /api/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	zeebeOk := false
	_, err := s.zeebeClient.NewTopologyCommand().Send(ctx)
	if err == nil {
		zeebeOk = true
	}

	tasklistOk := false
	if s.tasklistClient != nil {
		if err := s.tasklistClient.Authenticate(ctx); err == nil {
			tasklistOk = true
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"zeebe":    zeebeOk,
		"tasklist": tasklistOk,
		"status":   "UP",
	})
}

// POST /api/deploy
func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	deployCmd := s.zeebeClient.NewDeployResourceCommand().
		AddResourceFile("bpmn/order-risk-rules.dmn").
		AddResourceFile("bpmn/order-risk-fulfillment.bpmn").
		AddResourceFile("bpmn/order-fulfillment.bpmn")

	res, err := deployCmd.Send(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"key":     res.Key,
	})
}

type StartProcessRequest struct {
	ProcessID    string  `json:"processId"`
	CustomerID   string  `json:"customerId"`
	CustomerTier string  `json:"customerTier"`
	TotalAmount  float64 `json:"totalAmount"`
	FraudScore   float64 `json:"fraudScore"`
}

// POST /api/instances
func (s *Server) handleStartInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.ProcessID == "" {
		req.ProcessID = "order-risk-fulfillment-process"
	}
	if req.CustomerID == "" {
		req.CustomerID = "cust_web_user"
	}
	if req.CustomerTier == "" {
		req.CustomerTier = "GOLD"
	}

	orderID := fmt.Sprintf("ORD-%d", time.Now().Unix()%100000)
	payload := model.OrderPayload{
		OrderID:      orderID,
		CustomerID:   req.CustomerID,
		CustomerTier: req.CustomerTier,
		FraudScore:   req.FraudScore,
		TotalAmount:  req.TotalAmount,
		Items: []model.OrderItem{
			{ProductID: "PROD-ITEM-1", Name: "Standard Order Item", Quantity: 1, UnitPrice: req.TotalAmount},
		},
		Status:    model.OrderStatusCreated,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cmd, err := s.zeebeClient.NewCreateInstanceCommand().
		BPMNProcessId(req.ProcessID).
		LatestVersion().
		VariablesFromObject(map[string]interface{}{
			"order":        payload,
			"customerTier": req.CustomerTier,
			"totalAmount":  req.TotalAmount,
			"fraudScore":   req.FraudScore,
		})

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	res, err := cmd.Send(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":            true,
		"processInstanceKey": res.ProcessInstanceKey,
		"bpmnProcessId":      res.BpmnProcessId,
		"version":            res.Version,
	})
}

// POST /api/tasks/search
func (s *Server) handleTaskSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var query tasklist.TaskSearchQuery
	_ = json.NewDecoder(r.Body).Decode(&query)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// 1. Query Tasklist Read Model
	if s.tasklistClient != nil {
		tasks, err := s.tasklistClient.SearchTasks(ctx, query)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tasks)
			return
		}
	}

	// 2. Fallback: Query active userTask jobs directly from Zeebe Engine
	jobsResp, err := s.zeebeClient.NewActivateJobsCommand().
		JobType("io.camunda.zeebe:userTask").
		MaxJobsToActivate(50).
		Timeout(5 * time.Second).
		WorkerName("web-task-reader").
		Send(ctx)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	var fallbackTasks []tasklist.Task
	for _, j := range jobsResp {
		headers, _ := j.GetCustomHeadersAsMap()
		assignee := headers["io.camunda.zeebe:assignee"]
		candidateGroup := headers["io.camunda.zeebe:candidateGroups"]

		var candGroups []string
		if candidateGroup != "" {
			candGroups = append(candGroups, candidateGroup)
		}

		if query.Assignee != "" && assignee != query.Assignee {
			continue
		}
		if query.CandidateGroup != "" && !strings.Contains(candidateGroup, query.CandidateGroup) {
			continue
		}

		fallbackTasks = append(fallbackTasks, tasklist.Task{
			ID:                 strconv.FormatInt(j.GetKey(), 10),
			Name:               j.GetElementId(),
			Assignee:           assignee,
			CandidateGroups:    candGroups,
			TaskState:          tasklist.TaskStateCreated,
			ProcessInstanceKey: strconv.FormatInt(j.GetProcessInstanceKey(), 10),
			CreationDate:       time.Now().UTC().Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fallbackTasks)
}

type CompleteTaskRequest struct {
	Approved bool `json:"approved"`
}

// POST /api/tasks/:id/complete
func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[3] != "complete" {
		http.Error(w, "Invalid task action endpoint", http.StatusBadRequest)
		return
	}

	taskIDStr := pathParts[2]
	taskKey, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var req CompleteTaskRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cmd, err := s.zeebeClient.NewCompleteJobCommand().
		JobKey(taskKey).
		VariablesFromMap(map[string]interface{}{
			"orderApproved":   req.Approved,
			"managerApproved": req.Approved,
			"reviewedBy":      "web_operator_admin",
		})

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_, sendErr := cmd.Send(ctx)
	if sendErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": sendErr.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"taskId":   taskKey,
		"approved": req.Approved,
	})
}
