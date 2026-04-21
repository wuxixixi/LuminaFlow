package main

import (
	"context"
	"os"
	"sync"
	"time"
)

// TaskState represents the current state of a processing task
type TaskState int

const (
	StatePending TaskState = iota
	StateEncoding
	StateSubmitting
	StateProcessing
	StateDownloading
	StateDone
	StateFailed
	StateCancelled
)

func (s TaskState) String() string {
	switch s {
	case StatePending:
		return "等待中"
	case StateEncoding:
		return "编码中"
	case StateSubmitting:
		return "提交中"
	case StateProcessing:
		return "处理中"
	case StateDownloading:
		return "下载中"
	case StateDone:
		return "已完成"
	case StateFailed:
		return "失败"
	case StateCancelled:
		return "已取消"
	default:
		return "未知"
	}
}

// TaskEvent represents an event from the processing engine
type TaskEvent struct {
	Filename   string
	State      TaskState
	Error      error
	OutputPath string
	Elapsed    time.Duration
}

// Task represents a single image processing task
type Task struct {
	Image      ImageInfo
	State      TaskState
	Error      error
	OutputPath string
	StartedAt  time.Time
	FinishedAt time.Time
	Selected   bool // Whether this task is selected for processing
}

// Processor manages the batch processing of images
type Processor struct {
	config     *Config
	apiClient  *APIClient
	tasks      []*Task
	taskEvents chan TaskEvent
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
}

// NewProcessor creates a new batch processor
func NewProcessor(config *Config) *Processor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Processor{
		config:     config,
		apiClient:  NewAPIClient(config.APIKey),
		tasks:      make([]*Task, 0),
		taskEvents: make(chan TaskEvent, 100),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// AddImages adds images to the processing queue
func (p *Processor) AddImages(images []ImageInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, img := range images {
		// Check if output video already exists
		outputPath := GetOutputPath(img.Path, p.config.OutputDir)
		var state TaskState = StatePending
		if _, err := os.Stat(outputPath); err == nil {
			state = StateDone
		}

		task := &Task{
			Image:      img,
			State:      state,
			Selected:   true, // New images are selected by default
			OutputPath: outputPath,
		}
		p.tasks = append(p.tasks, task)
	}
}

// ClearTasks removes all tasks
func (p *Processor) ClearTasks() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tasks = make([]*Task, 0)
}

// SetTasks sets the task list (used for sorting)
func (p *Processor) SetTasks(tasks []*Task) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tasks = tasks
}

// GetTasks returns a copy of current tasks
func (p *Processor) GetTasks() []*Task {
	p.mu.Lock()
	defer p.mu.Unlock()
	tasks := make([]*Task, len(p.tasks))
	copy(tasks, p.tasks)
	return tasks
}

// GetTaskCount returns the number of tasks in each state
func (p *Processor) GetTaskCount() (total, pending, processing, done, failed int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, task := range p.tasks {
		total++
		switch task.State {
		case StatePending:
			pending++
		case StateEncoding, StateSubmitting, StateProcessing, StateDownloading:
			processing++
		case StateDone:
			done++
		case StateFailed:
			failed++
		}
	}
	return
}

// Events returns the channel for task events
func (p *Processor) Events() <-chan TaskEvent {
	return p.taskEvents
}

// Start begins processing all pending selected tasks
func (p *Processor) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.mu.Unlock()

	// Start worker pool
	workerCount := p.config.Concurrency
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > 4 {
		workerCount = 4
	}

	taskQueue := make(chan *Task, len(p.tasks))

	// Add pending selected tasks to queue
	p.mu.Lock()
	selectedCount := 0
	for _, task := range p.tasks {
		if task.State == StatePending && task.Selected {
			taskQueue <- task
			selectedCount++
		}
	}
	p.mu.Unlock()

	if selectedCount == 0 {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
		return
	}

	// Start workers
	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i, taskQueue)
	}

	// Close queue when all tasks are added
	go func() {
		p.wg.Wait()
		close(taskQueue)
	}()
}

