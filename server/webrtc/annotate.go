package webrtc

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/invopop/jsonschema"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
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

// BBox represents a scaled bounding box in pixel coordinates
type BBox struct {
	X1, Y1, X2, Y2 int
}

// GenerateVLMResponseSchema generates the JSON schema for VLMResponse
func GenerateVLMResponseSchema() any {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v VLMResponse
	schema := reflector.Reflect(v)
	return schema
}

// ScaleBBox scales bbox coordinates from normalized 1000 space to actual image resolution
// Qwen3-VL (and similar models) return bboxes in normalized 1000 coordinates
// where 0-1000 represents 0-100% of the image dimension
func ScaleBBox(rawBBox []float64, actualWidth, actualHeight int) BBox {
	// VLM bbox is in normalized 1000 space (0-1000)
	// Scale to actual image dimensions: normalized * actual / 1000
	x1 := int(rawBBox[0] * float64(actualWidth) / 1000.0)
	y1 := int(rawBBox[1] * float64(actualHeight) / 1000.0)
	x2 := int(rawBBox[2] * float64(actualWidth) / 1000.0)
	y2 := int(rawBBox[3] * float64(actualHeight) / 1000.0)

	return BBox{X1: x1, Y1: y1, X2: x2, Y2: y2}
}

// DrawLabel draws text with background for visibility
func DrawLabel(dst *image.RGBA, x, y int, text string, textColor, bgColor color.Color) {
	const padding = 1
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(textColor),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x + padding), Y: fixed.I(y + padding + 8)},
	}

	// Measure text
	advance := d.MeasureString(text)
	textWidth := advance.Ceil()
	textHeight := 10

	// Draw background rectangle
	for by := y; by < y+textHeight+2*padding; by++ {
		for bx := x; bx < x+textWidth+2*padding; bx++ {
			dst.Set(bx, by, bgColor)
		}
	}

	// Draw text
	d.DrawString(text)
}

// hashToBrightColor generates a consistent bright color from a string hash
func hashToBrightColor(s string) color.Color {
	h := md5.New()
	h.Write([]byte(s))
	hash := h.Sum(nil)

	// Use first 3 bytes of hash for RGB
	// Ensure brightness by keeping values high (128-255 range)
	r := uint8(128) + (hash[0] % 127)
	g := uint8(128) + (hash[1] % 127)
	b := uint8(128) + (hash[2] % 127)

	return color.RGBA{r, g, b, 255}
}

// AnnotateFrame draws tags (label and id) at the center of each bounding box
// Expects VLM to return normalized 1000 coordinates (0-1000)
// Returns annotated JPEG bytes
func AnnotateFrame(jpegData []byte, vlmResponse string) ([]byte, error) {
	// Parse VLM JSON response
	var resp VLMResponse
	if err := json.Unmarshal([]byte(vlmResponse), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse VLM response: %w", err)
	}

	// Decode JPEG
	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode JPEG: %w", err)
	}

	bounds := img.Bounds()
	actualWidth, actualHeight := bounds.Dx(), bounds.Dy()

	// Create drawable RGBA image
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, image.Point{}, draw.Src)

	// Draw each tag at center of bounding box
	for _, obj := range resp.Objects {
		if len(obj.BBox) < 4 {
			continue
		}

		// Scale bbox from normalized 1000 to actual dimensions
		bbox := ScaleBBox(obj.BBox, actualWidth, actualHeight)

		// Calculate center of bounding box
		centerX := (bbox.X1 + bbox.X2) / 2
		centerY := (bbox.Y1 + bbox.Y2) / 2

		// Generate color from hash of (label+id)
		tagKey := fmt.Sprintf("%s%d", obj.Label, obj.ID)
		textColor := hashToBrightColor(tagKey)

		// Draw only the ID number at center with black background
		tag := fmt.Sprintf("%d", obj.ID)
		DrawLabel(rgba, centerX, centerY, tag, textColor, color.Black)
	}

	// Re-encode as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	log.Printf("[Annotate] Drew %d tags on %dx%d frame (normalized 1000 scaling)",
		len(resp.Objects), actualWidth, actualHeight)
	return buf.Bytes(), nil
}

// SaveAnnotatedFrame writes annotated frame to disk for debugging
func SaveAnnotatedFrame(data []byte, serviceID string, timestamp time.Time, baseDir string) error {
	if baseDir == "" {
		return nil // No storage configured
	}

	// Create annotated subdirectory
	annotatedDir := filepath.Join(baseDir, serviceID, "annotated")
	if err := os.MkdirAll(annotatedDir, 0755); err != nil {
		return fmt.Errorf("failed to create annotated directory: %w", err)
	}

	// Write file with timestamp as filename (Unix nanos)
	filename := filepath.Join(annotatedDir, fmt.Sprintf("%019d.jpg", timestamp.UnixNano()))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write annotated frame: %w", err)
	}

	log.Printf("[Annotate] Saved annotated frame: %s", filename)
	return nil
}
