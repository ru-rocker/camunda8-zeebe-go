package resilience

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"github.com/camunda/zeebe/clients/go/v8/pkg/entities"
	"github.com/camunda/zeebe/clients/go/v8/pkg/worker"
)

// ResilientJobHandler is a business handler function returning variable updates or an error
type ResilientJobHandler func(ctx context.Context, job entities.Job) (map[string]interface{}, error)

// Middleware wraps a ResilientJobHandler with cross-cutting concerns (logging, metrics, recovery, retry)
type Middleware func(next ResilientJobHandler) ResilientJobHandler

// ResilienceMiddlewareConfig provides configuration for the middleware chain
type ResilienceMiddlewareConfig struct {
	Timeout        time.Duration
	Backoff        *ExponentialBackoff
	CircuitBreaker *CircuitBreaker
	InitialRetries int32
}

// WrapResilientHandler converts a ResilientJobHandler into a Zeebe worker.JobHandler with full resilience
func WrapResilientHandler(handler ResilientJobHandler, cfg ResilienceMiddlewareConfig) worker.JobHandler {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Backoff == nil {
		cfg.Backoff = NewExponentialBackoff(DefaultBackoffConfig())
	}
	if cfg.InitialRetries <= 0 {
		cfg.InitialRetries = int32(cfg.Backoff.MaxRetries())
	}

	return func(client worker.JobClient, job entities.Job) {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()

		jobKey := job.GetKey()
		jobType := job.GetType()
		processInstanceKey := job.GetProcessInstanceKey()

		log.Printf("[Job Worker] [START] type=%s jobKey=%d processInstanceKey=%d retriesLeft=%d",
			jobType, jobKey, processInstanceKey, job.GetRetries())

		// 1. Panic Recovery
		var (
			variables map[string]interface{}
			err       error
		)

		func() {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					err = fmt.Errorf("panic in worker %s: %v\n%s", jobType, r, stack)
					log.Printf("[Job Worker] [PANIC] jobKey=%d error=%v", jobKey, err)
				}
			}()

			// 2. Circuit Breaker protection (if configured)
			if cfg.CircuitBreaker != nil {
				cbErr := cfg.CircuitBreaker.Execute(func() error {
					var innerErr error
					variables, innerErr = handler(ctx, job)
					return innerErr
				})
				if cbErr != nil {
					err = cbErr
				}
			} else {
				variables, err = handler(ctx, job)
			}
		}()

		// 3. Handle Result
		if err == nil {
			// SUCCESS -> Complete Job
			cmd := client.NewCompleteJobCommand().JobKey(jobKey)
			if variables != nil && len(variables) > 0 {
				varCmd, cmdErr := cmd.VariablesFromMap(variables)
				if cmdErr != nil {
					log.Printf("[Job Worker] [ERROR] Failed to serialize variables for jobKey=%d: %v", jobKey, cmdErr)
					failJob(ctx, client, job, cmdErr, cfg.Backoff, cfg.InitialRetries)
					return
				}
				_, sendErr := varCmd.Send(ctx)
				if sendErr != nil {
					log.Printf("[Job Worker] [ERROR] Failed to send CompleteJob for jobKey=%d: %v", jobKey, sendErr)
				} else {
					log.Printf("[Job Worker] [SUCCESS] jobKey=%d completed successfully", jobKey)
				}
				return
			}

			_, sendErr := cmd.Send(ctx)
			if sendErr != nil {
				log.Printf("[Job Worker] [ERROR] Failed to send CompleteJob for jobKey=%d: %v", jobKey, sendErr)
			} else {
				log.Printf("[Job Worker] [SUCCESS] jobKey=%d completed successfully", jobKey)
			}
			return
		}

		// FAILURE HANDLING
		// Check for Business Error (ThrowError to BPMN boundary event)
		var bizErr *BusinessError
		if errors.As(err, &bizErr) {
			log.Printf("[Job Worker] [BPMN ERROR] jobKey=%d throwing BPMN error code=%s message=%s",
				jobKey, bizErr.Code, bizErr.Message)

			_, throwErr := client.NewThrowErrorCommand().
				JobKey(jobKey).
				ErrorCode(bizErr.Code).
				ErrorMessage(bizErr.Message).
				Send(ctx)

			if throwErr != nil {
				log.Printf("[Job Worker] [ERROR] Failed to send ThrowError for jobKey=%d: %v", jobKey, throwErr)
			}
			return
		}

		// Technical / Retriable Error or Panic -> FailJob with exponential backoff & jitter
		failJob(ctx, client, job, err, cfg.Backoff, cfg.InitialRetries)
	}
}

func failJob(ctx context.Context, client worker.JobClient, job entities.Job, err error, backoff *ExponentialBackoff, maxRetries int32) {
	currentRetries := job.GetRetries()
	newRetries := currentRetries - 1

	if newRetries <= 0 {
		// Retries exhausted: Zeebe will create an Incident in Operate
		log.Printf("[Job Worker] [INCIDENT] jobKey=%d retries exhausted (0). Raising incident. Reason: %v",
			job.GetKey(), err)

		_, failErr := client.NewFailJobCommand().
			JobKey(job.GetKey()).
			Retries(0).
			ErrorMessage(fmt.Sprintf("Retries exhausted. Root cause: %v", err)).
			Send(ctx)

		if failErr != nil {
			log.Printf("[Job Worker] [ERROR] Failed to send FailJob (0 retries) for jobKey=%d: %v", job.GetKey(), failErr)
		}
		return
	}

	// Calculate exponential backoff duration
	attemptIndex := int(maxRetries - currentRetries)
	backoffDuration := backoff.CalculateBackoff(attemptIndex)

	log.Printf("[Job Worker] [RETRY] jobKey=%d failed (retries left: %d -> %d, backoff: %v). Reason: %v",
		job.GetKey(), currentRetries, newRetries, backoffDuration, err)

	_, failErr := client.NewFailJobCommand().
		JobKey(job.GetKey()).
		Retries(newRetries).
		RetryBackoff(backoffDuration).
		ErrorMessage(err.Error()).
		Send(ctx)

	if failErr != nil {
		log.Printf("[Job Worker] [ERROR] Failed to send FailJob for jobKey=%d: %v", job.GetKey(), failErr)
	}
}
