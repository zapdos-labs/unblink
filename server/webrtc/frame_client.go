package webrtc

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// FrameClient handles communication with vLLM endpoints (OpenAI-compatible)
type FrameClient struct {
	client      *openai.Client
	model       string
	timeout     time.Duration
	baseURL     string
	instruction string // Static instruction for all requests
}

// NewFrameClient creates a new frame client for vLLM communication
func NewFrameClient(baseURL, model, apiKey string, timeout time.Duration, instruction string) *FrameClient {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	return &FrameClient{
		client:      &client,
		model:       model,
		timeout:     timeout,
		baseURL:     baseURL,
		instruction: instruction,
	}
}

// SendFrameBatch sends a batch of frames to the vLLM endpoint
func (c *FrameClient) SendFrameBatch(ctx context.Context, frames []*Frame) (*openai.ChatCompletion, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames to send")
	}

	// Build message content with images and text
	content := []openai.ChatCompletionContentPartUnionParam{}

	// Add static instruction
	if c.instruction != "" {
		content = append(content, openai.TextContentPart(c.instruction))
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

	// Build request
	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(c.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(content),
		},
		MaxTokens: openai.Int(500),
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
