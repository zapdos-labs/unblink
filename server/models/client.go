package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client fetches model information from OpenAI-compatible /v1/models endpoint
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// modelsResponse represents the JSON response from /v1/models
type modelsResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID         string `json:"id"`
		MaxModelLen int    `json:"max_model_len"`
	} `json:"data"`
}

// NewClient creates a new ModelInfo client
func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
	}
}

// GetModels fetches all models from the API
func (c *Client) GetModels() ([]ModelInfo, error) {
	url := c.baseURL + "/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var modelsResp modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]ModelInfo, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = ModelInfo{
			ID:          m.ID,
			MaxModelLen: m.MaxModelLen,
		}
	}

	return models, nil
}

// GetModelInfo fetches information for a specific model
func (c *Client) GetModelInfo(modelID string) (*ModelInfo, error) {
	models, err := c.GetModels()
	if err != nil {
		return nil, err
	}

	for _, m := range models {
		if m.ID == modelID {
			return &m, nil
		}
	}

	return nil, fmt.Errorf("model %q not found", modelID)
}
