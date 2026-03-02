package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config represents the server configuration
// All fields are required - the application will fail to start if any are missing
type Config struct {
	// Server settings
	ListenAddr string `json:"listen_addr"`

	// Dashboard URL (for logging/redirects)
	DashboardURL string `json:"dashboard_url"`

	// Database settings
	DatabaseURL string `json:"database_url"`

	// JWT secret for authentication
	JWTSecret string `json:"jwt_secret"`

	// Main chat model settings
	ChatOpenAIModel   string `json:"chat_openai_model"`
	ChatOpenAIBaseURL string `json:"chat_openai_base_url"`
	ChatOpenAIAPIKey  string `json:"chat_openai_api_key,omitempty"`

	// Content trimming safety margin (percentage)
	ContentTrimSafetyMargin int `json:"content_trim_safety_margin"`
	ChatMaxTokens           int `json:"chat_max_tokens"`

	// Frame extraction settings
	FrameIntervalSeconds float64 `json:"frame_interval_seconds"` // Extraction interval in seconds
	FrameBatchSize       int     `json:"frame_batch_size"`       // Frames to batch before sending (buffer size = batch size)

	// VLM OpenAI settings for frame processing
	VLMOpenAIModel   string `json:"vlm_openai_model"`
	VLMOpenAIBaseURL string `json:"vlm_openai_base_url"`
	VLMOpenAIAPIKey  string `json:"vlm_openai_api_key,omitempty"`
	VLMTimeoutSec    int    `json:"vlm_timeout_sec"` // Request timeout in seconds

	// Bridge idle detection and reconnection
	BridgeIdleTimeoutSec int `json:"bridge_idle_timeout_sec"` // How long before bridge is considered idle (seconds)
	BridgeMaxRetries     int `json:"bridge_max_retries"`      // Maximum reconnection attempts before giving up

	// App directory for storage (frames, logs, etc.)
	AppDir string `json:"app_dir"` // Path to application storage directory

	// Frontend dist directory (for serving static files)
	DistPath string `json:"dist_path,omitempty"` // Path to frontend dist directory (optional)

	// Frame indexing settings
	EnableIndexing bool `json:"enable_indexing"` // Enable frame indexing (default true). When true, batch manager is not created.
}

func (c *Config) applyModelFallbacks() {
	if strings.TrimSpace(c.VLMOpenAIModel) == "" {
		c.VLMOpenAIModel = c.ChatOpenAIModel
	}
	if strings.TrimSpace(c.VLMOpenAIBaseURL) == "" {
		c.VLMOpenAIBaseURL = c.ChatOpenAIBaseURL
	}
	if strings.TrimSpace(c.VLMOpenAIAPIKey) == "" {
		c.VLMOpenAIAPIKey = c.ChatOpenAIAPIKey
	}
	if c.ChatMaxTokens <= 0 {
		c.ChatMaxTokens = 128000
	}
}

// Validate checks that all required fields are present and valid
func (c *Config) Validate() error {
	var missing []string

	if c.ListenAddr == "" {
		missing = append(missing, "listen_addr")
	}
	if c.DashboardURL == "" {
		missing = append(missing, "dashboard_url")
	}
	if c.DatabaseURL == "" {
		missing = append(missing, "database_url")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "jwt_secret")
	}
	if c.ChatOpenAIModel == "" {
		missing = append(missing, "chat_openai_model")
	}
	if c.ChatOpenAIBaseURL == "" {
		missing = append(missing, "chat_openai_base_url")
	}
	if c.VLMOpenAIModel == "" {
		missing = append(missing, "vlm_openai_model")
	}
	if c.VLMOpenAIBaseURL == "" {
		missing = append(missing, "vlm_openai_base_url")
	}
	if c.AppDir == "" {
		missing = append(missing, "app_dir")
	}
	if c.ContentTrimSafetyMargin <= 0 {
		missing = append(missing, "content_trim_safety_margin")
	}
	if c.ChatMaxTokens <= 0 {
		missing = append(missing, "chat_max_tokens")
	}
	if c.FrameIntervalSeconds <= 0 {
		missing = append(missing, "frame_interval_seconds")
	}
	if c.FrameBatchSize <= 0 {
		missing = append(missing, "frame_batch_size")
	}
	if c.VLMTimeoutSec <= 0 {
		missing = append(missing, "vlm_timeout_sec")
	}
	if c.BridgeIdleTimeoutSec <= 0 {
		missing = append(missing, "bridge_idle_timeout_sec")
	}
	if c.BridgeMaxRetries <= 0 {
		missing = append(missing, "bridge_max_retries")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %v", missing)
	}

	// Validate listen_addr format (basic check)
	if c.ListenAddr[0] != ':' && len(c.ListenAddr) < 3 {
		return errors.New("listen_addr must be in format ':port' or 'host:port'")
	}

	return nil
}

// FramesBaseDir returns the base directory for storing extracted frames (without serviceID)
func (c *Config) FramesBaseDir() string {
	if c.AppDir == "" {
		// Default to current directory if not set
		return filepath.Join("storage", "frames")
	}
	return filepath.Join(c.AppDir, "storage", "frames")
}

