package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"camunda8-zeebe-go/internal/config"
	"camunda8-zeebe-go/internal/model"
	"camunda8-zeebe-go/internal/tasklist"
	"github.com/camunda/zeebe/clients/go/v8/pkg/zbc"
)

func main() {
	var (
		deployBPMN    bool
		startOrder    bool
		listTasks     bool
		tasklistQuery bool
		taskState     string
		approveOrder  bool
		rejectOrder   bool
		targetUser    string
		processID     string
		scenario      string
		customerID    string
		bpmnFilePath  string
	)

	flag.BoolVar(&deployBPMN, "deploy", false, "Deploy BPMN and DMN models to Zeebe")
	flag.BoolVar(&startOrder, "start", false, "Create and start a new order workflow instance")
	flag.BoolVar(&listTasks, "list-tasks", false, "List pending User Tasks via Zeebe Broker job polling (prototype)")
	flag.BoolVar(&tasklistQuery, "tasklist-query", false, "Query User Tasks via Camunda Tasklist Read Model (Production)")
	flag.StringVar(&taskState, "state", "CREATED", "Task state filter for Tasklist API: CREATED | COMPLETED | CANCELED")
	flag.BoolVar(&approveOrder, "approve", false, "Simulate human approval on active User Tasks")
	flag.BoolVar(&rejectOrder, "reject", false, "Simulate human rejection on active User Tasks")
	flag.StringVar(&targetUser, "user", "", "Filter tasks by Assignee or Candidate Group (e.g. manager_demo, order-reviewers)")
	flag.StringVar(&processID, "process", "", "Target BPMN process ID (default: auto based on scenario)")
	flag.StringVar(&scenario, "scenario", "success", "Scenario: success | review | decline | transient | invalid | risk-platinum | risk-high | risk-fraud | risk-stock")
	flag.StringVar(&customerID, "customer", "", "Custom Customer ID")
	flag.StringVar(&bpmnFilePath, "bpmn", "bpmn/order-fulfillment.bpmn", "Path to BPMN file")
	flag.Parse()

	cfg := config.LoadConfig()
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
		if credErr != nil {
			log.Fatalf("[Starter] Failed to create OAuth provider: %v", credErr)
		}
		clientConfig.CredentialsProvider = provider
	}

	client, err := zbc.NewClient(clientConfig)
	if err != nil {
		log.Fatalf("[Starter] Failed to create Zeebe client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if deployBPMN {
		log.Println("[Starter] Deploying all BPMN workflows and DMN decision tables to Zeebe...")
		deployCmd := client.NewDeployResourceCommand().
			AddResourceFile("bpmn/order-risk-rules.dmn").
			AddResourceFile("bpmn/order-risk-fulfillment.bpmn").
			AddResourceFile("bpmn/order-fulfillment.bpmn")

		deployResp, dErr := deployCmd.Send(ctx)
		if dErr != nil {
			log.Fatalf("[Starter] Deployment failed: %v", dErr)
		}

		log.Printf("[Starter] Deployment successful! Key=%d", deployResp.Key)
		for _, p := range deployResp.Deployments {
			if proc := p.GetProcess(); proc != nil {
				log.Printf("  -> Process: %s (v%d, key: %d)",
					proc.BpmnProcessId, proc.Version, proc.ProcessDefinitionKey)
			}
			if dec := p.GetDecision(); dec != nil {
				log.Printf("  -> Decision Table (DMN): %s (v%d, key: %d)",
					dec.DmnDecisionId, dec.Version, dec.DecisionKey)
			}
		}
	}

	if startOrder {
		orderID := fmt.Sprintf("ORD-%d", time.Now().Unix()%100000)
		targetProcess := "order-fulfillment-process"
		tier := "STANDARD"
		var fraudScore float64 = 5.0
		cust := customerID

		var items []model.OrderItem

		switch scenario {
		case "risk-platinum":
			targetProcess = "order-risk-fulfillment-process"
			tier = "PLATINUM"
			fraudScore = 5.0
			cust = "cust_platinum_alice"
			items = []model.OrderItem{
				{ProductID: "PROD-SERVER-1", Name: "Enterprise Cloud Server", Quantity: 1, UnitPrice: 1500.00},
			}

		case "risk-fraud":
			targetProcess = "order-risk-fulfillment-process"
			tier = "STANDARD"
			fraudScore = 95.0 // Critical fraud -> DMN will auto-reject
			cust = "cust_fraud_spammer"
			items = []model.OrderItem{
				{ProductID: "PROD-JEWELRY-9", Name: "Diamond Watch", Quantity: 5, UnitPrice: 3000.00},
			}

		case "risk-high":
			targetProcess = "order-risk-fulfillment-process"
			tier = "GOLD"
			fraudScore = 55.0 // High risk + High value -> DMN requires Manager Approval
			cust = "cust_high_risk_bob"
			items = []model.OrderItem{
				{ProductID: "PROD-CLUSTER-5", Name: "AI Compute Cluster", Quantity: 1, UnitPrice: 8000.00},
			}

		case "risk-stock":
			targetProcess = "order-risk-fulfillment-process"
			tier = "SILVER"
			fraudScore = 10.0
			cust = "cust_silver_charlie"
			items = []model.OrderItem{
				{ProductID: "PROD-OUT_OF_STOCK-99", Name: "Vintage GPU", Quantity: 1, UnitPrice: 800.00},
			}

		case "review":
			items = []model.OrderItem{
				{ProductID: "PROD-SERVER-99", Name: "Enterprise Cloud Server", Quantity: 1, UnitPrice: 2500.00},
			}
			if cust == "" {
				cust = "cust_vip_review_777"
			}

		case "decline":
			if cust == "" {
				cust = "decline_cust_999"
			}
			items = []model.OrderItem{
				{ProductID: "PROD-101", Name: "Golang Plush", Quantity: 2, UnitPrice: 25.00},
			}

		case "invalid":
			cust = "" // Empty triggers validation error

		default:
			if cust == "" {
				cust = "cust_happy_456"
			}
			items = []model.OrderItem{
				{ProductID: "PROD-101", Name: "Golang Gopher Plush", Quantity: 2, UnitPrice: 25.50},
				{ProductID: "PROD-202", Name: "Camunda 8 Guide Book", Quantity: 1, UnitPrice: 49.00},
			}
		}

		if processID != "" {
			targetProcess = processID
		}

		var totalAmt float64
		for _, it := range items {
			totalAmt += float64(it.Quantity) * it.UnitPrice
		}

		payload := model.OrderPayload{
			OrderID:      orderID,
			CustomerID:   cust,
			CustomerTier: tier,
			FraudScore:   fraudScore,
			Items:        items,
			TotalAmount:  totalAmt,
			Status:       model.OrderStatusCreated,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}

		log.Printf("[Starter] Starting process '%s' for scenario '%s' (OrderID: %s, Customer: %s, Tier: %s, Amount: $%.2f, FraudScore: %.1f)...",
			targetProcess, scenario, orderID, cust, tier, totalAmt, fraudScore)

		instResp, instErr := client.NewCreateInstanceCommand().
			BPMNProcessId(targetProcess).
			LatestVersion().
			VariablesFromObject(map[string]interface{}{
				"order":        payload,
				"customerTier": tier,
				"totalAmount":  totalAmt,
				"fraudScore":   fraudScore,
			})

		if instErr != nil {
			log.Fatalf("[Starter] Failed to create instance command: %v", instErr)
		}

		res, sendErr := instResp.Send(ctx)
		if sendErr != nil {
			log.Fatalf("[Starter] Failed to send process instance: %v", sendErr)
		}

		log.Printf("[Starter] Process instance created successfully!")
		log.Printf("  -> ProcessInstanceKey: %d", res.ProcessInstanceKey)
		log.Printf("  -> BPMNProcessId: %s (version: %d)", res.BpmnProcessId, res.Version)
	}

	if tasklistQuery {
		log.Printf("[Starter] Querying Tasklist Read Model (%s) for State='%s', User='%s'...",
			cfg.TasklistURL, taskState, targetUser)

		tlClient, tlErr := tasklist.NewClient(cfg.TasklistURL, cfg.TasklistUsername, cfg.TasklistPassword)
		if tlErr != nil {
			log.Fatalf("[Starter] Failed to initialize Tasklist Client: %v", tlErr)
		}

		query := tasklist.TaskSearchQuery{
			State:    tasklist.TaskState(taskState),
			PageSize: 50,
		}
		if targetUser != "" {
			query.Assignee = targetUser
		}

		tasks, sErr := tlClient.SearchTasks(ctx, query)
		if sErr != nil {
			log.Fatalf("[Starter] Tasklist query failed: %v", sErr)
		}

		if len(tasks) == 0 {
			log.Printf("[Starter] No tasks found matching State='%s', User='%s' in Tasklist.", taskState, targetUser)
			return
		}

		fmt.Printf("\n%-18s | %-16s | %-28s | %-16s | %-10s | %-20s\n",
			"Task ID", "Process Instance", "Task Name", "Assignee", "State", "Creation Date")
		fmt.Println("-------------------|------------------|------------------------------|------------------|------------|---------------------")

		for _, t := range tasks {
			assignee := t.Assignee
			if assignee == "" {
				assignee = "<unassigned>"
			}
			fmt.Printf("%-18s | %-16s | %-28s | %-16s | %-10s | %-20s\n",
				t.ID, t.ProcessInstanceKey, t.Name, assignee, t.TaskState, t.CreationDate)
		}

		fmt.Printf("\nTotal tasks returned from Tasklist Read Model: %d (Zero broker locks used)\n\n", len(tasks))
	}

	if listTasks {
		log.Printf("[Starter] Querying active User Tasks in Zeebe (Filter User: '%s')...", targetUser)

		jobsResp, actErr := client.NewActivateJobsCommand().
			JobType("io.camunda.zeebe:userTask").
			MaxJobsToActivate(50).
			Timeout(5 * time.Second).
			WorkerName("cli-task-lister").
			Send(ctx)

		if actErr != nil {
			log.Fatalf("[Starter] Failed to fetch user tasks: %v", actErr)
		}

		if len(jobsResp) == 0 {
			log.Println("[Starter] No pending User Tasks found waiting in queue.")
			return
		}

		matchedCount := 0
		fmt.Printf("\n%-18s | %-16s | %-16s | %-18s | %-12s | %-10s\n",
			"Task Key (JobID)", "Process Instance", "Assignee", "Candidate Groups", "Order ID", "Amount ($)")
		fmt.Println("-------------------|------------------|------------------|--------------------|--------------|-----------")

		for _, job := range jobsResp {
			headers, _ := job.GetCustomHeadersAsMap()
			vars, _ := job.GetVariablesAsMap()

			assignee := headers["io.camunda.zeebe:assignee"]
			candidateGroups := headers["io.camunda.zeebe:candidateGroups"]

			if assignee == "" {
				assignee = "<unassigned>"
			}
			if candidateGroups == "" {
				candidateGroups = "<none>"
			}

			// Filter if user provided
			if targetUser != "" {
				if assignee != targetUser && !strings.Contains(candidateGroups, targetUser) {
					continue
				}
			}

			matchedCount++
			orderID := "-"
			var totalAmount float64
			if orderMap, ok := vars["order"].(map[string]interface{}); ok {
				if id, ok := orderMap["orderId"].(string); ok {
					orderID = id
				}
				if amt, ok := orderMap["totalAmount"].(float64); ok {
					totalAmount = amt
				}
			} else if amt, ok := vars["totalAmount"].(float64); ok {
				totalAmount = amt
			}

			fmt.Printf("%-18d | %-16d | %-16s | %-18s | %-12s | $%-9.2f\n",
				job.GetKey(), job.GetProcessInstanceKey(), assignee, candidateGroups, orderID, totalAmount)
		}

		fmt.Printf("\nTotal pending tasks found for '%s': %d (Locks will auto-release back to queue)\n\n",
			targetUser, matchedCount)
	}

	if approveOrder || rejectOrder {
		decision := approveOrder
		decisionStr := "APPROVED"
		if rejectOrder {
			decisionStr = "REJECTED"
		}

		log.Printf("[Starter] Polling for pending 'Manual Order Review' User Tasks to mark as %s...", decisionStr)

		jobsResp, actErr := client.NewActivateJobsCommand().
			JobType("io.camunda.zeebe:userTask").
			MaxJobsToActivate(10).
			Timeout(30 * time.Second).
			WorkerName("cli-human-reviewer").
			Send(ctx)

		if actErr != nil {
			log.Fatalf("[Starter] Failed to activate user tasks: %v", actErr)
		}

		if len(jobsResp) == 0 {
			log.Println("[Starter] No pending User Tasks found waiting for review.")
			return
		}

		log.Printf("[Starter] Found %d active User Task(s). Submitting human review decision (%s)...",
			len(jobsResp), decisionStr)

		for _, job := range jobsResp {
			headers, _ := job.GetCustomHeadersAsMap()
			assignee := headers["io.camunda.zeebe:assignee"]
			candidateGroups := headers["io.camunda.zeebe:candidateGroups"]

			if targetUser != "" && assignee != targetUser && !strings.Contains(candidateGroups, targetUser) {
				log.Printf("  -> Skipping jobKey=%d (assignee: %s, candidateGroup: %s - not matching filter '%s')",
					job.GetKey(), assignee, candidateGroups, targetUser)
				continue
			}

			log.Printf("  -> Completing User Task jobKey=%d for processInstanceKey=%d (Assignee: %s, Groups: %s)...",
				job.GetKey(), job.GetProcessInstanceKey(), assignee, candidateGroups)

			cmd, cErr := client.NewCompleteJobCommand().
				JobKey(job.GetKey()).
				VariablesFromMap(map[string]interface{}{
					"orderApproved":   decision,
					"managerApproved": decision,
					"reviewedBy":      "operations_officer_demo",
				})

			if cErr != nil {
				log.Printf("     [ERROR] Failed to set variables for jobKey=%d: %v", job.GetKey(), cErr)
				continue
			}

			_, sendErr := cmd.Send(ctx)
			if sendErr != nil {
				log.Printf("     [ERROR] Failed to complete user task jobKey=%d: %v", job.GetKey(), sendErr)
			} else {
				log.Printf("     [SUCCESS] User Task jobKey=%d marked as %s! Token will continue in workflow.",
					job.GetKey(), decisionStr)
			}
		}
	}

	if !deployBPMN && !startOrder && !listTasks && !tasklistQuery && !approveOrder && !rejectOrder {
		flag.Usage()
		os.Exit(1)
	}
}
