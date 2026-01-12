package relay

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds all relay configuration loaded from environment
type Config struct {
	// Base directory
	AppDir       string
	StorageDir   string // Computed: AppDir + "/storage"
	DatabasePath string // Computed: AppDir + "/database/relay.db"

	// CV Configuration
	FrameInterval time.Duration
	BatchSize     int // Number of frames per batch

	// Ports
	RelayPort string // Port for WebSocket connections (nodes + workers) (e.g., "9020")
	APIPort   string // Port for HTTP API for browsers (e.g., "8020")

	// Dashboard
	DashboardURL string

	// Realtime Streams
	AutoRequestRealtimeStream bool // Auto-create streams for RTSP/MJPEG

	// Security
	JWTSecret string // Secret for signing JWT tokens
}

// LoadConfig loads and validates all configuration from environment
func LoadConfig() (*Config, error) {
	var missingVars []string
	var errors []string

	// ALL variables are required - no defaults
	appDir := os.Getenv("APP_DIR")
	if appDir == "" {
		missingVars = append(missingVars, "APP_DIR")
	}

	frameIntervalStr := os.Getenv("FRAME_INTERVAL_SECONDS")
	if frameIntervalStr == "" {
		missingVars = append(missingVars, "FRAME_INTERVAL_SECONDS")
	}

	relayPort := os.Getenv("RELAY_PORT")
	if relayPort == "" {
		missingVars = append(missingVars, "RELAY_PORT")
	}

	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		missingVars = append(missingVars, "API_PORT")
	}

	dashboardURL := os.Getenv("DASHBOARD_URL")
	if dashboardURL == "" {
		missingVars = append(missingVars, "DASHBOARD_URL")
	}

	// Optional: Auto-request realtime streams (default: true)
	autoRequestRealtimeStream := true
	if val := os.Getenv("AUTO_REQUEST_REALTIME_STREAM"); val != "" {
		autoRequestRealtimeStream = val == "true" || val == "1"
	}

	// Optional: Batch size (default: 10)
	batchSize := 10
	if val := os.Getenv("BATCH_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			batchSize = parsed
		} else {
			errors = append(errors, fmt.Sprintf("BATCH_SIZE must be a positive number, got: %s", val))
		}
	}

	// JWT secret for token signing
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "change-me-in-production" // fallback
		log.Printf("[Config] WARNING: Using default JWT_SECRET. Set JWT_SECRET in production!")
	}

	// Stop if any required variables are missing
	if len(missingVars) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v\nPlease set them in .env file or environment", missingVars)
	}

	// Parse and validate durations
	frameIntervalSeconds, err := strconv.Atoi(frameIntervalStr)
	if err != nil {
		errors = append(errors, fmt.Sprintf("FRAME_INTERVAL_SECONDS must be a number, got: %s", frameIntervalStr))
	}
	frameInterval := time.Duration(frameIntervalSeconds) * time.Second

	// Validate port number
	if _, err := strconv.Atoi(relayPort); err != nil {
		errors = append(errors, fmt.Sprintf("RELAY_PORT must be a number, got: %s", relayPort))
	}

	if _, err := strconv.Atoi(apiPort); err != nil {
		errors = append(errors, fmt.Sprintf("API_PORT must be a number, got: %s", apiPort))
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("configuration validation errors:\n%v", errors)
	}

	// Compute paths from APP_DIR
	storageDir := filepath.Join(appDir, "storage")
	databasePath := filepath.Join(appDir, "database", "relay.db")

	config := &Config{
		AppDir:                    appDir,
		StorageDir:                storageDir,
		DatabasePath:              databasePath,
		FrameInterval:             frameInterval,
		BatchSize:                 batchSize,
		RelayPort:                 relayPort,
		APIPort:                   apiPort,
		DashboardURL:              dashboardURL,
		AutoRequestRealtimeStream: autoRequestRealtimeStream,
		JWTSecret:                 jwtSecret,
	}

	// Log loaded configuration
	log.Printf("[Config] Loaded configuration:")
	log.Printf("[Config]   APP_DIR: %s", config.AppDir)
	log.Printf("[Config]   STORAGE_DIR: %s", config.StorageDir)
	log.Printf("[Config]   DATABASE_PATH: %s", config.DatabasePath)
	log.Printf("[Config]   FRAME_INTERVAL: %v", config.FrameInterval)
	log.Printf("[Config]   RELAY_PORT: %s (nodes + workers)", config.RelayPort)
	log.Printf("[Config]   API_PORT: %s", config.APIPort)
	log.Printf("[Config]   BATCH_SIZE: %d", config.BatchSize)
	log.Printf("[Config]   DASHBOARD_URL: %s", config.DashboardURL)
	log.Printf("[Config]   AUTO_REQUEST_REALTIME_STREAM: %v", config.AutoRequestRealtimeStream)

	return config, nil
}
