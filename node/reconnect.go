package node

import (
	"log"
	"math"
	"math/rand"
	"strings"
	"time"
)

// Reconnector handles automatic reconnection with exponential backoff
type Reconnector struct {
	configFile        *ConfigFile
	initialDelay      time.Duration
	backoffMultiplier float64
	jitterFactor      float64
	shutdown          chan struct{}
}

// NewReconnector creates a new reconnector with the given config
func NewReconnector(configFile *ConfigFile) *Reconnector {
	return &Reconnector{
		configFile:        configFile,
		initialDelay:      10 * time.Second,
		backoffMultiplier: 2.0,
		jitterFactor:      0.1,
		shutdown:          make(chan struct{}),
	}
}

// Run runs the node connection with automatic reconnection
func (r *Reconnector) Run(createConn func() *Conn) {
	attempt := int64(0)

	for {
		// Check for shutdown BEFORE attempting connection
		select {
		case <-r.shutdown:
			log.Printf("[Node] Shutdown requested, exiting reconnection loop")
			return
		default:
		}

		attempt++

		if attempt > 1 {
			log.Printf("[Node] Connection attempt %d", attempt)
		}

		conn := createConn()
		err := conn.Run()

		// Check if we received shutdown signal FIRST (before processing error)
		select {
		case <-r.shutdown:
			if err == nil {
				log.Printf("[Node] Clean shutdown")
			} else {
				log.Printf("[Node] Shutdown requested, exiting reconnection loop")
			}
			return
		default:
		}

		// No shutdown, process the error
		if err == nil {
			log.Printf("[Node] Clean shutdown")
			return
		}

		log.Printf("[Node] Connection error: %v", err)
		log.Printf("[Node] Checking if error is retryable...")

		// Check if error is retryable
		if !IsRetryableError(err) {
			log.Fatalf("[Node] Non-retryable error: %v", err)
		}

		log.Printf("[Node] Error is retryable, will attempt reconnection")

		// Calculate backoff delay
		delay := r.calculateDelay(attempt)
		log.Printf("[Node] Connection failed: %v. Reconnecting in %v (attempt %d)...", err, delay, attempt)

		// Wait for delay or shutdown
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-r.shutdown:
			log.Printf("[Node] Shutdown requested during backoff, exiting")
			return
		}
	}
}

// calculateDelay calculates the backoff delay
// First 3 attempts: exponential backoff with jitter
// After 3 attempts: uniform 5-minute interval (no jitter)
func (r *Reconnector) calculateDelay(attempt int64) time.Duration {
	// First 3 attempts: exponential backoff with jitter
	if attempt <= 3 {
		delay := float64(r.initialDelay) * math.Pow(r.backoffMultiplier, float64(attempt-1))
		jitter := 1.0 + (rand.Float64()*2-1)*r.jitterFactor
		delay *= jitter
		return time.Duration(delay)
	}
	// After 3 attempts: uniform 5-minute interval (no jitter)
	return 5 * time.Minute
}

// Close signals the reconnector to stop
func (r *Reconnector) Close() {
	select {
	case <-r.shutdown:
		// Already closed
	default:
		close(r.shutdown)
	}
}

// IsRetryableError determines if an error should trigger a reconnection attempt
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Non-retryable errors (authentication/configuration issues)
	nonRetryablePatterns := []string{
		"invalid_token",
		"invalid relay address",
		"unauthorized",
		"forbidden",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(pattern)) {
			return false
		}
	}

	// Retryable errors (network/connection issues)
	retryablePatterns := []string{
		"connection refused",
		"dial",
		"EOF",
		"read message",
		"write message",
		"timeout",
		"network",
		"closed",
		"reset",
		"broken pipe",
		"no route to host",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(pattern)) {
			return true
		}
	}

	// Default to non-retryable for unknown errors
	return false
}
