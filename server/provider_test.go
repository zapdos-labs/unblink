package server

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"LLM_PROVIDER", "MINIMAX_API_KEY", "OPENAI_API_KEY",
		"CHAT_OPENAI_MODEL", "CHAT_OPENAI_BASE_URL", "CHAT_OPENAI_API_KEY",
		"CHAT_MAX_TOKENS",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func TestResolveProvider_ExplicitMiniMax(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("LLM_PROVIDER", "minimax")
	t.Setenv("MINIMAX_API_KEY", "test-minimax-key")

	preset, apiKey := ResolveProvider()
	assert.Equal(t, "MiniMax", preset.Name)
	assert.Equal(t, "https://api.minimax.io/v1", preset.BaseURL)
	assert.Equal(t, "MiniMax-M2.7", preset.ChatModel)
	assert.Equal(t, "MiniMax-M2.7", preset.VLMModel)
	assert.Equal(t, 1000000, preset.MaxTokens)
	assert.Equal(t, "test-minimax-key", apiKey)
}

func TestResolveProvider_ExplicitOpenAI(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	preset, apiKey := ResolveProvider()
	assert.Equal(t, "OpenAI", preset.Name)
	assert.Equal(t, "gpt-4o", preset.ChatModel)
	assert.Equal(t, "test-openai-key", apiKey)
}

func TestResolveProvider_AutoDetectMiniMax(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MINIMAX_API_KEY", "auto-detected-key")

	preset, apiKey := ResolveProvider()
	assert.Equal(t, "MiniMax", preset.Name)
	assert.Equal(t, "auto-detected-key", apiKey)
}

func TestResolveProvider_DefaultsToOpenAI(t *testing.T) {
	clearProviderEnv(t)

	preset, _ := ResolveProvider()
	assert.Equal(t, "OpenAI", preset.Name)
}

func TestResolveProvider_CaseInsensitive(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("LLM_PROVIDER", "MiniMax")
	t.Setenv("MINIMAX_API_KEY", "key")

	preset, _ := ResolveProvider()
	assert.Equal(t, "MiniMax", preset.Name)
}

func TestResolveProvider_FallbackToChatKey(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("LLM_PROVIDER", "minimax")
	t.Setenv("CHAT_OPENAI_API_KEY", "chat-key-fallback")

	preset, apiKey := ResolveProvider()
	assert.Equal(t, "MiniMax", preset.Name)
	assert.Equal(t, "chat-key-fallback", apiKey)
}

func TestApplyProviderDefaults_SetsEmptyFields(t *testing.T) {
	cfg := &Config{}
	preset := KnownProviders["minimax"]

	ApplyProviderDefaults(cfg, preset, "my-key")

	assert.Equal(t, "https://api.minimax.io/v1", cfg.ChatOpenAIBaseURL)
	assert.Equal(t, "MiniMax-M2.7", cfg.ChatOpenAIModel)
	assert.Equal(t, "my-key", cfg.ChatOpenAIAPIKey)
	assert.Equal(t, 1000000, cfg.ChatMaxTokens)
}

func TestApplyProviderDefaults_DoesNotOverrideExplicitValues(t *testing.T) {
	cfg := &Config{
		ChatOpenAIBaseURL: "https://custom.api.example.com/v1",
		ChatOpenAIModel:   "custom-model",
		ChatOpenAIAPIKey:  "explicit-key",
		ChatMaxTokens:     256000,
	}
	preset := KnownProviders["minimax"]

	ApplyProviderDefaults(cfg, preset, "provider-key")

	assert.Equal(t, "https://custom.api.example.com/v1", cfg.ChatOpenAIBaseURL)
	assert.Equal(t, "custom-model", cfg.ChatOpenAIModel)
	assert.Equal(t, "explicit-key", cfg.ChatOpenAIAPIKey)
	assert.Equal(t, 256000, cfg.ChatMaxTokens)
}

