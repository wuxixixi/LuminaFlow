package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

// APIServer provides HTTP API for external access
type APIServer struct {
	config    *Config
	processor *Processor
	server    *http.Server
	tasks     map[string]*APITask // task_id -> task
	mu        sync.RWMutex
	port      int
}

// APITask represents a task for API tracking
type APITask struct {
	ID         string     `json:"id"`
	ImagePath  string     `json:"image_path"`
	OutputPath string     `json:"output_path"`
	Status     string     `json:"status"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// APIRequest structures
type ConvertRequest struct {
	ImagePath   string `json:"image_path"`   // Local file path
	ImageBase64 string `json:"image_base64"` // Base64 encoded image (alternative to ImagePath)
	Filename    string `json:"filename"`     // Required if ImageBase64 is provided
	Prompt      string `json:"prompt"`       // Optional, uses default if empty
	Duration    int    `json:"duration"`     // Optional, uses default if 0
	Resolution  string `json:"resolution"`   // Optional, uses default if empty
	OutputDir   string `json:"output_dir"`   // Optional, uses default if empty
}

type BatchConvertRequest struct {
	Images      []ConvertRequest `json:"images"`
	Concurrency int              `json:"concurrency"` // Optional, uses default if 0
}

type ConvertResponse struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type TaskStatusResponse struct {
	Success bool     `json:"success"`
	Task    *APITask `json:"task,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type TaskListResponse struct {
	Success bool       `json:"success"`
	Tasks   []*APITask `json:"tasks"`
	Total   int        `json:"total"`
}

