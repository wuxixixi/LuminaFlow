package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel parses a string to LogLevel
func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// Logger provides logging functionality
type Logger struct {
	file      *os.File
	logger    *log.Logger
	level     LogLevel
}

var appLogger *Logger

// InitLogger initializes the application logger
func InitLogger(level string) error {
	// Create logs directory
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Clean up old log files (keep last 7 days)
	cleanupOldLogs(logDir, 7)

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
		level:  ParseLogLevel(level),
	}

	Info("Logger initialized with level: %s", appLogger.level)
	return nil
}

// cleanupOldLogs removes log files older than maxDays
func cleanupOldLogs(logDir string, maxDays int) {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -maxDays)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		info, err := file.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(logDir, file.Name()))
		}
	}
}

// CloseLogger closes the log file
func CloseLogger() {
	if appLogger != nil && appLogger.file != nil {
		appLogger.file.Close()
	}
}

// SetLogLevel sets the current log level
func SetLogLevel(level LogLevel) {
	if appLogger != nil {
		appLogger.level = level
	}
}

// GetLogLevel returns the current log level
func GetLogLevel() LogLevel {
	if appLogger != nil {
		return appLogger.level
	}
	return LogLevelInfo
}

// log writes a log message if the level is sufficient
func logMessage(level LogLevel, format string, args ...interface{}) {
	if appLogger == nil {
		return
	}
	if level < appLogger.level {
		return
	}

	appLogger.logger.SetPrefix(fmt.Sprintf("[%s] ", level))
	appLogger.logger.Output(2, fmt.Sprintf(format, args...))
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	logMessage(LogLevelInfo, format, args...)
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	logMessage(LogLevelError, format, args...)
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	logMessage(LogLevelWarn, format, args...)
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	logMessage(LogLevelDebug, format, args...)
}
