package webrtc

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"time"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/draw"
	"golang.org/x/image/font/gofont/goregular"
)

// PreprocessFrame resizes the frame (max edge = 800px, maintaining aspect ratio) and burns in the timestamp
// Returns the preprocessed JPEG data
func PreprocessFrame(frameData []byte, timestamp time.Time) ([]byte, error) {
	// Decode JPEG
	img, err := jpeg.Decode(bytes.NewReader(frameData))
	if err != nil {
		return nil, fmt.Errorf("decode jpeg: %w", err)
	}

	// Calculate new dimensions maintaining aspect ratio with max edge = 800
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	maxEdge := 800
	var newWidth, newHeight int

	if width > height {
		// Width is the larger edge
		if width > maxEdge {
			newWidth = maxEdge
			newHeight = (height * maxEdge) / width
		} else {
			newWidth = width
			newHeight = height
		}
	} else {
		// Height is the larger edge
		if height > maxEdge {
			newHeight = maxEdge
			newWidth = (width * maxEdge) / height
		} else {
			newWidth = width
			newHeight = height
		}
	}

	// Resize maintaining aspect ratio
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Over, nil)

	// Burn in timestamp
	timestampStr := timestamp.Format("2006-01-02 15:04:05.000 MST")
	if err := drawTimestamp(resized, timestampStr); err != nil {
		// Non-fatal: just log and continue without timestamp
		// (we still want to send the resized frame even if text rendering fails)
		fmt.Printf("[PreprocessFrame] Failed to draw timestamp: %v\n", err)
	}

	// Encode back to JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), nil
}

// drawTimestamp draws the timestamp text on the image
func drawTimestamp(img *image.RGBA, text string) error {
	// Parse the font
	font, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return fmt.Errorf("parse font: %w", err)
	}

	// Create freetype context
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(font)
	c.SetFontSize(16)
	c.SetClip(img.Bounds())
	c.SetDst(img)

	// Draw black background rectangle for text (for better readability)
	textHeight := 24
	for y := 0; y < textHeight; y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 200}) // Semi-transparent black
		}
	}

	// Draw white text
	c.SetSrc(image.NewUniform(color.RGBA{255, 255, 255, 255}))
	pt := freetype.Pt(10, 18) // 10px from left, 18px from top

	if _, err := c.DrawString(text, pt); err != nil {
		return fmt.Errorf("draw string: %w", err)
	}

	return nil
}
