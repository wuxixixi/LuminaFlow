package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/bmp"
	_ "golang.org/x/image/webp" // Register webp decoder
)

// Image validation constants
const (
	MinShortSide   = 300
	MaxFileSize    = 20 * 1024 * 1024 // 20MB
	MinAspectRatio = 0.4              // 2:5
	MaxAspectRatio = 2.5              // 5:2
	MaxImagePixels = 50 * 1000 * 1000 // 50 megapixels limit for decoding
)

// placeholderImage is a cached placeholder image
var placeholderImage image.Image
var placeholderOnce sync.Once

// GetPlaceholderImage returns a placeholder image for thumbnails (singleton)
func GetPlaceholderImage() image.Image {
	placeholderOnce.Do(func() {
		// Create a 80x60 placeholder image
		img := image.NewRGBA(image.Rect(0, 0, 80, 60))

		// Fill with light gray background
		bgColor := color.RGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff}
		draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

		// Draw a simple image icon in the center
		// Icon is a simple rectangle with a circle (representing sun/mountain)
		iconColor := color.RGBA{R: 0xb0, G: 0xb0, B: 0xb0, A: 0xff}

		// Draw mountain-like shape
		for y := 35; y < 50; y++ {
			for x := 20; x < 60; x++ {
				// Left mountain
				if x >= 20 && x <= 35 && y >= 35 && y < 50 {
					if y-35 >= (x-20) && y-35 >= (35-x)+20 {
						img.Set(x, y, iconColor)
					}
				}
				// Right mountain
				if x >= 35 && x <= 55 && y >= 30 && y < 50 {
					if y-30 >= (x-35)/2 && y-30 >= (55-x)/2 {
						img.Set(x, y, iconColor)
					}
				}
			}
		}

		// Draw sun (circle)
		for y := 15; y < 28; y++ {
			for x := 50; x < 63; x++ {
				dx := x - 56
				dy := y - 21
				if dx*dx+dy*dy <= 36 { // radius 6
					img.Set(x, y, iconColor)
				}
			}
		}

		placeholderImage = img
	})
	return placeholderImage
}

// ImageInfo holds validated image information
type ImageInfo struct {
	Path     string
	Filename string
	Width    int
	Height   int
	Size     int64
	Format   string
	Base64   string // Base64 encoded image with data URI prefix
}

// SupportedFormats lists the allowed image extensions
var SupportedFormats = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// ScanImages scans a directory for valid image files (including subdirectories)
func ScanImages(dir string) ([]ImageInfo, error) {
	var images []ImageInfo

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !SupportedFormats[ext] {
			return nil
		}

		imgInfo, err := LoadImageInfo(path)
		if err != nil {
			return nil // Skip invalid images
		}
		images = append(images, *imgInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return images, nil
}

// LoadImageInfo loads and validates a single image file
func LoadImageInfo(path string) (*ImageInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check file size limit
	if stat.Size() > MaxFileSize {
		return nil, fmt.Errorf("file size exceeds %dMB limit (got %.1fMB)", MaxFileSize/1024/1024, float64(stat.Size())/1024/1024)
	}

	// Read entire file into memory once
	data := make([]byte, stat.Size())
	_, err = file.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Decode image config from memory to get dimensions (doesn't decode full image)
	reader := bytes.NewReader(data)
	config, format, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	width := config.Width
	height := config.Height

	// Check for extremely large images to prevent memory issues
	pixels := int64(width) * int64(height)
	if pixels > MaxImagePixels {
		return nil, fmt.Errorf("image too large: %d megapixels (max %d)", pixels/1000000, MaxImagePixels/1000000)
	}

	// Check minimum dimension (short side >= 300px)
	minDim := width
	if height < minDim {
		minDim = height
	}
	if minDim < MinShortSide {
		return nil, fmt.Errorf("short side must be at least %dpx (got %d)", MinShortSide, minDim)
	}

	// Check aspect ratio (2:5 ~ 5:2, i.e., 0.4 ~ 2.5)
	ratio := float64(width) / float64(height)
	if ratio < MinAspectRatio || ratio > MaxAspectRatio {
		return nil, fmt.Errorf("aspect ratio must be between 2:5 and 5:2 (got %.2f:1)", ratio)
	}

	// Encode to base64 with data URI prefix
	mimeType := getMimeType(format)
	base64Str := base64.StdEncoding.EncodeToString(data)
	base64WithPrefix := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)

	return &ImageInfo{
		Path:     path,
		Filename: filepath.Base(path),
		Width:    width,
		Height:   height,
		Size:     stat.Size(),
		Format:   format,
		Base64:   base64WithPrefix,
	}, nil
}

// getMimeType returns the MIME type for an image format
func getMimeType(format string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	case "gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

// LoadImageForThumbnail loads an image for display as thumbnail
// Returns raw image data that can be rendered by Fyne
func LoadImageForThumbnail(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Decode based on format
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Decode(file)
	case ".png":
		return png.Decode(file)
	case ".bmp":
		return bmp.Decode(file)
	case ".webp":
		// Try to decode webp using generic decoder
		file.Seek(0, 0)
		img, _, err := image.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("failed to decode webp: %w", err)
		}
		return img, nil
	default:
		// Try generic decode
		file.Seek(0, 0)
		img, _, err := image.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("unsupported format: %s", ext)
		}
		return img, nil
	}
}

// GetOutputPath returns the output video path for an image
func GetOutputPath(imagePath, outputDir string) string {
	filename := filepath.Base(imagePath)
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	return filepath.Join(outputDir, name+".mp4")
}
