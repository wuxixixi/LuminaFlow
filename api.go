package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	BaseURL          = "https://www.dmxapi.cn"
	SubmitEndpoint   = "/v1/video_generation"
	QueryEndpoint    = "/v1/query/video_generation"
	RetrieveEndpoint = "/v1/files/retrieve"
	BalanceEndpoint  = "/v1/balance"

	// Polling configuration
	InitialPollInterval = 5 * time.Second
	MaxPollInterval     = 30 * time.Second
	PollTimeout         = 10 * time.Minute

	// Retry configuration
	MaxRetries    = 3
	RetryBaseWait = 2 * time.Second
)

// APIError represents an error from the API
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
}

// IsRetryable checks if the error is retryable
func (e *APIError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}

// SubmitRequest for video generation
type SubmitRequest struct {
	Model           string `json:"model"`
	Prompt          string `json:"prompt"`
	FirstFrameImage string `json:"first_frame_image"`
	Duration        int    `json:"duration"`
	Resolution      string `json:"resolution"`
}

// SubmitResponse from video generation submission
type SubmitResponse struct {
	TaskID   string `json:"task_id"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// QueryResponse from status query
type QueryResponse struct {
	Status      string `json:"status"`
	FileID      string `json:"file_id,omitempty"`
	TaskID      string `json:"task_id"`
	VideoWidth  int    `json:"video_width,omitempty"`
	VideoHeight int    `json:"video_height,omitempty"`
	BaseResp    struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// RetrieveResponse for file download
type RetrieveResponse struct {
	File struct {
		FileID      int    `json:"file_id"`
		Filename    string `json:"filename"`
		DownloadURL string `json:"download_url"`
	} `json:"file"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// APIClient handles all DMXAPI communications
type APIClient struct {
	APIKey     string
	HTTPClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(apiKey string) *APIClient {
	return &APIClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with retry logic
func (c *APIClient) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var jsonData []byte
	var err error
	if body != nil {
		jsonData, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * RetryBaseWait
			Info("Retrying request (attempt %d/%d) after %v", attempt+1, MaxRetries, backoff)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Create a new request for each attempt (required for body to be readable)
		var reqBody *strings.Reader
		if jsonData != nil {
			reqBody = strings.NewReader(string(jsonData))
		} else {
			reqBody = strings.NewReader("")
		}

		req, err := http.NewRequestWithContext(ctx, method, BaseURL+endpoint, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", c.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			Warn("Request failed: %v", err)
			lastErr = err
			continue
		}

		// Success
		if resp.StatusCode < 400 {
			return resp, nil
		}

		// Read error body
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		apiErr := &APIError{StatusCode: resp.StatusCode, Message: string(bodyBytes)}

		// Don't retry client errors (except 429)
		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			Warn("API client error (%d): %s", resp.StatusCode, string(bodyBytes))
			return nil, apiErr
		}

		Warn("API server error (%d), will retry: %s", resp.StatusCode, string(bodyBytes))
		lastErr = apiErr
	}

	Error("All %d retry attempts failed", MaxRetries)
	return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
}

// SubmitTask submits a video generation task
func (c *APIClient) SubmitTask(ctx context.Context, req *SubmitRequest) (string, error) {
	Info("Submitting video generation task: model=%s, duration=%d, resolution=%s", req.Model, req.Duration, req.Resolution)

	resp, err := c.doRequest(ctx, "POST", SubmitEndpoint, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result SubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.BaseResp.StatusCode != 0 {
		return "", &APIError{StatusCode: result.BaseResp.StatusCode, Message: result.BaseResp.StatusMsg}
	}

	Info("Task submitted successfully: task_id=%s", result.TaskID)
	return result.TaskID, nil
}

// QueryTask queries the status of a video generation task
func (c *APIClient) QueryTask(ctx context.Context, taskID string) (*QueryResponse, error) {
	queryURL := fmt.Sprintf("%s%s?task_id=%s", BaseURL, QueryEndpoint, url.QueryEscape(taskID))

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// RetrieveFile gets the download URL for a completed video
func (c *APIClient) RetrieveFile(ctx context.Context, fileID, taskID string) (string, error) {
	queryURL := fmt.Sprintf("%s%s?file_id=%s&task_id=%s", BaseURL, RetrieveEndpoint, url.QueryEscape(fileID), url.QueryEscape(taskID))

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result RetrieveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.BaseResp.StatusCode != 0 {
		return "", &APIError{StatusCode: result.BaseResp.StatusCode, Message: result.BaseResp.StatusMsg}
	}

	Info("Retrieved download URL for file_id=%s", fileID)
	return result.File.DownloadURL, nil
}

// DownloadVideo downloads a video file from the given URL with retry
func (c *APIClient) DownloadVideo(ctx context.Context, downloadURL, outputPath string) error {
	Info("Downloading video to: %s", outputPath)

	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * RetryBaseWait
			Info("Retrying download (attempt %d/%d) after %v", attempt+1, MaxRetries, backoff)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			Warn("Download failed: %v", err)
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			lastErr = &APIError{StatusCode: resp.StatusCode, Message: string(body)}
			Warn("Download failed with status %d", resp.StatusCode)
			continue
		}

		// Create output file
		outFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}

		// Copy with progress tracking
		_, err = io.Copy(outFile, resp.Body)
		outFile.Close()

		if err != nil {
			os.Remove(outputPath) // Clean up partial file
			Warn("Failed to write video file: %v", err)
			lastErr = fmt.Errorf("failed to write video file: %w", err)
			continue
		}

		Info("Video downloaded successfully: %s", outputPath)
		return nil
	}

	return fmt.Errorf("download failed after %d attempts: %w", MaxRetries, lastErr)
}

