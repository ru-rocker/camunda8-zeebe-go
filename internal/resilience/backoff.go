package resilience

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// BackoffConfig holds configuration for exponential backoff with jitter
type BackoffConfig struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	MaxRetries      int
}

// DefaultBackoffConfig provides sensible default values for Zeebe worker retries
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		MaxRetries:      5,
	}
}

// ExponentialBackoff calculates backoff duration with full jitter
type ExponentialBackoff struct {
	config BackoffConfig
	rng    *rand.Rand
	mu     sync.Mutex
}

// NewExponentialBackoff creates a new ExponentialBackoff instance
func NewExponentialBackoff(cfg BackoffConfig) *ExponentialBackoff {
	if cfg.InitialInterval <= 0 {
		cfg.InitialInterval = 500 * time.Millisecond
	}
	if cfg.MaxInterval <= 0 {
		cfg.MaxInterval = 30 * time.Second
	}
	if cfg.Multiplier <= 1.0 {
		cfg.Multiplier = 2.0
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 5
	}

	return &ExponentialBackoff{
		config: cfg,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// CalculateBackoff returns backoff duration for a given attempt number (0-indexed) using Full Jitter
func (b *ExponentialBackoff) CalculateBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate exponential interval: initial * (multiplier ^ attempt)
	expFactor := math.Pow(b.config.Multiplier, float64(attempt))
	rawInterval := float64(b.config.InitialInterval) * expFactor

	// Cap at MaxInterval
	maxInterval := float64(b.config.MaxInterval)
	if rawInterval > maxInterval || rawInterval < 0 { // handle potential float overflow
		rawInterval = maxInterval
	}

	// Apply Full Jitter: Sleep between 0 and rawInterval
	b.mu.Lock()
	defer b.mu.Unlock()
	jittered := b.rng.Float64() * rawInterval

	return time.Duration(jittered)
}

// MaxRetries returns the configured maximum retries
func (b *ExponentialBackoff) MaxRetries() int {
	return b.config.MaxRetries
}
