package models

import (
	"log"
	"sync"
)

// Cache stores model information (cached indefinitely)
type Cache struct {
	mu     sync.RWMutex
	models map[string]*ModelInfo
	client *Client
}

// NewCache creates a new model info cache
func NewCache(client *Client) *Cache {
	return &Cache{
		models: make(map[string]*ModelInfo),
		client: client,
	}
}

// GetModelInfo returns model info from cache, fetching if needed
func (c *Cache) GetModelInfo(modelID string) (*ModelInfo, error) {
	// Try to get from cache first
	c.mu.RLock()
	info, exists := c.models[modelID]
	c.mu.RUnlock()

	if exists {
		return info, nil
	}

	// Cache miss, fetch fresh data
	return c.fetchAndCache(modelID)
}

// fetchAndCache fetches model info from the API and caches it
func (c *Cache) fetchAndCache(modelID string) (*ModelInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if info, exists := c.models[modelID]; exists {
		return info, nil
	}

	info, err := c.client.GetModelInfo(modelID)
	if err != nil {
		return nil, err
	}

	c.models[modelID] = info

	// Probe image dimensions asynchronously (fire-and-forget)
	go c.probeDimensionsAsync(modelID)

	log.Printf("[models.Cache] Cached model info for %s: max_tokens=%d", modelID, info.MaxModelLen)

	return info, nil
}

// probeDimensionsAsync probes the model for image dimensions and updates the cache
func (c *Cache) probeDimensionsAsync(modelID string) {
	width, height, err := c.client.ProbeImageDimensions(modelID)
	if err != nil {
		log.Printf("[models.Cache] Failed to probe dimensions for %s: %v", modelID, err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if info, exists := c.models[modelID]; exists {
		info.EffectiveWidth = &width
		info.EffectiveHeight = &height
		log.Printf("[models.Cache] Updated %s: effective_image=%dx%d", modelID, width, height)
	}
}

// GetMaxTokens returns the maximum context length for a model
func (c *Cache) GetMaxTokens(modelID string) (int, error) {
	info, err := c.GetModelInfo(modelID)
	if err != nil {
		return 0, err
	}
	return info.MaxModelLen, nil
}
