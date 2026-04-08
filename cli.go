// CLI UI - 终端版本（无 Fyne 依赖）
package main

import (
	"fmt"
	"os"
	"time"
)

// CLIUI provides a terminal-based user interface
type CLIUI struct {
	processor *Processor
	config    *Config
}

// NewCLIUI creates a new CLI interface
func NewCLIUI(processor *Processor, config *Config) *CLIUI {
	return &CLIUI{
		processor: processor,
		config:    config,
	}
}

// Run starts the CLI interface
func (cli *CLIUI) Run() error {
	// Check API key
	if cli.config.APIKey == "" {
		fmt.Println("错误: 未设置 API Key")
		fmt.Println("请设置环境变量 DMXAPI_API_KEY 或创建 .env 文件")
		fmt.Println()
		fmt.Println("创建 .env 文件示例:")
		fmt.Println("  echo DMXAPI_API_KEY=your_api_key > .env")
		return fmt.Errorf("API key not configured")
	}

	// Show config
	fmt.Printf("配置信息:\n")
	fmt.Printf("  API Key: %s***\n", maskString(cli.config.APIKey))
	fmt.Printf("  输出目录: %s\n", cli.config.OutputDir)
	fmt.Printf("  并发数: %d\n", cli.config.Concurrency)
	fmt.Printf("  模型: %s\n", cli.config.Model)
	fmt.Printf("  时长: %d秒\n", cli.config.Duration)
	fmt.Printf("  分辨率: %s\n", cli.config.Resolution)
	fmt.Println()

	// Find data directory
	dataDir := "."
	possiblePaths := []string{"./data", "./input", "./images", "data", "input"}
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			dataDir = path
			break
		}
	}

	images, err := ScanImages(dataDir)
	if err != nil || len(images) == 0 {
		fmt.Printf("从 %s 扫描图片失败或没有找到图片\n", dataDir)
		fmt.Println("请将图片文件放入 data 目录")
		return fmt.Errorf("no images found")
	}

	fmt.Printf("找到 %d 个图片文件:\n", len(images))
	for _, img := range images {
		fmt.Printf("  - %s (%dx%d)\n", img.Filename, img.Width, img.Height)
	}
	fmt.Println()

	// Process images
	cli.processImages(images)

	return nil
}

func maskString(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:8]
}

func (cli *CLIUI) processImages(images []ImageInfo) {
	// Add images to processor
	cli.processor.AddImages(images)

	fmt.Printf("开始处理 %d 个图片...\n", len(images))
	fmt.Println()

	// Start processing
	cli.processor.Start()

	// Event loop
	for {
		tasks := cli.processor.GetTasks()

		// Check if all done
		allDone := true
		for _, t := range tasks {
			if t.State != StateDone && t.State != StateFailed {
				allDone = false
				break
			}
		}

		// Print progress
		cli.printProgress(tasks)

		if allDone {
			break
		}

		time.Sleep(1 * time.Second)
	}

	// Final summary
	fmt.Println()
	fmt.Println("处理完成!")
	fmt.Println()

	tasks := cli.processor.GetTasks()
	cli.printSummary(tasks)
}

func (cli *CLIUI) printProgress(tasks []*Task) {
	processing := 0
	for _, t := range tasks {
		switch t.State {
		case StateEncoding, StateSubmitting, StateProcessing, StateDownloading:
			processing++
		}
	}

	activeStr := ""
	if processing > 0 {
		activeStr = fmt.Sprintf(" [%d 处理中]", processing)
	}

	fmt.Printf("\r已加载: %d | 处理中: %d | 完成: %d | 失败: %d%s",
		len(tasks), processing, cli.countState(tasks, StateDone), cli.countState(tasks, StateFailed), activeStr)
}

func (cli *CLIUI) countState(tasks []*Task, state TaskState) int {
	count := 0
	for _, t := range tasks {
		if t.State == state {
			count++
		}
	}
	return count
}

func (cli *CLIUI) printSummary(tasks []*Task) {
	successCount := 0
	failCount := 0

	fmt.Println("------------------------------------------------")
	for _, t := range tasks {
		if t.State == StateDone {
			successCount++
			fmt.Printf("[成功] %s\n", t.Image.Filename)
		} else if t.State == StateFailed {
			failCount++
			fmt.Printf("[失败] %s: %v\n", t.Image.Filename, t.Error)
		}
	}
	fmt.Println("------------------------------------------------")
	fmt.Printf("\n总计: %d | 成功: %d | 失败: %d\n", len(tasks), successCount, failCount)
}

// RunCLI is the main entry point for CLI mode
func RunCLI(config *Config) error {
	// Initialize logger with configured level
	if err := InitLogger(config.LogLevel); err != nil {
		// Continue without file logging if it fails
	}
	defer CloseLogger()

	Info("LuminaFlow CLI mode starting")

	if err := config.EnsureOutputDir(); err != nil {
		return fmt.Errorf("无法创建输出目录: %w", err)
	}

	dataDir := "."
	possiblePaths := []string{"./data", "./input", "./images", "data", "input"}
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			dataDir = path
			break
		}
	}

	images, err := ScanImages(dataDir)
	if err != nil {
		return fmt.Errorf("扫描图片失败: %w", err)
	}

	if len(images) == 0 {
		return fmt.Errorf("没有找到图片文件，请将图片放入 data 目录")
	}

	processor := NewProcessor(config)
	cliUI := NewCLIUI(processor, config)

	return cliUI.Run()
}
