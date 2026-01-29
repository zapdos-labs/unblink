package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// OpenAI-compatible chat completion request/response structures
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type ResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index   int             `json:"index"`
	Message ResponseMessage `json:"message"`
}

// Test configuration
var (
	baseURL        = flag.String("url", "https://vllm.unblink.net/v1", "VLM base URL")
	model          = flag.String("model", "Qwen/Qwen3-VL-4B-Instruct", "Model name")
	apiKey         = flag.String("apikey", "", "API key for authentication (optional)")
	concurrency    = flag.Int("concurrency", 10, "Number of concurrent requests")
	totalRequests  = flag.Int("requests", 100, "Total number of requests to send")
	imageWidth     = flag.Int("width", 640, "Test image width")
	imageHeight    = flag.Int("height", 480, "Test image height")
	timeout        = flag.Duration("timeout", 60*time.Second, "Request timeout")
)

// Statistics
type Stats struct {
	totalRequests   atomic.Int64
	successRequests atomic.Int64
	failedRequests  atomic.Int64
	totalLatency    atomic.Int64 // in milliseconds
	minLatency      atomic.Int64 // in milliseconds
	maxLatency      atomic.Int64 // in milliseconds
}

func (s *Stats) recordSuccess(latency time.Duration) {
	s.successRequests.Add(1)
	s.totalRequests.Add(1)

	latencyMs := latency.Milliseconds()
	s.totalLatency.Add(latencyMs)

	// Update min latency
	for {
		current := s.minLatency.Load()
		if current == 0 || latencyMs < current {
			if s.minLatency.CompareAndSwap(current, latencyMs) {
				break
			}
		} else {
			break
		}
	}

	// Update max latency
	for {
		current := s.maxLatency.Load()
		if latencyMs > current {
			if s.maxLatency.CompareAndSwap(current, latencyMs) {
				break
			}
		} else {
			break
		}
	}
}

func (s *Stats) recordFailure() {
	s.failedRequests.Add(1)
	s.totalRequests.Add(1)
}

func (s *Stats) print(duration time.Duration) {
	total := s.totalRequests.Load()
	success := s.successRequests.Load()
	failed := s.failedRequests.Load()

	fmt.Printf("\n=== Test Results ===\n")
	fmt.Printf("Total Requests:   %d\n", total)
	fmt.Printf("Successful:       %d (%.2f%%)\n", success, float64(success)/float64(total)*100)
	fmt.Printf("Failed:           %d (%.2f%%)\n", failed, float64(failed)/float64(total)*100)
	fmt.Printf("\n")
	fmt.Printf("Test Duration:    %v\n", duration.Round(time.Millisecond))
	fmt.Printf("Throughput:       %.2f req/s\n", float64(success)/duration.Seconds())
	fmt.Printf("\n")

	if success > 0 {
		avgLatency := s.totalLatency.Load() / success
		fmt.Printf("Latency Stats (ms):\n")
		fmt.Printf("  Min:            %d ms\n", s.minLatency.Load())
		fmt.Printf("  Max:            %d ms\n", s.maxLatency.Load())
		fmt.Printf("  Avg:            %d ms\n", avgLatency)
	}
}

// generateTestImage creates a simple test image with random colors
func generateTestImage(width, height int) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8(x * 255 / width)
			g := uint8(y * 255 / height)
			b := uint8((x + y) * 255 / (width + height))
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	// Add some shapes for the VLM to detect
	// Draw a rectangle
	for x := width/4; x < width*3/4; x++ {
		for y := height/4; y < height*3/4; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	// Encode to JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	// Convert to base64 data URL
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/jpeg;base64," + encoded, nil
}

// sendRequest sends a single request to the VLM endpoint
func sendRequest(ctx context.Context, client *http.Client, imageURL string, stats *Stats) {
	start := time.Now()

	// Build request with content as array of parts
	type RequestMessage struct {
		Role    string        `json:"role"`
		Content []ContentPart `json:"content"`
	}

	type RequestPayload struct {
		Model     string           `json:"model"`
		Messages  []RequestMessage `json:"messages"`
		MaxTokens int              `json:"max_tokens,omitempty"`
	}

	req := RequestPayload{
		Model: *model,
		Messages: []RequestMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{
						Type: "text",
						Text: "Analyze this image. Describe what you see and detect any objects.",
					},
					{
						Type: "image_url",
						ImageURL: &ImageURL{
							URL: imageURL,
						},
					},
				},
			},
		},
		MaxTokens: 500,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		log.Printf("Failed to marshal request: %v", err)
		stats.recordFailure()
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", *baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		stats.recordFailure()
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if *apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+*apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Request failed: %v", err)
		stats.recordFailure()
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response: %v", err)
		stats.recordFailure()
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Request failed with status %d: %s", resp.StatusCode, string(body))
		stats.recordFailure()
		return
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Failed to parse response: %v", err)
		stats.recordFailure()
		return
	}

	latency := time.Since(start)
	stats.recordSuccess(latency)

	// Print first few responses for verification
	if stats.successRequests.Load() <= 3 {
		if len(result.Choices) > 0 {
			content := result.Choices[0].Message.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			fmt.Printf("\nSample response #%d (latency: %v):\n%s\n",
				stats.successRequests.Load(),
				latency.Round(time.Millisecond),
				content)
		}
	}
}

func main() {
	flag.Parse()

	fmt.Printf("=== VLM Throughput Test ===\n")
	fmt.Printf("Base URL:      %s\n", *baseURL)
	fmt.Printf("Model:         %s\n", *model)
	fmt.Printf("Concurrency:   %d\n", *concurrency)
	fmt.Printf("Total Requests: %d\n", *totalRequests)
	fmt.Printf("Image Size:    %dx%d\n", *imageWidth, *imageHeight)
	fmt.Printf("Timeout:       %v\n", *timeout)
	fmt.Printf("\n")

	// Generate test image
	fmt.Printf("Generating test image...\n")
	imageURL, err := generateTestImage(*imageWidth, *imageHeight)
	if err != nil {
		log.Fatalf("Failed to generate test image: %v", err)
	}
	fmt.Printf("Test image generated (%d bytes encoded)\n\n", len(imageURL))

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: *timeout,
	}

	// Initialize stats
	stats := &Stats{}
	stats.minLatency.Store(1<<63 - 1) // Max int64

	// Create worker pool
	semaphore := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup

	fmt.Printf("Starting test...\n")
	testStart := time.Now()

	// Send requests
	for i := 0; i < *totalRequests; i++ {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(reqNum int) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			defer cancel()

			sendRequest(ctx, client, imageURL, stats)

			// Progress indicator
			if reqNum%10 == 0 {
				fmt.Printf(".")
			}
		}(i)
	}

	// Wait for all requests to complete
	wg.Wait()
	testDuration := time.Since(testStart)

	// Print results
	fmt.Printf("\n\nTest completed!\n")
	stats.print(testDuration)
}
