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
	BaseURL              = "https://www.dmxapi.cn"
	SubmitEndpoint       = "/v1/video_generation"
	QueryEndpoint        = "/v1/query/video_generation"
	RetrieveEndpoint     = "/v1/files/retrieve"
	BalanceEndpoint      = "/api/user/self"
	TokenBalanceEndpoint = "/api/token/key/"

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
		req.Header.Set("User-Agent", "LuminaFlow/"+AppVersion)

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
			interval = time.Duration(min(int(interval*2), int(MaxPollInterval)))
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

		interval = time.Duration(min(int(interval*2), int(MaxPollInterval)))
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
		ID          int     `json:"id"`
		Username    string  `json:"username"`
		Quota       int64   `json:"quota"`
		UsedQuota   int64   `json:"used_quota"`
		BonusQuota  int64   `json:"bonus_quota"`
		TopupAmount float64 `json:"topup_amount"`
		DisplayName string  `json:"display_name"`
		Email       string  `json:"email"`
		Level       string  `json:"level"`
	} `json:"data"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// BalanceInfo holds parsed balance information
type BalanceInfo struct {
	Quota     int64   // 剩余配额
	UsedQuota int64   // 已使用配额
	Balance   float64 // 余额（元）
	Used      float64 // 已使用（元）
}

// TokenBalanceResponse represents the token balance response
type TokenBalanceResponse struct {
	Data struct {
		ID             int     `json:"id"`
		Name           string  `json:"name"`
		Key            string  `json:"key"`
		Status         int     `json:"status"`
		UsedQuota      float64 `json:"used_quota"`
		RemainQuota    float64 `json:"remain_quota"`
		RemainCount    int     `json:"remain_count"`
		UnlimitedQuota bool    `json:"unlimited_quota"`
		UnlimitedCount bool    `json:"unlimited_count"`
	} `json:"data"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// GetBalance queries the API account balance using system token
func (c *APIClient) GetBalance(ctx context.Context, systemToken, userID string) (*BalanceInfo, error) {
	if systemToken == "" {
		return nil, fmt.Errorf("系统令牌未配置，请在设置中配置系统令牌")
	}
	if userID == "" {
		return nil, fmt.Errorf("用户ID未配置，请在设置中配置用户ID")
	}

	queryURL := fmt.Sprintf("%s%s", BaseURL, BalanceEndpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", systemToken)
	req.Header.Set("Rix-Api-User", userID)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LuminaFlow/"+AppVersion)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	Info("Balance API response: %s", string(body))

	var result BalanceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API returned error: %s", result.Message)
	}

	// Calculate balance: quota / 500000 = CNY
	info := &BalanceInfo{
		Quota:     result.Data.Quota,
		UsedQuota: result.Data.UsedQuota,
		Balance:   float64(result.Data.Quota) / 500000.0,
		Used:      float64(result.Data.UsedQuota) / 500000.0,
	}

	Info("Balance query successful: quota=%d, used=%d, balance=%.2f CNY", info.Quota, info.UsedQuota, info.Balance)
	return info, nil
}

// GetTokenBalance queries the token's remaining quota and count
func (c *APIClient) GetTokenBalance(ctx context.Context, systemToken, userID string) (remainCount int, err error) {
	if systemToken == "" {
		return 0, fmt.Errorf("系统令牌未配置")
	}
	if userID == "" {
		return 0, fmt.Errorf("用户ID未配置")
	}

	queryURL := fmt.Sprintf("%s%s%s", BaseURL, TokenBalanceEndpoint, c.APIKey)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	// Token balance query requires Authorization, Rix-Api-User headers
	req.Header.Set("Authorization", systemToken)
	req.Header.Set("Rix-Api-User", userID)
	req.Header.Set("User-Agent", "LuminaFlow/"+AppVersion)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	Info("Token balance API response: %s", string(body))

	if resp.StatusCode != 200 {
		return 0, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result TokenBalanceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("API error: %s", result.Message)
	}

	Info("Token balance query successful: remain_count=%d, remain_quota=%.2f", result.Data.RemainCount, result.Data.RemainQuota)
	return result.Data.RemainCount, nil
}

// ChatCompletionMessage represents a message in chat completion
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest for LLM chat completion API
type ChatCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []ChatCompletionMessage `json:"messages"`
}

// ChatCompletionResponse from LLM chat completion API
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int                   `json:"index"`
		Message ChatCompletionMessage `json:"message"`
		Finish  string                `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatCompletion calls the LLM chat completion API
func (c *APIClient) ChatCompletion(ctx context.Context, messages []ChatCompletionMessage) (string, error) {
	reqBody := ChatCompletionRequest{
		Model:    "glm-5.1-free",
		Messages: messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.dmxapi.cn/v1/chat/completions", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LuminaFlow/"+AppVersion)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	Info("Chat completion successful: model=%s, tokens=%d", result.Model, result.Usage.TotalTokens)
	return result.Choices[0].Message.Content, nil
}

// OptimizePrompt uses LLM to optimize a video generation prompt
func (c *APIClient) OptimizePrompt(ctx context.Context, originalPrompt string) (string, error) {
	systemPrompt := `你是一个专业的AI视频生成提示词优化专家。你的任务是优化用户提供的视频生成提示词，使其更加精确、生动，能够生成更高质量的视频。

优化原则：
1. 保持原始提示词的核心意图不变
2. 添加具体的视觉描述（光影、色彩、构图）
3. 明确镜头运动方式和速度
4. 加入专业电影术语（如推拉摇移、景深、焦点变化等）
5. 保持提示词简洁但有表现力
6. 输出格式：直接输出优化后的提示词，不要添加任何解释或前缀

请优化以下提示词：`

	messages := []ChatCompletionMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: originalPrompt},
	}

	return c.ChatCompletion(ctx, messages)
}