// Stop cancels all in-progress tasks
func (p *Processor) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	p.cancel()

	// Update cancelled tasks
	p.mu.Lock()
	for _, task := range p.tasks {
		if task.State != StateDone && task.State != StateFailed {
			task.State = StateCancelled
		}
	}
	p.mu.Unlock()

	p.wg.Wait()
}

// IsRunning returns whether the processor is currently running
func (p *Processor) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// RetryTask resets a failed task to pending
func (p *Processor) RetryTask(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= 0 && index < len(p.tasks) {
		if p.tasks[index].State == StateFailed {
			p.tasks[index].State = StatePending
			p.tasks[index].Error = nil
		}
	}
}

// SetTaskSelected sets the selection state of a task
func (p *Processor) SetTaskSelected(index int, selected bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= 0 && index < len(p.tasks) {
		p.tasks[index].Selected = selected
	}
}

// SelectAllTasks selects or deselects all pending tasks
func (p *Processor) SelectAllTasks(selected bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, task := range p.tasks {
		if task.State == StatePending || task.State == StateFailed {
			task.Selected = selected
		}
	}
}

// GetSelectedCount returns the number of selected pending tasks
func (p *Processor) GetSelectedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := 0
	for _, task := range p.tasks {
		if task.Selected && task.State == StatePending {
			count++
		}
	}
	return count
}

// RemoveTask removes a task from the list by index
func (p *Processor) RemoveTask(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= 0 && index < len(p.tasks) {
		p.tasks = append(p.tasks[:index], p.tasks[index+1:]...)
	}
}

// worker processes tasks from the queue
func (p *Processor) worker(id int, tasks chan *Task) {
	defer p.wg.Done()

	for task := range tasks {
		select {
		case <-p.ctx.Done():
			// Context cancelled, stop processing
			p.mu.Lock()
			if task.State != StateDone {
				task.State = StateCancelled
				p.taskEvents <- TaskEvent{
					Filename: task.Image.Filename,
					State:    StateCancelled,
				}
			}
			p.mu.Unlock()
			return
		default:
		}

		p.processTask(task)
	}
}

// processTask handles a single image-to-video conversion
func (p *Processor) processTask(task *Task) {
	task.StartedAt = time.Now()

	// Update state: Encoding
	p.updateTaskState(task, StateEncoding)

	// Get output path
	outputPath := GetOutputPath(task.Image.Path, p.config.OutputDir)

	// Process with API
	err := p.apiClient.ConvertImageToVideo(
		p.ctx,
		task.Image.Base64,
		p.config.Prompt,
		outputPath,
		p.config.Duration,
		p.config.Resolution,
	)

	task.FinishedAt = time.Now()

	if err != nil {
		// Check if cancelled
		if p.ctx.Err() != nil {
			p.mu.Lock()
			task.State = StateCancelled
			p.mu.Unlock()
			p.taskEvents <- TaskEvent{
				Filename: task.Image.Filename,
				State:    StateCancelled,
			}
			return
		}

		// Update state: Failed
		p.mu.Lock()
		task.State = StateFailed
		task.Error = err
		p.mu.Unlock()

		p.taskEvents <- TaskEvent{
			Filename: task.Image.Filename,
			State:    StateFailed,
			Error:    err,
		}
		return
	}

	// Update state: Done
	p.mu.Lock()
	task.State = StateDone
	task.OutputPath = outputPath
	p.mu.Unlock()

	p.taskEvents <- TaskEvent{
		Filename:   task.Image.Filename,
		State:      StateDone,
		OutputPath: outputPath,
		Elapsed:    task.FinishedAt.Sub(task.StartedAt),
	}
}

// updateTaskState safely updates a task's state and emits an event
func (p *Processor) updateTaskState(task *Task, state TaskState) {
	p.mu.Lock()
	task.State = state
	p.mu.Unlock()

	p.taskEvents <- TaskEvent{
		Filename: task.Image.Filename,
		State:    state,
	}
}
