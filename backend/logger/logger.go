package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Log level constants
const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

var currentLogLevel = levelInfo

func init() {
	level := strings.ToLower(os.Getenv("APPDECK_LOG_LEVEL"))
	switch level {
	case "debug":
		currentLogLevel = levelDebug
	case "info":
		currentLogLevel = levelInfo
	case "warn":
		currentLogLevel = levelWarn
	case "error":
		currentLogLevel = levelError
	default:
		currentLogLevel = levelInfo
	}
	if currentLogLevel == levelDebug {
		log.Println("[logger] Debug logging enabled")
	}
}

// Debug logs at debug level with prefix
func Debug(prefix, format string, args ...interface{}) {
	if currentLogLevel <= levelDebug {
		log.Printf("[%s] DEBUG: %s", prefix, fmt.Sprintf(format, args...))
	}
}

// Info logs at info level with prefix
func Info(prefix, format string, args ...interface{}) {
	if currentLogLevel <= levelInfo {
		log.Printf("[%s] %s", prefix, fmt.Sprintf(format, args...))
	}
}

// Warn logs at warn level with prefix
func Warn(prefix, format string, args ...interface{}) {
	if currentLogLevel <= levelWarn {
		log.Printf("[%s] WARNING: %s", prefix, fmt.Sprintf(format, args...))
	}
}

// Error logs at error level with prefix
func Error(prefix, format string, args ...interface{}) {
	log.Printf("[%s] ERROR: %s", prefix, fmt.Sprintf(format, args...))
}

// IsDebugEnabled returns true if debug logging is active
func IsDebugEnabled() bool {
	return currentLogLevel <= levelDebug
}
