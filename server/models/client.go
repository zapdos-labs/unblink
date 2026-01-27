package models

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
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

// ProbeImageDimensions sends a test image to the model to determine the effective
// dimensions it sees after internal scaling.
func (c *Client) ProbeImageDimensions(modelID string) (width, height int, err error) {
	log.Printf("[models.Client] Starting dimension probe for %s (baseURL=%s)", modelID, c.baseURL)

	// Use COCO test image via URL
	imageURL := "http://images.cocodataset.org/val2017/000000039769.jpg"

	// Create OpenAI client
	opts := []option.RequestOption{option.WithAPIKey(c.apiKey)}
	if c.baseURL != "" {
		opts = append(opts, option.WithBaseURL(c.baseURL))
	}
	client := openai.NewClient(opts...)

	// Build request - ask for JSON response
	content := []openai.ChatCompletionContentPartUnionParam{
		openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: imageURL,
		}),
		openai.TextContentPart("What are the dimensions of this image? Respond with JSON only: {\"width\": int, \"height\": int}"),
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(modelID),
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(content)},
		MaxTokens: openai.Int(int64(100)),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return 0, 0, fmt.Errorf("probe request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return 0, 0, fmt.Errorf("no response from model")
	}

	// Parse JSON response
	var result struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return 0, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("[models.Client] Probed %s: effective_image=%dx%d", modelID, result.Width, result.Height)
	return result.Width, result.Height, nil
}
