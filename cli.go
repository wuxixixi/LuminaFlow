// CLI UI - 终端版本（无 Fyne 依赖）
package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// CLI color codes
const (
	cliReset   = "\033[0m"
	cliRed     = "\033[31m"
	cliGreen   = "\033[32m"
	cliYellow  = "\033[33m"
	cliBlue    = "\033[34m"
	cliMagenta = "\033[35m"
	cliCyan    = "\033[36m"
	cliWhite   = "\033[37m"
	cliBold    = "\033[1m"
	cliDim     = "\033[2m"
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
		cli.printError("错误: 未设置 API Key")
		fmt.Println()
		fmt.Println("请设置环境变量 DMXAPI_API_KEY 或创建 .env 文件")
		fmt.Println()
		fmt.Println("创建 .env 文件示例:")
		fmt.Printf("  %secho DMXAPI_API_KEY=your_api_key > .env%s\n", cliDim, cliReset)
		return fmt.Errorf("API key not configured")
	}

	// Show config with colors
	cli.printHeader("配置信息")
	fmt.Printf("  %sAPI Key:%s %s***%s\n", cliCyan, cliReset, cli.config.APIKey[:min(8, len(cli.config.APIKey))], cliReset)
	fmt.Printf("  %s输出目录:%s %s\n", cliCyan, cliReset, cli.config.OutputDir)
	fmt.Printf("  %s并发数:%s %d\n", cliCyan, cliReset, cli.config.Concurrency)
	fmt.Printf("  %s模型:%s %s\n", cliCyan, cliReset, cli.config.Model)
	fmt.Printf("  %s时长:%s %d秒\n", cliCyan, cliReset, cli.config.Duration)
	fmt.Printf("  %s分辨率:%s %s\n", cliCyan, cliReset, cli.config.Resolution)
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
		cli.printError("从 %s 扫描图片失败或没有找到图片", dataDir)
		fmt.Println("请将图片文件放入 data 目录")
		return fmt.Errorf("no images found")
	}

	cli.printSuccess("找到 %d 个图片文件:", len(images))
	for _, img := range images {
		fmt.Printf("  %s•%s %s (%s%d%sx%s%d%s)\n",
			cliGreen, cliReset, img.Filename,
			cliDim, img.Width, cliReset,
			cliDim, img.Height, cliReset)
	}
	fmt.Println()

	// Ask for confirmation before processing
	fmt.Print("确认开始转换? (y/n): ")
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Println("已取消操作")
		return nil
	}
	fmt.Println()

	// Process images
	cli.processImages(images)

	return nil
}

func (cli *CLIUI) printError(format string, args ...interface{}) {
	fmt.Printf("%s%s✗ %s%s\n", cliBold, cliRed, cliReset, fmt.Sprintf(format, args...))
}

func (cli *CLIUI) printSuccess(format string, args ...interface{}) {
	fmt.Printf("%s%s✓ %s%s\n", cliBold, cliGreen, cliReset, fmt.Sprintf(format, args...))
}

func (cli *CLIUI) printHeader(text string) {
	fmt.Printf("\n%s%s▌ %s %s\n", cliBold, cliBlue, text, cliReset)
	fmt.Printf("%s%s%s%s\n", cliDim, strings.Repeat("─", 40), cliReset, cliReset)
}

func (cli *CLIUI) processImages(images []ImageInfo) {
	// Add images to processor
	cli.processor.AddImages(images)

	cli.printHeader("开始处理")
	fmt.Printf("共 %d 个图片待处理...\n\n", len(images))

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

		time.Sleep(500 * time.Millisecond)
	}

	// Final summary
	fmt.Println()
	fmt.Println()
	cli.printSummary(cli.processor.GetTasks())
}

func (cli *CLIUI) printProgress(tasks []*Task) {
	total := len(tasks)
	done := cli.countState(tasks, StateDone)
	failed := cli.countState(tasks, StateFailed)
	processing := 0
	pending := 0

	for _, t := range tasks {
		switch t.State {
		case StateEncoding, StateSubmitting, StateProcessing, StateDownloading:
			processing++
		case StatePending:
			pending++
		}
	}

	// Progress bar
	width := 30
	completed := done + failed
	progress := float64(completed) / float64(total)
	filled := int(progress * float64(width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	// Status line with colors
	fmt.Printf("\r%s%s%s [%s%s%s] %s%d/%d%s │ ",
		cliDim, bar, cliReset,
		cliGreen, fmt.Sprintf("%.0f%%", progress*100), cliReset,
		cliCyan, completed, total, cliReset)

	// Color-coded counts
	fmt.Printf("%s完成:%d%s %s失败:%d%s %s处理中:%d%s %s等待:%d%s",
		cliGreen, done, cliReset,
		cliRed, failed, cliReset,
		cliYellow, processing, cliReset,
		cliDim, pending, cliReset)

	// Clear rest of line
	fmt.Print("   ")
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

	cli.printHeader("处理结果")

	for _, t := range tasks {
		if t.State == StateDone {
			successCount++
			fmt.Printf("  %s✓%s %s%s%s → %s\n",
				cliGreen, cliReset,
				cliBold, t.Image.Filename, cliReset,
				t.OutputPath)
		} else if t.State == StateFailed {
			failCount++
			fmt.Printf("  %s✗%s %s%s%s: %s%s%s\n",
				cliRed, cliReset,
				cliBold, t.Image.Filename, cliReset,
				cliRed, t.Error, cliReset)
		}
	}

	fmt.Println()
	fmt.Printf("  %s总计:%s %d  %s成功:%s %d  %s失败:%s %d\n",
		cliCyan, cliReset, len(tasks),
		cliGreen, cliReset, successCount,
		cliRed, cliReset, failCount)
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

	// Show images first
	fmt.Printf("找到 %d 个图片文件:\n", len(images))
	for _, img := range images {
		fmt.Printf("  - %s (%dx%d)\n", img.Filename, img.Width, img.Height)
	}
	fmt.Println()

	// Ask for confirmation
	fmt.Print("确认开始转换? (y/n): ")
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Println("已取消操作")
		return nil
	}

	processor := NewProcessor(config)
	cliUI := NewCLIUI(processor, config)

	return cliUI.Run()
}
