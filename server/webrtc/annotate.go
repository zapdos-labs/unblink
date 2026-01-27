package webrtc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// VLMResponse represents the JSON response from VLM with structured output
type VLMResponse struct {
	Objects      []VLMObject `json:"objects" jsonschema_description:"All detected objects with bounding boxes and IDs"`
	Description string      `json:"description" jsonschema_description:"Detailed analysis of motion, action, emotion, expressions, and subtle changes. Reference objects by their IDs in brackets, e.g., 'The person [3] is driving a red car [4]'."`
}

type VLMObject struct {
	ID    int       `json:"id" jsonschema_description:"Unique identifier for this object (used for tracking across frames)"`
	Label string    `json:"label" jsonschema_description:"Label/name of the object"`
	BBox  []float64 `json:"bbox" jsonschema_description:"Bounding box as [x1, y1, x2, y2] in pixel coordinates relative to the model's effective image resolution"`
}

// BBox represents a scaled bounding box in pixel coordinates
type BBox struct {
	X1, Y1, X2, Y2 int
	ID             int
	Label          string
}

// GenerateVLMResponseSchema generates the JSON schema for VLMResponse
func GenerateVLMResponseSchema() interface{} {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v VLMResponse
	schema := reflector.Reflect(v)
	return schema
}

// ScaleBBox scales bbox coordinates from effective resolution to actual image resolution
// VLM returns bboxes in pixel coordinates relative to its effective resolution
// We need to scale them to actual image dimensions
func ScaleBBox(rawBBox []float64, effectiveWidth, effectiveHeight, actualWidth, actualHeight int) BBox {
	// VLM bbox is in effective resolution pixel space
	// Scale to actual image dimensions
	x1 := int(rawBBox[0] * float64(actualWidth) / float64(effectiveWidth))
	y1 := int(rawBBox[1] * float64(actualHeight) / float64(effectiveHeight))
	x2 := int(rawBBox[2] * float64(actualWidth) / float64(effectiveWidth))
	y2 := int(rawBBox[3] * float64(actualHeight) / float64(effectiveHeight))

	return BBox{X1: x1, Y1: y1, X2: x2, Y2: y2, ID: 0, Label: ""}
}

// DrawRect draws a rectangle on the image
func DrawRect(dst *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	// Draw horizontal lines
	for x := x1; x <= x2; x++ {
		dst.Set(x, y1, c)
		dst.Set(x, y2, c)
	}
	// Draw vertical lines
	for y := y1; y <= y2; y++ {
		dst.Set(x1, y, c)
		dst.Set(x2, y, c)
	}
}

// DrawLabel draws text with background for visibility
func DrawLabel(dst *image.RGBA, x, y int, text string, textColor, bgColor color.Color) {
	const padding = 2
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(textColor),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x + padding), Y: fixed.I(y + padding + 10)},
	}

	// Measure text
	advance := d.MeasureString(text)
	textWidth := advance.Ceil()
	textHeight := 13

	// Draw background rectangle
	for by := y; by < y+textHeight+2*padding; by++ {
		for bx := x; bx < x+textWidth+2*padding; bx++ {
			dst.Set(bx, by, bgColor)
		}
	}

	// Draw text
	d.DrawString(text)
}

// Colors for different objects (cycle through)
var boxColors = []color.Color{
	color.RGBA{255, 0, 0, 255},    // Red
	color.RGBA{0, 255, 0, 255},    // Green
	color.RGBA{0, 0, 255, 255},    // Blue
	color.RGBA{255, 255, 0, 255},  // Yellow
	color.RGBA{255, 0, 255, 255},  // Magenta
	color.RGBA{0, 255, 255, 255},  // Cyan
}

// AnnotateFrame draws bounding boxes and IDs onto a JPEG frame
// Returns annotated JPEG bytes
func AnnotateFrame(jpegData []byte, vlmResponse string, effectiveWidth, effectiveHeight int) ([]byte, error) {
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

	// Draw each bounding box
	for i, obj := range resp.Objects {
		if len(obj.BBox) < 4 {
			continue
		}

		// Scale bbox from effective dimensions to actual dimensions
		bbox := ScaleBBox(obj.BBox, effectiveWidth, effectiveHeight, actualWidth, actualHeight)
		bbox.ID = obj.ID
		bbox.Label = obj.Label

		// Pick color
		boxColor := boxColors[i%len(boxColors)]

		// Draw bounding box
		DrawRect(rgba, bbox.X1, bbox.Y1, bbox.X2, bbox.Y2, boxColor)

		// Draw label: "ID: label"
		label := fmt.Sprintf("%d: %s", obj.ID, obj.Label)
		DrawLabel(rgba, bbox.X1, bbox.Y1-15, label, color.White, boxColor)
	}

	// Re-encode as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	log.Printf("[Annotate] Drew %d bounding boxes on %dx%d frame (effective: %dx%d)",
		len(resp.Objects), actualWidth, actualHeight, effectiveWidth, effectiveHeight)
	return buf.Bytes(), nil
}

// SaveAnnotatedFrame writes annotated frame to disk for debugging
func SaveAnnotatedFrame(data []byte, serviceID string, sequence int64, baseDir string) error {
	if baseDir == "" {
		return nil // No storage configured
	}

	// Create annotated subdirectory
	annotatedDir := filepath.Join(baseDir, serviceID, "annotated")
	if err := os.MkdirAll(annotatedDir, 0755); err != nil {
		return fmt.Errorf("failed to create annotated directory: %w", err)
	}

	// Write file
	filename := filepath.Join(annotatedDir, fmt.Sprintf("%019d.jpg", sequence))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write annotated frame: %w", err)
	}

	log.Printf("[Annotate] Saved annotated frame: %s", filename)
	return nil
}