type ServerInfoResponse struct {
	Success bool   `json:"success"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Port    int    `json:"port"`
	Status  string `json:"status"`
}

// NewAPIServer creates a new API server
func NewAPIServer(config *Config, port int) *APIServer {
	return &APIServer{
		config:    config,
		processor: NewProcessor(config),
		tasks:     make(map[string]*APITask),
		port:      port,
	}
}

// Start begins the API server with graceful shutdown
func (s *APIServer) Start() error {
	r := mux.NewRouter()

	// CORS middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// API routes
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/info", s.handleInfo).Methods("GET")
	api.HandleFunc("/convert", s.handleConvert).Methods("POST")
	api.HandleFunc("/batch", s.handleBatch).Methods("POST")
	api.HandleFunc("/status/{id}", s.handleStatus).Methods("GET")
	api.HandleFunc("/tasks", s.handleTasks).Methods("GET")
	api.HandleFunc("/download/{filename}", s.handleDownload).Methods("GET")
	api.HandleFunc("/upload", s.handleUpload).Methods("POST")
	api.HandleFunc("/stop/{id}", s.handleStop).Methods("POST")
	api.HandleFunc("/stop", s.handleStopAll).Methods("POST")

	// Static file serving for output directory
	r.PathPrefix("/output/").Handler(http.StripPrefix("/output/", http.FileServer(http.Dir(s.config.OutputDir))))

	addr := fmt.Sprintf(":%d", s.port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown setup
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		Info("Shutting down API server...")
		s.server.Shutdown(context.Background())
	}()

	Info("API server starting on %s", addr)
	return s.server.ListenAndServe()
}

// Stop shuts down the API server
func (s *APIServer) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handleInfo returns server information
func (s *APIServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	status := "idle"
	if s.processor.IsRunning() {
		status = "processing"
	}

	resp := ServerInfoResponse{
		Success: true,
		Name:    AppName,
		Version: AppVersion,
		Port:    s.port,
		Status:  status,
	}
	json.NewEncoder(w).Encode(resp)
}

// handleConvert handles single image conversion
func (s *APIServer) handleConvert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Validate request
	if req.ImagePath == "" && req.ImageBase64 == "" {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   "Either image_path or image_base64 is required",
		})
		return
	}

	if req.ImageBase64 != "" && req.Filename == "" {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   "filename is required when using image_base64",
		})
		return
	}

	// Check API key
	if s.config.APIKey == "" {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   "API key not configured",
		})
		return
	}

	// Prepare image
	var imagePath string
	var cleanup func()

	if req.ImagePath != "" {
		imagePath = req.ImagePath
	} else {
		// Save base64 image to temp file
		tempPath, err := s.saveBase64Image(req.ImageBase64, req.Filename)
		if err != nil {
			json.NewEncoder(w).Encode(ConvertResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to save image: %v", err),
			})
			return
		}
		imagePath = tempPath
		cleanup = func() { os.Remove(tempPath) }
	}

	// Load and validate image
	imgInfo, err := LoadImageInfo(imagePath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid image: %v", err),
		})
		return
	}

	// Determine output directory
	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = s.config.OutputDir
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		if cleanup != nil {
			cleanup()
		}
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create output directory: %v", err),
		})
		return
	}

	outputPath := GetOutputPath(imagePath, outputDir)

	// Use defaults for optional parameters
	prompt := req.Prompt
	if prompt == "" {
		prompt = s.config.Prompt
	}
	duration := req.Duration
	if duration == 0 {
		duration = s.config.Duration
	}
	resolution := req.Resolution
	if resolution == "" {
		resolution = s.config.Resolution
	}

	// Create task ID
	taskID := generateTaskID()

	// Create API task
	task := &APITask{
		ID:         taskID,
		ImagePath:  imagePath,
		OutputPath: outputPath,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	// Start conversion in background
	go func() {
		s.processTask(taskID, imgInfo, prompt, duration, resolution, outputPath, cleanup)
	}()

	json.NewEncoder(w).Encode(ConvertResponse{
		Success: true,
		TaskID:  taskID,
		Message: "Task submitted successfully",
	})
}

// handleBatch handles batch conversion
func (s *APIServer) handleBatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req BatchConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if len(req.Images) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No images provided",
		})
		return
	}

	taskIDs := make([]string, 0, len(req.Images))
	for _, img := range req.Images {
		// Create individual convert request
		convertReq := ConvertRequest{
			ImagePath:   img.ImagePath,
			ImageBase64: img.ImageBase64,
			Filename:    img.Filename,
			Prompt:      img.Prompt,
			Duration:    img.Duration,
			Resolution:  img.Resolution,
			OutputDir:   img.OutputDir,
		}

		// Process each image
		taskID := s.submitConvertTask(convertReq)
		if taskID != "" {
			taskIDs = append(taskIDs, taskID)
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"task_ids": taskIDs,
		"total":    len(taskIDs),
		"message":  fmt.Sprintf("Submitted %d tasks", len(taskIDs)),
	})
}

// submitConvertTask submits a single convert task
func (s *APIServer) submitConvertTask(req ConvertRequest) string {
	// Validate request
	if req.ImagePath == "" && req.ImageBase64 == "" {
		return ""
	}

	if req.ImageBase64 != "" && req.Filename == "" {
		return ""
	}

	// Check API key
	if s.config.APIKey == "" {
		return ""
	}

	// Prepare image
	var imagePath string
	var cleanup func()

	if req.ImagePath != "" {
		imagePath = req.ImagePath
	} else {
		tempPath, err := s.saveBase64Image(req.ImageBase64, req.Filename)
		if err != nil {
			return ""
		}
		imagePath = tempPath
		cleanup = func() { os.Remove(tempPath) }
	}

	imgInfo, err := LoadImageInfo(imagePath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return ""
	}

	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = s.config.OutputDir
	}
	os.MkdirAll(outputDir, 0755)

	outputPath := GetOutputPath(imagePath, outputDir)

	prompt := req.Prompt
	if prompt == "" {
		prompt = s.config.Prompt
	}
	duration := req.Duration
	if duration == 0 {
		duration = s.config.Duration
	}
	resolution := req.Resolution
	if resolution == "" {
		resolution = s.config.Resolution
	}

	taskID := generateTaskID()
	task := &APITask{
		ID:         taskID,
		ImagePath:  imagePath,
		OutputPath: outputPath,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	go s.processTask(taskID, imgInfo, prompt, duration, resolution, outputPath, cleanup)

	return taskID
}

// processTask processes a single conversion task
func (s *APIServer) processTask(taskID string, imgInfo *ImageInfo, prompt string, duration int, resolution string, outputPath string, cleanup func()) {
	s.updateTaskStatus(taskID, "processing")

	apiClient := NewAPIClient(s.config.APIKey)
	ctx := context.Background()

	err := apiClient.ConvertImageToVideo(ctx, imgInfo.Base64, prompt, outputPath, duration, resolution)

	if cleanup != nil {
		cleanup()
	}

	if err != nil {
		s.mu.Lock()
		if task, ok := s.tasks[taskID]; ok {
			task.Status = "failed"
			task.Error = err.Error()
			now := time.Now()
			task.FinishedAt = &now
		}
		s.mu.Unlock()
		Error("Task %s failed: %v", taskID, err)
		return
	}

	s.mu.Lock()
	if task, ok := s.tasks[taskID]; ok {
		task.Status = "completed"
		now := time.Now()
		task.FinishedAt = &now
	}
	s.mu.Unlock()
	Info("Task %s completed: %s", taskID, outputPath)
}

// updateTaskStatus updates a task's status
func (s *APIServer) updateTaskStatus(taskID string, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks[taskID]; ok {
		task.Status = status
	}
}

// handleStatus returns task status
func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	s.mu.RLock()
	task, ok := s.tasks[taskID]
	s.mu.RUnlock()

	if !ok {
		json.NewEncoder(w).Encode(TaskStatusResponse{
			Success: false,
			Error:   "Task not found",
		})
		return
	}

	json.NewEncoder(w).Encode(TaskStatusResponse{
		Success: true,
		Task:    task,
	})
}

// handleTasks returns all tasks
func (s *APIServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s.mu.RLock()
	tasks := make([]*APITask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	s.mu.RUnlock()

	json.NewEncoder(w).Encode(TaskListResponse{
		Success: true,
		Tasks:   tasks,
		Total:   len(tasks),
	})
}

// handleDownload serves the output video file
func (s *APIServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]

	// Security: prevent path traversal
	if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	filepath := filepath.Join(s.config.OutputDir, filename)
	http.ServeFile(w, r, filepath)
}

// handleStop stops a specific task
func (s *APIServer) handleStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	s.mu.Lock()
	if task, ok := s.tasks[taskID]; ok {
		if task.Status == "pending" || task.Status == "processing" {
			task.Status = "cancelled"
			now := time.Now()
			task.FinishedAt = &now
		}
	}
	s.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Task stopped",
	})
}

// handleStopAll stops all processing
func (s *APIServer) handleStopAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s.processor.Stop()

	s.mu.Lock()
	for _, task := range s.tasks {
		if task.Status == "pending" || task.Status == "processing" {
			task.Status = "cancelled"
			now := time.Now()
			task.FinishedAt = &now
		}
	}
	s.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "All tasks stopped",
	})
}

// saveBase64Image saves a base64 encoded image to a temp file
func (s *APIServer) saveBase64Image(base64Data string, filename string) (string, error) {
	// Decode base64
	var data []byte
	var err error

	// Check if it has data URI prefix
	if strings.HasPrefix(base64Data, "data:") {
		// Extract the base64 part after the comma
		parts := strings.SplitN(base64Data, ",", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid data URI format")
		}
		data, err = base64.StdEncoding.DecodeString(parts[1])
	} else {
		data, err = base64.StdEncoding.DecodeString(base64Data)
	}

	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %v", err)
	}

	// Create temp file
	tempDir := os.TempDir()
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	tempFile := filepath.Join(tempDir, fmt.Sprintf("luminaflow_%s%s", generateTaskID(), ext))

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write temp file: %v", err)
	}

	return tempFile, nil
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// StartAPIServer starts the API server (for CLI mode)
func StartAPIServer(config *Config, port int) error {
	server := NewAPIServer(config, port)

	// Ensure output directory exists
	if err := config.EnsureOutputDir(); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	Info("Starting LuminaFlow API server on port %d", port)
	Info("API endpoints:")
	Info("  GET  /api/info           - Server information")
	Info("  POST /api/convert        - Convert single image (JSON)")
	Info("  POST /api/upload         - Upload and convert image (multipart)")
	Info("  POST /api/batch          - Batch convert images")
	Info("  GET  /api/status/:id     - Get task status")
	Info("  GET  /api/tasks          - List all tasks")
	Info("  GET  /api/download/:filename - Download video")
	Info("  POST /api/stop/:id       - Stop a task")
	Info("  POST /api/stop           - Stop all tasks")

	return server.Start()
}

// handleUpload handles file upload for conversion
func (s *APIServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse multipart form
	maxSize := int64(50 * 1024 * 1024) // 50MB max
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	if err := r.ParseMultipartForm(maxSize); err != nil {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse form: %v", err),
		})
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("No image file provided: %v", err),
		})
		return
	}
	defer file.Close()

	// Save to temp file
	ext := filepath.Ext(header.Filename)
	tempFile, err := os.CreateTemp("", "luminaflow_upload_*"+ext)
	if err != nil {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create temp file: %v", err),
		})
		return
	}
	defer os.Remove(tempFile.Name())

	if _, err := io.Copy(tempFile, file); err != nil {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to save file: %v", err),
		})
		return
	}
	tempFile.Close()

	// Get optional parameters
	prompt := r.FormValue("prompt")
	duration := 0
	if d := r.FormValue("duration"); d != "" {
		fmt.Sscanf(d, "%d", &duration)
	}
	resolution := r.FormValue("resolution")

	// Submit convert task
	req := ConvertRequest{
		ImagePath:  tempFile.Name(),
		Prompt:     prompt,
		Duration:   duration,
		Resolution: resolution,
	}

	taskID := s.submitConvertTask(req)
	if taskID == "" {
		json.NewEncoder(w).Encode(ConvertResponse{
			Success: false,
			Error:   "Failed to submit task",
		})
		return
	}

	json.NewEncoder(w).Encode(ConvertResponse{
		Success: true,
		TaskID:  taskID,
		Message: "Image uploaded and task submitted",
	})
}
