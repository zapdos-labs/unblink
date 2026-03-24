package server

import (
	"os"
	"strings"
)

// ProviderPreset defines default settings for a known LLM provider.
type ProviderPreset struct {
	Name         string // Display name
	BaseURL      string // OpenAI-compatible API base URL
	ChatModel    string // Default chat model
	VLMModel     string // Default vision model (falls back to ChatModel)
	MaxTokens    int    // Default context window size
	APIKeyEnvVar string // Environment variable name for the API key
}

// KnownProviders maps provider identifiers to their presets.
var KnownProviders = map[string]ProviderPreset{
	"openai": {
		Name:         "OpenAI",
		BaseURL:      "https://api.openai.com/v1",
		ChatModel:    "gpt-4o",
		VLMModel:     "gpt-4o",
		MaxTokens:    128000,
		APIKeyEnvVar: "OPENAI_API_KEY",
	},
	"minimax": {
		Name:         "MiniMax",
		BaseURL:      "https://api.minimax.io/v1",
		ChatModel:    "MiniMax-M2.7",
		VLMModel:     "MiniMax-M2.7",
		MaxTokens:    1000000,
		APIKeyEnvVar: "MINIMAX_API_KEY",
	},
}

// ResolveProvider detects the configured provider from LLM_PROVIDER env var
// or auto-detects based on available API keys.
// Returns the preset and the resolved API key.
func ResolveProvider() (ProviderPreset, string) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_PROVIDER")))

	if provider != "" {
		if preset, ok := KnownProviders[provider]; ok {
			apiKey := os.Getenv(preset.APIKeyEnvVar)
			// Also check the generic key as fallback
			if apiKey == "" {
				apiKey = os.Getenv("CHAT_OPENAI_API_KEY")
			}
			return preset, apiKey
		}
	}

	// Auto-detect: check provider-specific API keys in priority order.
	if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		return KnownProviders["minimax"], key
	}

	// Default to OpenAI preset
	return KnownProviders["openai"], os.Getenv("OPENAI_API_KEY")
}

// ApplyProviderDefaults merges provider preset values into the config,
// without overriding any values explicitly set by the user.
func ApplyProviderDefaults(cfg *Config, preset ProviderPreset, apiKey string) {
	if cfg.ChatOpenAIBaseURL == "" || cfg.ChatOpenAIBaseURL == "https://api.openai.com/v1" {
		if preset.BaseURL != "" {
			cfg.ChatOpenAIBaseURL = preset.BaseURL
		}
	}
	if cfg.ChatOpenAIModel == "" || cfg.ChatOpenAIModel == "gpt-4o" {
		if preset.ChatModel != "" {
			cfg.ChatOpenAIModel = preset.ChatModel
		}
	}
	if cfg.ChatOpenAIAPIKey == "" && apiKey != "" {
		cfg.ChatOpenAIAPIKey = apiKey
	}
	if cfg.ChatMaxTokens <= 0 || cfg.ChatMaxTokens == 128000 {
		if preset.MaxTokens > 0 {
			cfg.ChatMaxTokens = preset.MaxTokens
		}
	}
}
