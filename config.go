package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Application version
const (
	AppName    = "LuminaFlow"
	AppVersion = "1.2.0"
)

// Config holds all application settings
type Config struct {
	APIKey      string `json:"api_key"`
	OutputDir   string `json:"output_dir"`
	Concurrency int    `json:"concurrency"`
	Prompt      string `json:"prompt"`
	Duration    int    `json:"duration"`
	Resolution  string `json:"resolution"`
	Model       string `json:"model"`
}

// PromptTemplate represents a preset prompt template
type PromptTemplate struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// Default prompt templates
var PromptTemplates = []PromptTemplate{
	{
		Name:   "电影推镜头",
		Prompt: "缓慢、平滑的电影级推镜头。摄像机笔直向前推进。保持完全原始的构图、光影和主体。高分辨率，真实的景深，保持结构几何形状完美稳定，无扭曲。",
	},
	{
		Name:   "自然动态",
		Prompt: "让画面自然地动起来，保持原始画面的风格、色彩和构图，添加微妙的自然动态效果。",
	},
	{
		Name:   "缓慢拉远",
		Prompt: "缓慢、平滑的电影级拉镜头。摄像机笔直向后拉远。保持原始构图和主体，景深自然，画面稳定。",
	},
	{
		Name:   "左右平移",
		Prompt: "平滑的横向平移镜头。摄像机从左向右缓慢移动。保持画面稳定，构图完整。",
	},
	{
		Name:   "自定义",
		Prompt: "",
	},
}

// DefaultConfig returns default configuration values
func DefaultConfig() *Config {
	cwd, _ := os.Getwd()
	outputDir := filepath.Join(cwd, "output")

	return &Config{
		APIKey:      "",
		OutputDir:   outputDir,
		Concurrency: 2,
		Prompt:      PromptTemplates[0].Prompt,
		Duration:    6,
		Resolution:  "768P",
		Model:       "MiniMax-Hailuo-2.3",
	}
}

// LoadConfig loads configuration from .env file and saved config
func LoadConfig() *Config {
	config := DefaultConfig()

	// Try to load .env file for API key
	_ = godotenv.Load()
	if apiKey := os.Getenv("DMXAPI_API_KEY"); apiKey != "" {
		config.APIKey = apiKey
	}

	// Load saved configuration
	config.loadFromFile()

	return config
}

// ConfigFile returns the path to the config file
func ConfigFile() string {
	// Use AppData directory for Windows
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = "."
	}
	configDir := filepath.Join(appData, "LuminaFlow")
	os.MkdirAll(configDir, 0755)
	return filepath.Join(configDir, "config.json")
}

// loadFromFile loads configuration from JSON file
func (c *Config) loadFromFile() {
	data, err := os.ReadFile(ConfigFile())
	if err != nil {
		return
	}

	var saved Config
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}

	// Only load non-empty values
	if saved.APIKey != "" {
		c.APIKey = saved.APIKey
	}
	if saved.OutputDir != "" {
		c.OutputDir = saved.OutputDir
	}
	if saved.Concurrency > 0 {
		c.Concurrency = saved.Concurrency
	}
	if saved.Prompt != "" {
		c.Prompt = saved.Prompt
	}
	if saved.Duration > 0 {
		c.Duration = saved.Duration
	}
	if saved.Resolution != "" {
		c.Resolution = saved.Resolution
	}

	Info("Configuration loaded from %s", ConfigFile())
}

// Save saves configuration to JSON file
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	configFile := ConfigFile()
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return err
	}

	Info("Configuration saved to %s", configFile)
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return nil // API key will trigger settings dialog
	}
	return nil
}

// EnsureOutputDir creates the output directory if it doesn't exist
func (c *Config) EnsureOutputDir() error {
	return os.MkdirAll(c.OutputDir, 0755)
}
