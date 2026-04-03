//go:build gui
// +build gui

package main

import (
	"fyne.io/fyne/v2/app"
)

func main() {
	// Initialize logger
	if err := InitLogger(); err != nil {
		// Continue without file logging if it fails
	}
	defer CloseLogger()

	Info("LuminaFlow starting...")

	// Load configuration
	config := LoadConfig()

	// Create Fyne application
	fyneApp := app.NewWithID("com.luminaflow.app")

	// Create processor
	processor := NewProcessor(config)

	// Create UI
	ui := NewUI(fyneApp, processor, config)

	// Check if API key is configured
	if config.APIKey == "" {
		// Show settings on first launch if no API key
		Info("No API key configured, showing settings dialog")
		ui.window.Show()
		ui.onSettings()
	} else {
		ui.Show()
	}

	Info("LuminaFlow UI ready")

	// Run the application
	ui.Run()
}

// RunCLI is defined in ui.go for CLI mode
