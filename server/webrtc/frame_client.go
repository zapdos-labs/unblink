package webrtc

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"unb/server/models"
)

// FrameClient handles communication with vLLM endpoints (OpenAI-compatible)
type FrameClient struct {
	client      *openai.Client
	Model       string
	timeout     time.Duration
	baseURL     string
	instruction string // Static instruction for all requests
	ModelCache  *models.Registry
}

// NewFrameClient creates a new frame client for vLLM communication
func NewFrameClient(baseURL, Model, apiKey string, timeout time.Duration, instruction string, ModelCache *models.Registry) *FrameClient {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	return &FrameClient{
		client:      &client,
		Model:       Model,
		timeout:     timeout,
		baseURL:     baseURL,
		instruction: instruction,
		ModelCache:  ModelCache,
	}
}

// SendFrameBatch sends a batch of frames to the vLLM endpoint using the static instruction
func (c *FrameClient) SendFrameBatch(ctx context.Context, frames []*Frame) (*openai.ChatCompletion, error) {
	return c.SendFrameBatchWithInstruction(ctx, frames, c.instruction)
}

// SendFrameBatchWithInstruction sends a batch of frames to the vLLM endpoint with a custom instruction
func (c *FrameClient) SendFrameBatchWithInstruction(ctx context.Context, frames []*Frame, instruction string) (*openai.ChatCompletion, error) {
	return c.SendFrameBatchWithStructuredOutput(ctx, frames, instruction)
}

// SendFrameBatchWithStructuredOutput sends frames with structured output using response_format
func (c *FrameClient) SendFrameBatchWithStructuredOutput(ctx context.Context, frames []*Frame, instruction string) (*openai.ChatCompletion, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames to send")
	}

	// Build message content with images and text
	content := []openai.ChatCompletionContentPartUnionParam{}

	// Add instruction
	if instruction != "" {
		content = append(content, openai.TextContentPart(instruction))
	}

	// Add images
	for _, frame := range frames {
		// Encode JPEG to base64
		base64Data := base64.StdEncoding.EncodeToString(frame.Data)
		dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", base64Data)

		content = append(content, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: dataURL,
		}))
	}

	// Determine max tokens from cache
	maxTokens := 2000
	if c.ModelCache != nil {
		if cachedTokens, err := c.ModelCache.GetMaxTokens(c.Model); err == nil {
			// Use 25% of max context for response
			maxTokens = min(cachedTokens/4, 2000)
		}
	}

	// Build JSON schema for structured output
	schema := GenerateVLMResponseSchema()
	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "vlm_response",
		Description: openai.String("Video frame analysis with detected objects and detailed description"),
		Schema:      schema,
		Strict:      openai.Bool(true),
	}

	// Build request with structured output
	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(c.Model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(content),
		},
		MaxTokens: openai.Int(int64(maxTokens)),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		},
	}

	// Send request with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	startTime := time.Now()
	response, err := c.client.Chat.Completions.New(timeoutCtx, params)
	if err != nil {
		return nil, fmt.Errorf("vLLM request failed: %w", err)
	}
	duration := time.Since(startTime)

	log.Printf("[FrameClient] Request successful: duration=%v, frames=%d, tokens=%d",
		duration, len(frames), response.Usage.TotalTokens)

	if len(response.Choices) > 0 {
		log.Printf("[FrameClient] Response content: %s", response.Choices[0].Message.Content)
	}

	return response, nil
}
