package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"camunda8-zeebe-go/internal/config"
	"camunda8-zeebe-go/internal/worker"
	"github.com/camunda/zeebe/clients/go/v8/pkg/zbc"
)

func main() {
	log.Println("=====================================================")
	log.Println("  Camunda 8 Resilient Zeebe Worker (Go)")
	log.Println("=====================================================")

	cfg := config.LoadConfig()
	log.Printf("[Config] Gateway: %s (Plaintext/Insecure: %t)", cfg.ZeebeAddress, cfg.ZeebeInsecure)
	log.Printf("[Config] Concurrency: %d, MaxJobsActive: %d, Timeout: %v",
		cfg.WorkerConcurrency, cfg.WorkerMaxJobsActive, cfg.WorkerTimeout)
	log.Printf("[Config] Circuit Breaker: FailThreshold=%d, RecoveryTimeout=%v",
		cfg.CBFailureThreshold, cfg.CBRecoveryTimeout)

	clientConfig := &zbc.ClientConfig{
		GatewayAddress:         cfg.ZeebeAddress,
		UsePlaintextConnection: cfg.ZeebeInsecure,
	}

	// Configure credentials if provided (e.g. for Camunda Cloud / Camunda SaaS / Camunda 8 Self-Managed Keycloak)
	if cfg.ZeebeClientID != "" && cfg.ZeebeClientSecret != "" {
		clientConfig.UsePlaintextConnection = false
		provider, credErr := zbc.NewOAuthCredentialsProvider(&zbc.OAuthProviderConfig{
			ClientID:               cfg.ZeebeClientID,
			ClientSecret:           cfg.ZeebeClientSecret,
			Audience:               cfg.ZeebeAudience,
			AuthorizationServerURL: cfg.ZeebeAuthServerURL,
		})
		if credErr != nil {
			log.Fatalf("[Worker Init] Failed to create OAuth provider: %v", credErr)
		}
		clientConfig.CredentialsProvider = provider
	}

	client, err := zbc.NewClient(clientConfig)
	if err != nil {
		log.Fatalf("[Worker Init] Failed to create Zeebe client: %v", err)
	}
	defer client.Close()

	// Initialize Worker Manager and register handlers
	manager := worker.NewManager(client, cfg, nil)
	manager.RegisterWorkers()

	// Wait for OS shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	log.Println("[Worker Daemon] Running and waiting for workflow jobs. Press Ctrl+C to terminate.")
	sig := <-sigChan
	log.Printf("[Worker Daemon] Received signal %s. Shutting down...", sig)

	manager.Close()
	log.Println("[Worker Daemon] Graceful shutdown complete. Bye!")
}
