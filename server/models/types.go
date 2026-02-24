package models

// Config holds the configuration for the ModelInfo client
type Config struct {
	BaseURL string
	APIKey  string
}

// ModelInfo holds information about a model
type ModelInfo struct {
	ID          string
	MaxModelLen int
}
