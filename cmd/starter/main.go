package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"camunda8-zeebe-go/internal/config"
	"camunda8-zeebe-go/internal/model"
	"github.com/camunda/zeebe/clients/go/v8/pkg/zbc"
)

func main() {
	var (
		deployBPMN   bool
		startOrder   bool
		approveOrder bool
		rejectOrder  bool
		processID    string
		scenario     string
		customerID   string
		bpmnFilePath string
	)

	flag.BoolVar(&deployBPMN, "deploy", false, "Deploy BPMN and DMN models to Zeebe")
	flag.BoolVar(&startOrder, "start", false, "Create and start a new order workflow instance")
	flag.BoolVar(&approveOrder, "approve", false, "Simulate human approval on active User Tasks")
	flag.BoolVar(&rejectOrder, "reject", false, "Simulate human rejection on active User Tasks")
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
			log.Printf("  -> Completing User Task jobKey=%d for processInstanceKey=%d...",
				job.GetKey(), job.GetProcessInstanceKey())

			cmd, cErr := client.NewCompleteJobCommand().
				JobKey(job.GetKey()).
				VariablesFromMap(map[string]interface{}{
					"orderApproved": decision,
					"reviewedBy":    "operations_officer_demo",
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

	if !deployBPMN && !startOrder && !approveOrder && !rejectOrder {
		flag.Usage()
		os.Exit(1)
	}
}
