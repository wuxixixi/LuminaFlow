//go:build !gui
// +build !gui

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Command line flags
	showHelp := flag.Bool("h", false, "显示帮助信息")
	flag.Parse()

	if *showHelp {
		printHelp()
		return
	}

	// Load configuration
	config := LoadConfig()

	// CLI mode (default when using this file)
	fmt.Println("================================================")
	fmt.Println("  LuminaFlow - 批量图片转视频工具 (CLI模式)")
	fmt.Println("================================================")
	fmt.Println()

	if err := RunCLI(config); err != nil {
		fmt.Fprintf(os.Stderr, "\n错误: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("LuminaFlow - 批量图片转视频工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  luminaflow              启动 CLI 模式")
	fmt.Println("  luminaflow -h           显示帮助信息")
	fmt.Println()
	fmt.Println("环境变量:")
	fmt.Println("  DMXAPI_API_KEY         DMXAPI API Key (必填)")
	fmt.Println()
	fmt.Println("配置文件:")
	fmt.Println("  创建 .env 文件设置 API Key:")
	fmt.Println("    echo DMXAPI_API_KEY=your_api_key > .env")
	fmt.Println()
	fmt.Println("输入目录:")
	fmt.Println("  程序会自动扫描以下目录中的图片:")
	fmt.Println("    ./data, ./input, ./images")
}
