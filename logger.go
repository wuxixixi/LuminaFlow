package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Logger provides logging functionality
type Logger struct {
	file   *os.File
	logger *log.Logger
}

var appLogger *Logger

// InitLogger initializes the application logger
func InitLogger() error {
	// Create logs directory
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Create log file with date
	logFile := filepath.Join(logDir, fmt.Sprintf("luminaflow_%s.log", time.Now().Format("2006-01-02")))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// Log to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, file)
	logger := log.New(multiWriter, "", log.LstdFlags|log.Lshortfile)

	appLogger = &Logger{
		file:   file,
		logger: logger,
	}

	Info("Logger initialized")
	return nil
}

// CloseLogger closes the log file
func CloseLogger() {
	if appLogger != nil && appLogger.file != nil {
		appLogger.file.Close()
	}
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	if appLogger != nil {
		appLogger.logger.SetPrefix("[INFO] ")
		appLogger.logger.Output(2, fmt.Sprintf(format, args...))
	}
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	if appLogger != nil {
		appLogger.logger.SetPrefix("[ERROR] ")
		appLogger.logger.Output(2, fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	if appLogger != nil {
		appLogger.logger.SetPrefix("[WARN] ")
		appLogger.logger.Output(2, fmt.Sprintf(format, args...))
	}
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	if appLogger != nil {
		appLogger.logger.SetPrefix("[DEBUG] ")
		appLogger.logger.Output(2, fmt.Sprintf(format, args...))
	}
}
