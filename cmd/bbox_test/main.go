package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"

	"unblink/server/webrtc"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// VLMResponse represents the JSON response from VLM with structured output
type VLMResponse struct {
	Objects     []VLMObject `json:"objects" jsonschema_description:"All detected objects with bounding boxes and IDs"`
	Description string      `json:"description" jsonschema_description:"Detailed analysis of motion, action, emotion, expressions, and subtle changes. Reference objects by their IDs in brackets, e.g., 'The person [3] is driving a red car [4]'."`
}

type VLMObject struct {
	ID    int       `json:"id" jsonschema_description:"Unique identifier for this object (used for tracking across frames)"`
	Label string    `json:"label" jsonschema_description:"Label/name of the object"`
	BBox  []float64 `json:"bbox" jsonschema_description:"Bounding box as [x1, y1, x2, y2] in normalized 1000 coordinates. 0=top/left, 1000=bottom/right. For example, [250, 300, 750, 800] means the object spans from 25% to 75% horizontally and 30% to 80% vertically."`
}

func main() {
	// Configuration - from actual server config
	baseURL := "https://vllm.unblink.net/v1" // vLLM server
	apiKey := "sk-"
	model := "Qwen/Qwen3-VL-4B-Instruct" // vision model

	// Use COCO test image via URL
	imageURL := "http://images.cocodataset.org/val2017/000000039769.jpg"

	log.Printf("Fetching COCO test image from: %s", imageURL)

	// Download the actual image bytes and dimensions
	resp, err := http.Get(imageURL)
	if err != nil {
		log.Fatalf("Failed to fetch image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Failed to fetch image: status %d", resp.StatusCode)
	}

	// Read image bytes
	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read image bytes: %v", err)
	}

	log.Printf("Downloaded image: %d bytes", len(imageBytes))

	// Decode JPEG to get actual dimensions
	img, err := jpeg.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Fatalf("Failed to decode JPEG: %v", err)
	}
	bounds := img.Bounds()
	actualWidth, actualHeight := bounds.Dx(), bounds.Dy()
	log.Printf("Actual image dimensions: %dx%d pixels", actualWidth, actualHeight)

	// Create OpenAI client
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	opts = append(opts, option.WithBaseURL(baseURL))
	client := openai.NewClient(opts...)

	// Build JSON schema for structured output
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"objects": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "integer",
							"description": "Unique identifier for this object",
						},
						"label": map[string]any{
							"type":        "string",
							"description": "Label/name of the object",
						},
						"bbox": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "number",
							},
							"minItems":    4,
							"maxItems":    4,
							"description": "Bounding box as [x1, y1, x2, y2] in normalized 1000 coordinates. 0=top/left, 1000=bottom/right. For example, [250, 300, 750, 800] means 25%-75% horizontal and 30%-80% vertical.",
						},
					},
					"required": []string{"id", "label", "bbox"},
				},
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Detailed description of the scene",
			},
		},
		"required":             []string{"objects", "description"},
		"additionalProperties": false,
	}

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "object_detection",
		Description: openai.String("Object detection with bounding boxes"),
		Schema:      schema,
		Strict:      openai.Bool(true),
	}

	// Build request with structured output
	prompt := `Detect ALL objects in this image and return bounding boxes.

IMPORTANT: Use NORMALIZED 1000 COORDINATES for bounding boxes.
- Coordinates range from 0 to 1000
- 0 = top/left edge
- 1000 = bottom/right edge
- For example: [250, 300, 750, 800] means the object spans from 25% to 75% horizontally (250 to 750) and 30% to 80% vertically (300 to 800)

For each object, provide:
- id: unique integer identifier
- label: what the object is (e.g., "person", "car", "cat")
- bbox: [x1, y1, x2, y2] in normalized 1000 coordinates

Also provide a description of the scene.`

	content := []openai.ChatCompletionContentPartUnionParam{
		openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: imageURL,
		}),
		openai.TextContentPart(prompt),
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(content),
		},
		MaxTokens: openai.Int(int64(2000)),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		},
	}

	ctx := context.Background()
	log.Printf("Sending request to model: %s", model)

	vlmResp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	if len(vlmResp.Choices) == 0 {
		log.Fatalf("No response from model")
	}

	log.Printf("\n========== RAW VLM RESPONSE ==========")
	log.Printf("%s", vlmResp.Choices[0].Message.Content)
	log.Printf("====================================\n")

	// Parse JSON response
	var result VLMResponse
	if err := json.Unmarshal([]byte(vlmResp.Choices[0].Message.Content), &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	log.Printf("\n========== PARSED OBJECTS ==========")
	for i, obj := range result.Objects {
		log.Printf("Object %d: ID=%d, Label=%q, BBox=%v", i, obj.ID, obj.Label, obj.BBox)
	}
	log.Printf("Description: %s", result.Description)
	log.Printf("====================================\n")

	// Analyze bounding box values - should be 0-1000 if using normalized coordinates
	log.Printf("\n========== BBOX COORDINATE ANALYSIS ==========")
	maxX, maxY := 0.0, 0.0
	minX, minY := 1000.0, 1000.0
	for _, obj := range result.Objects {
		if len(obj.BBox) >= 4 {
			if obj.BBox[0] < minX {
				minX = obj.BBox[0]
			}
			if obj.BBox[1] < minY {
				minY = obj.BBox[1]
			}
			if obj.BBox[2] > maxX {
				maxX = obj.BBox[2]
			}
			if obj.BBox[3] > maxY {
				maxY = obj.BBox[3]
			}
		}
	}
	log.Printf("Min bbox values: x1=%.1f, y1=%.1f", minX, minY)
	log.Printf("Max bbox values: x2=%.1f, y2=%.1f", maxX, maxY)

	// Check if coordinates are in normalized 1000 range
	if maxX <= 1000 && maxY <= 1000 {
		log.Printf("✓ Coordinates appear to be in NORMALIZED 1000 format (0-1000)")
	} else if maxX > 1000 || maxY > 1000 {
		log.Printf("⚠️  Coordinates exceed 1000 - model may be using pixel coordinates instead")
	}
	log.Printf("================================================\n")

	// Annotate using normalized 1000 scaling
	log.Printf("\n========== ANNOTATING WITH NORMALIZED 1000 SCALING ==========")

	// Pass the raw normalized 1000 response directly to AnnotateFrame
	// AnnotateFrame will handle the scaling
	annotatedData, err := webrtc.AnnotateFrame(imageBytes, vlmResp.Choices[0].Message.Content)
	if err != nil {
		log.Fatalf("Failed to annotate frame: %v", err)
	}

	// Save annotated image
	outputPath := "/tmp/bbox_test_normalized_1000.jpg"
	if err := os.WriteFile(outputPath, annotatedData, 0644); err != nil {
		log.Fatalf("Failed to save annotated image: %v", err)
	}

	log.Printf("Saved annotated image to: %s", outputPath)
	log.Printf("=========================================================\n")

	log.Printf("\n========== SUMMARY ==========")
	log.Printf("Using normalized 1000 coordinates: %s", outputPath)
	log.Printf("=============================\n")
}