// PollTask polls a task until completion or timeout
func (c *APIClient) PollTask(ctx context.Context, taskID string) (fileID string, err error) {
	Info("Polling task: %s", taskID)

	interval := InitialPollInterval
	deadline := time.Now().Add(PollTimeout)
	pollCount := 0

	for {
		pollCount++
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			Error("Poll timeout after %v (task_id=%s)", PollTimeout, taskID)
			return "", fmt.Errorf("poll timeout after %v", PollTimeout)
		}

		result, err := c.QueryTask(ctx, taskID)
		if err != nil {
			Warn("Query failed (poll %d): %v", pollCount, err)
			time.Sleep(interval)
			interval = min(interval*2, MaxPollInterval)
			continue
		}

		switch result.Status {
		case "Success":
			Info("Task completed successfully (task_id=%s, file_id=%s)", taskID, result.FileID)
			return result.FileID, nil
		case "Fail", "Failed":
			Error("Video generation failed (task_id=%s): %s", taskID, result.BaseResp.StatusMsg)
			return "", fmt.Errorf("video generation failed: %s", result.BaseResp.StatusMsg)
		case "Processing":
			Debug("Task processing... (poll %d, task_id=%s)", pollCount, taskID)
		case "":
			Debug("Empty status, continuing poll (task_id=%s)", taskID)
		default:
			Warn("Unknown status: %s (task_id=%s)", result.Status, taskID)
		}

		// Wait before next poll
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		interval = min(interval*2, MaxPollInterval)
	}
}

// ConvertImageToVideo performs the full 3-step workflow with retry
func (c *APIClient) ConvertImageToVideo(ctx context.Context, imageBase64, prompt, outputPath string, duration int, resolution string) error {
	Info("Starting image to video conversion: output=%s", outputPath)

	// Step 1: Submit task
	submitReq := &SubmitRequest{
		Model:           "MiniMax-Hailuo-2.3",
		Prompt:          prompt,
		FirstFrameImage: imageBase64,
		Duration:        duration,
		Resolution:      resolution,
	}

	taskID, err := c.SubmitTask(ctx, submitReq)
	if err != nil {
		return fmt.Errorf("submit failed: %w", err)
	}

	// Step 2: Poll for completion
	fileID, err := c.PollTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("poll failed: %w", err)
	}

	// Step 3: Get download URL
	downloadURL, err := c.RetrieveFile(ctx, fileID, taskID)
	if err != nil {
		return fmt.Errorf("retrieve failed: %w", err)
	}

	// Step 4: Download video
	err = c.DownloadVideo(ctx, downloadURL, outputPath)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	Info("Image to video conversion completed: %s", outputPath)
	return nil
}

// BalanceResponse represents the API balance response
type BalanceResponse struct {
	Data struct {
		TotalBalance float64 `json:"total_balance"`
		Currency     string  `json:"currency"`
	} `json:"data"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// GetBalance queries the API account balance
func (c *APIClient) GetBalance(ctx context.Context) (float64, string, error) {
	queryURL := fmt.Sprintf("%s%s", BaseURL, BalanceEndpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, "", &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.BaseResp.StatusCode != 0 {
		return 0, "", &APIError{StatusCode: result.BaseResp.StatusCode, Message: result.BaseResp.StatusMsg}
	}

	Info("Balance query successful: %.2f %s", result.Data.TotalBalance, result.Data.Currency)
	return result.Data.TotalBalance, result.Data.Currency, nil
}
