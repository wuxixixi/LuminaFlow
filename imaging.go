package main

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/bmp"
)

// ImageInfo holds validated image information
type ImageInfo struct {
	Path      string
	Filename  string
	Width     int
	Height    int
	Size      int64
	Format    string
	Base64    string // Base64 encoded image with data URI prefix
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

	// Check file size (must be < 20MB)
	if stat.Size() > 20*1024*1024 {
		return nil, fmt.Errorf("file size exceeds 20MB limit")
	}

	// Read entire file into memory once
	data := make([]byte, stat.Size())
	_, err = file.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Decode image from memory to get dimensions
	reader := strings.NewReader(string(data))
	config, format, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	width := config.Width
	height := config.Height

	// Check minimum dimension (short side >= 300px)
	minDim := width
	if height < minDim {
		minDim = height
	}
	if minDim < 300 {
		return nil, fmt.Errorf("short side must be at least 300px (got %d)", minDim)
	}

	// Check aspect ratio (2:5 ~ 5:2, i.e., 0.4 ~ 2.5)
	ratio := float64(width) / float64(height)
	if ratio < 0.4 || ratio > 2.5 {
		return nil, fmt.Errorf("aspect ratio must be between 2:5 and 5:2 (got %.2f)", ratio)
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
