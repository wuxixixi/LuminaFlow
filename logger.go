package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

// ANSI color codes for console output
const (
	colorReset  = "\033[0m"
	colorDebug  = "\033[36m" // Cyan
	colorInfo   = "\033[32m" // Green
	colorWarn   = "\033[33m" // Yellow
	colorError  = "\033[31m" // Red
	colorTime   = "\033[90m" // Dark gray
	colorPrefix = "\033[1m"  // Bold
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

// Logger provides logging functionality with colored console output
type Logger struct {
	file     *os.File
	logger   *log.Logger
	level    LogLevel
	useColor bool
	mu       sync.Mutex
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

	// Log to both file and stdout with colored output
	appLogger = &Logger{
		file:     file,
		level:    ParseLogLevel(level),
		useColor: true,
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
		appLogger.mu.Lock()
		appLogger.level = level
		appLogger.mu.Unlock()
	}
}

// GetLogLevel returns the current log level
func GetLogLevel() LogLevel {
	if appLogger != nil {
		appLogger.mu.Lock()
		defer appLogger.mu.Unlock()
		return appLogger.level
	}
	return LogLevelInfo
}

// formatLogMessage creates a formatted log message with colors
func (l *Logger) formatLogMessage(level LogLevel, format string, args ...interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)

	var levelStr, colorCode string
	switch level {
	case LogLevelDebug:
		levelStr = "DEBUG"
		colorCode = colorDebug
	case LogLevelInfo:
		levelStr = "INFO"
		colorCode = colorInfo
	case LogLevelWarn:
		levelStr = "WARN"
		colorCode = colorWarn
	case LogLevelError:
		levelStr = "ERROR"
		colorCode = colorError
	}

	// Console output with colors
	if l.useColor {
		return fmt.Sprintf("%s%s%s [%s%s%s] %s%s",
			colorTime, timestamp, colorReset,
			colorPrefix+colorCode, levelStr, colorReset,
			message, colorReset)
	}

	// Plain output for file
	return fmt.Sprintf("%s [%s] %s", timestamp, levelStr, message)
}

// log writes a log message if the level is sufficient
func logMessage(level LogLevel, format string, args ...interface{}) {
	if appLogger == nil {
		return
	}

	appLogger.mu.Lock()
	defer appLogger.mu.Unlock()

	if level < appLogger.level {
		return
	}

	// Format message
	message := appLogger.formatLogMessage(level, format, args...)

	// Write to console
	fmt.Fprintln(os.Stdout, message)

	// Write to file (without colors)
	if appLogger.file != nil {
		// Strip ANSI codes for file output
		cleanMsg := stripANSI(message)
		fmt.Fprintln(appLogger.file, cleanMsg)
	}
}

// stripANSI removes ANSI color codes from a string
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
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
