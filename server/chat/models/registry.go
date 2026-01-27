package models

import (
	"fmt"
	"log"
	"sync"

	"unb/server/models"
)

// ModelConfig defines a model's configuration
type ModelConfig struct {
	ModelID string
	BaseURL string
	APIKey  string
}

// ModelEntry holds config and fetched info for a model
type ModelEntry struct {
	Config ModelConfig
	Info   *models.ModelInfo
}

// Registry holds model configurations and fetched info
type Registry struct {
	mu     sync.RWMutex
	models map[string]*ModelEntry // Key: modelID
}

// NewRegistry creates a new registry and fetches all model info in parallel
func NewRegistry(configs []ModelConfig) *Registry {
	r := &Registry{
		models: make(map[string]*ModelEntry),
	}

	// Create clients and fetch info in parallel
	var wg sync.WaitGroup
	for _, cfg := range configs {
		r.models[cfg.ModelID] = &ModelEntry{Config: cfg}

		wg.Add(1)
		go func(mc ModelConfig) {
			defer wg.Done()

			client := models.NewClient(models.Config{
				BaseURL: mc.BaseURL,
				APIKey:  mc.APIKey,
			})

			info, err := client.GetModelInfo(mc.ModelID)
			if err != nil {
				log.Printf("[models.Registry] Failed to fetch %s: %v", mc.ModelID, err)
				return
			}

			r.mu.Lock()
			r.models[mc.ModelID].Info = info
			r.mu.Unlock()

			log.Printf("[models.Registry] Cached %s: max_tokens=%d", mc.ModelID, info.MaxModelLen)
		}(cfg)
	}
	wg.Wait()

	return r
}

// GetModelInfo returns cached model info
func (r *Registry) GetModelInfo(modelID string) (*models.ModelInfo, error) {
	r.mu.RLock()
	entry, exists := r.models[modelID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model %q not found in registry", modelID)
	}
	if entry.Info == nil {
		return nil, fmt.Errorf("model %q info not available (fetch failed?)", modelID)
	}
	return entry.Info, nil
}

// GetConfig returns the config for a model
func (r *Registry) GetConfig(modelID string) (ModelConfig, error) {
	r.mu.RLock()
	entry, exists := r.models[modelID]
	r.mu.RUnlock()

	if !exists {
		return ModelConfig{}, fmt.Errorf("model %q not found in registry", modelID)
	}
	return entry.Config, nil
}

// GetMaxTokens returns max context length for a model
func (r *Registry) GetMaxTokens(modelID string) (int, error) {
	info, err := r.GetModelInfo(modelID)
	if err != nil {
		return 0, err
	}
	return info.MaxModelLen, nil
}

// GetMaxTokensOr returns max tokens or fallback if not found
func (r *Registry) GetMaxTokensOr(modelID string, fallback int) int {
	if info, err := r.GetModelInfo(modelID); err == nil {
		return info.MaxModelLen
	}
	return fallback
}
