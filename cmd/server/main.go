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
	ProcessID       string                 `json:"processId"`
	CustomerID      string                 `json:"customerId"`
	CustomerTier    string                 `json:"customerTier"`
	TotalAmount     float64                `json:"totalAmount"`
	FraudScore      float64                `json:"fraudScore"`
	CustomVariables map[string]interface{} `json:"customVariables,omitempty"`
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

	// Prepare variables map
	instanceVars := map[string]interface{}{
		"order":        payload,
		"orderId":      orderID,
		"customerId":   req.CustomerID,
		"customerTier": req.CustomerTier,
		"totalAmount":  req.TotalAmount,
		"fraudScore":   req.FraudScore,
	}

	// Merge any user-supplied custom variables
	for k, v := range req.CustomVariables {
		if k != "" {
			instanceVars[k] = v
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cmd, err := s.zeebeClient.NewCreateInstanceCommand().
		BPMNProcessId(req.ProcessID).
		LatestVersion().
		VariablesFromObject(instanceVars)

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
			// Populate task variables for each task
			for i := range tasks {
				if len(tasks[i].Variables) == 0 {
					vars, _ := s.tasklistClient.FetchTaskVariables(ctx, tasks[i].ID)
					tasks[i].Variables = vars
				}
			}
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
		varsMap, _ := j.GetVariablesAsMap()

		assignee := headers["io.camunda.zeebe:assignee"]
		candidateGroup := headers["io.camunda.zeebe:candidateGroups"]

		var candGroups []string
		if candidateGroup != "" {
			candGroups = append(candGroups, candidateGroup)
		}

		if query.TaskDefinitionID != "" && !strings.EqualFold(j.GetElementId(), query.TaskDefinitionID) {
			continue
		}
		if query.Assignee != "" && !strings.EqualFold(assignee, query.Assignee) {
			continue
		}
		if query.CandidateGroup != "" && !strings.Contains(strings.ToLower(candidateGroup), strings.ToLower(query.CandidateGroup)) {
			continue
		}

		// Check Task Variables filter (Case-Insensitive string matching)
		matchedVars := true
		for _, filter := range query.TaskVariables {
			// Find variable key with case-insensitive check
			var val interface{}
			found := false
			for k, v := range varsMap {
				if strings.EqualFold(k, filter.Name) {
					val = v
					found = true
					break
				}
			}

			if !found {
				matchedVars = false
				break
			}

			valStr := fmt.Sprintf("%v", val)
			cleanFilterVal := strings.Trim(filter.Value, "\"")

			switch filter.Operator {
			case tasklist.OpEqual, "":
				if !strings.EqualFold(valStr, cleanFilterVal) {
					matchedVars = false
				}
			case tasklist.OpLike:
				if !strings.Contains(strings.ToLower(valStr), strings.ToLower(cleanFilterVal)) {
					matchedVars = false
				}
			case tasklist.OpNotEqual:
				if strings.EqualFold(valStr, cleanFilterVal) {
					matchedVars = false
				}
			case tasklist.OpGreaterThanOrEqual:
				vNum, _ := strconv.ParseFloat(valStr, 64)
				fNum, _ := strconv.ParseFloat(cleanFilterVal, 64)
				if vNum < fNum {
					matchedVars = false
				}
			case tasklist.OpLessThanOrEqual:
				vNum, _ := strconv.ParseFloat(valStr, 64)
				fNum, _ := strconv.ParseFloat(cleanFilterVal, 64)
				if vNum > fNum {
					matchedVars = false
				}
			}
		}

		if !matchedVars {
			continue
		}

		// Convert variables to Tasklist Variable objects
		var taskVars []tasklist.Variable
		for k, v := range varsMap {
			if k != "order" { // avoid huge order object in summary tag
				taskVars = append(taskVars, tasklist.Variable{
					Name:  k,
					Value: v,
				})
			}
		}

		fallbackTasks = append(fallbackTasks, tasklist.Task{
			ID:                 strconv.FormatInt(j.GetKey(), 10),
			Name:               j.GetElementId(),
			Assignee:           assignee,
			CandidateGroups:    candGroups,
			TaskState:          tasklist.TaskStateCreated,
			ProcessInstanceKey: strconv.FormatInt(j.GetProcessInstanceKey(), 10),
			Variables:          taskVars,
			CreationDate:       time.Now().UTC().Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fallbackTasks)
}

type CompleteTaskRequest struct {
	Approved bool `json:"approved"`
}

// GET /api/tasks/:id or POST /api/tasks/:id/complete
func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid task endpoint", http.StatusBadRequest)
		return
	}

	taskID := pathParts[2]

	// Handle GET /api/tasks/{id} (Fetch specific task)
	if r.Method == http.MethodGet {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if s.tasklistClient != nil {
			task, err := s.tasklistClient.GetTask(ctx, taskID)
			if err == nil {
				vars, _ := s.tasklistClient.FetchTaskVariables(ctx, taskID)
				task.Variables = vars
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(task)
				return
			}
		}

		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if len(pathParts) < 4 || pathParts[3] != "complete" {
		http.Error(w, "Invalid task action endpoint", http.StatusBadRequest)
		return
	}

	taskKey, err := strconv.ParseInt(taskID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var req CompleteTaskRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// 1. Try completing via Tasklist REST API
	completed := false
	if s.tasklistClient != nil {
		completeVars := []tasklist.Variable{
			{Name: "orderApproved", Value: req.Approved},
			{Name: "managerApproved", Value: req.Approved},
			{Name: "reviewedBy", Value: "web_operator_admin"},
		}

		// First assign if needed, then complete
		_ = s.tasklistClient.AssignTask(ctx, taskID, "manager_demo")
		if err := s.tasklistClient.CompleteTask(ctx, taskID, completeVars); err == nil {
			completed = true
		}
	}

	// 2. Fallback: complete via Zeebe Job Command if Tasklist complete did not execute
	if !completed {
		cmd, err := s.zeebeClient.NewCompleteJobCommand().
			JobKey(taskKey).
			VariablesFromMap(map[string]interface{}{
				"orderApproved":   req.Approved,
				"managerApproved": req.Approved,
				"reviewedBy":      "web_operator_admin",
			})

		if err == nil {
			if _, sendErr := cmd.Send(ctx); sendErr == nil {
				completed = true
			}
		}
	}

	if !completed {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to complete task via Tasklist and Zeebe engine"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"taskId":   taskID,
		"approved": req.Approved,
	})
}
