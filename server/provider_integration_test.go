//go:build integration

package server

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiniMaxIntegration_ChatCompletion(t *testing.T) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		t.Skip("MINIMAX_API_KEY not set, skipping integration test")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://api.minimax.io/v1"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel("MiniMax-M2.7"),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Reply with exactly: hello unblink"),
		},
		MaxTokens: openai.Int(20),
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Choices)
	assert.Contains(t, resp.Choices[0].Message.Content, "hello")
}

func TestMiniMaxIntegration_StructuredOutput(t *testing.T) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		t.Skip("MINIMAX_API_KEY not set, skipping integration test")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://api.minimax.io/v1"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel("MiniMax-M2.7"),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Return a JSON object with key 'status' set to 'ok'"),
		},
		MaxTokens: openai.Int(50),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &openai.ResponseFormatJSONObjectParam{},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Choices)
	assert.Contains(t, resp.Choices[0].Message.Content, "ok")
}

func TestMiniMaxIntegration_Streaming(t *testing.T) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		t.Skip("MINIMAX_API_KEY not set, skipping integration test")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://api.minimax.io/v1"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel("MiniMax-M2.7"),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Say hello"),
		},
		MaxTokens: openai.Int(20),
	})
	defer stream.Close()

	chunks := 0
	for stream.Next() {
		chunks++
	}
	require.NoError(t, stream.Err())
	assert.Greater(t, chunks, 0, "should receive at least one chunk")
}