// FramesDir returns the directory path for storing extracted frames for a service
func (c *Config) FramesDir(serviceID string) string {
	if c.AppDir == "" {
		// Default to current directory if not set
		return filepath.Join("storage", "frames", serviceID)
	}
	return filepath.Join(c.AppDir, "storage", "frames", serviceID)
}

// LoadConfig loads server configuration from environment variables.
// Environment variables:
//   - VITE_SERVER_API_PORT: Server listen address (default: :8080)
//   - DATABASE_URL: PostgreSQL connection string
//   - JWT_SECRET: JWT signing secret
//   - DASHBOARD_URL: Dashboard URL for redirects
//   - CHAT_OPENAI_MODEL: OpenAI model ID for chat
//   - CHAT_OPENAI_BASE_URL: OpenAI API base URL for chat
//   - CHAT_OPENAI_API_KEY: OpenAI API key for chat (optional)
//   - CHAT_MAX_TOKENS: Chat model context length for trimming (default: 128000)
//   - VLM_OPENAI_MODEL: OpenAI model ID for vision (falls back to CHAT_OPENAI_MODEL)
//   - VLM_OPENAI_BASE_URL: OpenAI API base URL for vision (falls back to CHAT_OPENAI_BASE_URL)
//   - VLM_OPENAI_API_KEY: OpenAI API key for vision (falls back to CHAT_OPENAI_API_KEY)
//   - VLM_TIMEOUT_SEC: VLM request timeout in seconds (default: 120)
//   - CONTENT_TRIM_SAFETY_MARGIN: Content trimming safety margin % (default: 10)
//   - FRAME_INTERVAL_SECONDS: Frame extraction interval (default: 5.0)
//   - FRAME_BATCH_SIZE: Frame batch size (default: 3)
//   - BRIDGE_IDLE_TIMEOUT_SEC: Bridge idle timeout (default: 300)
//   - BRIDGE_MAX_RETRIES: Bridge max retries (default: 3)
//   - ENABLE_INDEXING: Enable frame indexing (default: true)
//   - CONFIG_DIR: Application storage directory (default: ~/.unblink)
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Server settings (web-agent style: single source of truth)
	cfg.ListenAddr = getEnv("VITE_SERVER_API_PORT", "8080")
	if !strings.HasPrefix(cfg.ListenAddr, ":") {
		cfg.ListenAddr = ":" + cfg.ListenAddr
	}

	// Dashboard URL
	cfg.DashboardURL = getEnv("DASHBOARD_URL", "http://localhost:8080")

	// Database URL (required - caller should check this exists)
	cfg.DatabaseURL = os.Getenv("DATABASE_URL")

	// JWT Secret (required - caller should check this exists)
	cfg.JWTSecret = os.Getenv("JWT_SECRET")

	// Chat model settings
	cfg.ChatOpenAIModel = getEnv("CHAT_OPENAI_MODEL", "gpt-4o")
	cfg.ChatOpenAIBaseURL = getEnv("CHAT_OPENAI_BASE_URL", "https://api.openai.com/v1")
	cfg.ChatOpenAIAPIKey = os.Getenv("CHAT_OPENAI_API_KEY")
	cfg.ChatMaxTokens = getEnvInt("CHAT_MAX_TOKENS", 128000)

	// VLM settings
	cfg.VLMOpenAIModel = os.Getenv("VLM_OPENAI_MODEL")
	cfg.VLMOpenAIBaseURL = os.Getenv("VLM_OPENAI_BASE_URL")
	cfg.VLMOpenAIAPIKey = os.Getenv("VLM_OPENAI_API_KEY")
	cfg.applyModelFallbacks()
	cfg.VLMTimeoutSec = getEnvInt("VLM_TIMEOUT_SEC", 120)

	// Content trimming
	cfg.ContentTrimSafetyMargin = getEnvInt("CONTENT_TRIM_SAFETY_MARGIN", 10)

	// Frame extraction
	cfg.FrameIntervalSeconds = getEnvFloat("FRAME_INTERVAL_SECONDS", 5.0)
	cfg.FrameBatchSize = getEnvInt("FRAME_BATCH_SIZE", 3)

	// Bridge settings
	cfg.BridgeIdleTimeoutSec = getEnvInt("BRIDGE_IDLE_TIMEOUT_SEC", 300)
	cfg.BridgeMaxRetries = getEnvInt("BRIDGE_MAX_RETRIES", 3)

	// Indexing
	cfg.EnableIndexing = getEnvBool("ENABLE_INDEXING", true)

	// Configuration directory (storage root)
	cfg.AppDir = getEnv("CONFIG_DIR", defaultConfigDir())

	// Frontend dist directory (optional - if not set, frontend is not served)
	cfg.DistPath = os.Getenv("DIST_PATH")

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid environment config: %w", err)
	}

	return cfg, nil
}

// Helper functions for reading environment variables with defaults

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func defaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".unblink")
}
