package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	// Fast model for follow-ups
	FastOpenAIModel   string `json:"fast_openai_model"`
	FastOpenAIBaseURL string `json:"fast_openai_base_url"`
	FastOpenAIAPIKey  string `json:"fast_openai_api_key,omitempty"`

	// Content trimming safety margin (percentage)
	ContentTrimSafetyMargin int `json:"content_trim_safety_margin"`

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
}

// ConfigPath returns the default config file path
func ConfigPath() (string, error) {
	// Check for server.config.json in current directory first
	localPath := "server.config.json"
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	// Fall back to home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(homeDir, ".unblink", "server.config.json"), nil
}

// LoadConfig reads and validates the server configuration from a JSON file
// It enforces that all required fields must be present
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return nil, fmt.Errorf("get config path: %w", err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	// Validate all required fields
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config in %s: %w", path, err)
	}

	return &cfg, nil
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
	if c.FastOpenAIModel == "" {
		missing = append(missing, "fast_openai_model")
	}
	if c.FastOpenAIBaseURL == "" {
		missing = append(missing, "fast_openai_base_url")
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

// Save writes the config to a file
func (c *Config) Save(path string) error {
	// Validate before saving
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
