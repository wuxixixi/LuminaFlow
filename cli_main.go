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
	apiMode := flag.Bool("api", false, "启动 API 服务器模式")
	apiPort := flag.Int("port", 8080, "API 服务器端口 (默认: 8080)")
	flag.Parse()

	if *showHelp {
		printHelp()
		return
	}

	// Load configuration
	config := LoadConfig()

	// API mode
	if *apiMode {
		fmt.Println("================================================")
		fmt.Println("  LuminaFlow - 图片转视频 API 服务器")
		fmt.Println("================================================")
		fmt.Println()
		
		if config.APIKey == "" {
			fmt.Println("错误: 未设置 API Key")
			fmt.Println("请设置环境变量 DMXAPI_API_KEY 或创建 .env 文件")
			os.Exit(1)
		}

		fmt.Printf("API Key: %s***\n", maskAPIKey(config.APIKey))
		fmt.Printf("输出目录: %s\n", config.OutputDir)
		fmt.Println()

		if err := StartAPIServer(config, *apiPort); err != nil {
			fmt.Fprintf(os.Stderr, "\n错误: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// CLI mode (default)
	fmt.Println("================================================")
	fmt.Println("  LuminaFlow - 批量图片转视频工具 (CLI模式)")
	fmt.Println("================================================")
	fmt.Println()

	if err := RunCLI(config); err != nil {
		fmt.Fprintf(os.Stderr, "\n错误: %v\n", err)
		os.Exit(1)
	}
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8]
}

func printHelp() {
	fmt.Println("LuminaFlow - 批量图片转视频工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  luminaflow              启动 CLI 模式 (批量处理)")
	fmt.Println("  luminaflow -api         启动 API 服务器模式")
	fmt.Println("  luminaflow -api -port 9000  指定 API 端口")
	fmt.Println("  luminaflow -h           显示帮助信息")
	fmt.Println()
	fmt.Println("API 模式:")
	fmt.Println("  启动 HTTP API 服务器，提供以下接口:")
	fmt.Println("    GET  /api/info           服务器信息")
	fmt.Println("    POST /api/convert        转换单张图片 (JSON)")
	fmt.Println("    POST /api/upload         上传并转换图片 (multipart)")
	fmt.Println("    POST /api/batch          批量转换")
	fmt.Println("    GET  /api/status/:id     查询任务状态")
	fmt.Println("    GET  /api/tasks          获取所有任务")
	fmt.Println("    GET  /api/download/:name 下载视频")
	fmt.Println("    POST /api/stop/:id       停止任务")
	fmt.Println("    POST /api/stop           停止所有任务")
	fmt.Println()
	fmt.Println("环境变量:")
	fmt.Println("  DMXAPI_API_KEY         DMXAPI API Key (必填)")
	fmt.Println()
	fmt.Println("配置文件:")
	fmt.Println("  创建 .env 文件设置 API Key:")
	fmt.Println("    echo DMXAPI_API_KEY=your_api_key > .env")
	fmt.Println()
	fmt.Println("输入目录 (CLI模式):")
	fmt.Println("  程序会自动扫描以下目录中的图片:")
	fmt.Println("    ./data, ./input, ./images")
}