func TestApplyProviderDefaults_OverridesOpenAIDefaults(t *testing.T) {
	cfg := &Config{
		ChatOpenAIBaseURL: "https://api.openai.com/v1",
		ChatOpenAIModel:   "gpt-4o",
		ChatMaxTokens:     128000,
	}
	preset := KnownProviders["minimax"]

	ApplyProviderDefaults(cfg, preset, "mm-key")

	assert.Equal(t, "https://api.minimax.io/v1", cfg.ChatOpenAIBaseURL)
	assert.Equal(t, "MiniMax-M2.7", cfg.ChatOpenAIModel)
	assert.Equal(t, 1000000, cfg.ChatMaxTokens)
}

func TestKnownProviders_AllPresetsValid(t *testing.T) {
	for name, preset := range KnownProviders {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, preset.Name, "Name should not be empty")
			assert.NotEmpty(t, preset.BaseURL, "BaseURL should not be empty")
			assert.NotEmpty(t, preset.ChatModel, "ChatModel should not be empty")
			assert.Greater(t, preset.MaxTokens, 0, "MaxTokens should be positive")
			assert.NotEmpty(t, preset.APIKeyEnvVar, "APIKeyEnvVar should not be empty")
		})
	}
}

func TestKnownProviders_MiniMaxPreset(t *testing.T) {
	preset, ok := KnownProviders["minimax"]
	require.True(t, ok, "minimax preset should exist")
	assert.Equal(t, "MiniMax", preset.Name)
	assert.Equal(t, "https://api.minimax.io/v1", preset.BaseURL)
	assert.Equal(t, "MiniMax-M2.7", preset.ChatModel)
	assert.Equal(t, "MiniMax-M2.7", preset.VLMModel)
	assert.Equal(t, 1000000, preset.MaxTokens)
	assert.Equal(t, "MINIMAX_API_KEY", preset.APIKeyEnvVar)
}

func TestLoadConfig_MiniMaxProvider(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("LLM_PROVIDER", "minimax")
	t.Setenv("MINIMAX_API_KEY", "test-key")
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	t.Setenv("JWT_SECRET", "test-secret")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "MiniMax-M2.7", cfg.ChatOpenAIModel)
	assert.Equal(t, "https://api.minimax.io/v1", cfg.ChatOpenAIBaseURL)
	assert.Equal(t, "test-key", cfg.ChatOpenAIAPIKey)
	assert.Equal(t, 1000000, cfg.ChatMaxTokens)

	// VLM should fall back to chat settings
	assert.Equal(t, "MiniMax-M2.7", cfg.VLMOpenAIModel)
	assert.Equal(t, "https://api.minimax.io/v1", cfg.VLMOpenAIBaseURL)
	assert.Equal(t, "test-key", cfg.VLMOpenAIAPIKey)
}

func TestLoadConfig_MiniMaxAutoDetect(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MINIMAX_API_KEY", "auto-key")
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	t.Setenv("JWT_SECRET", "test-secret")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "MiniMax-M2.7", cfg.ChatOpenAIModel)
	assert.Equal(t, "https://api.minimax.io/v1", cfg.ChatOpenAIBaseURL)
	assert.Equal(t, "auto-key", cfg.ChatOpenAIAPIKey)
}

func TestLoadConfig_MiniMaxWithVLMOverride(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("LLM_PROVIDER", "minimax")
	t.Setenv("MINIMAX_API_KEY", "mm-key")
	t.Setenv("VLM_OPENAI_MODEL", "custom-vlm")
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	t.Setenv("JWT_SECRET", "test-secret")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "MiniMax-M2.7", cfg.ChatOpenAIModel)
	assert.Equal(t, "custom-vlm", cfg.VLMOpenAIModel)
	assert.Equal(t, "https://api.minimax.io/v1", cfg.VLMOpenAIBaseURL)
}

func TestLoadConfig_ExplicitOverridesProvider(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("LLM_PROVIDER", "minimax")
	t.Setenv("MINIMAX_API_KEY", "mm-key")
	t.Setenv("CHAT_OPENAI_MODEL", "my-custom-model")
	t.Setenv("CHAT_OPENAI_BASE_URL", "https://custom.api.example.com/v1")
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	t.Setenv("JWT_SECRET", "test-secret")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "my-custom-model", cfg.ChatOpenAIModel)
	assert.Equal(t, "https://custom.api.example.com/v1", cfg.ChatOpenAIBaseURL)
}
