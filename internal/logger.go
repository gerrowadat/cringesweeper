package internal

import (
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger provides a global logger instance for the application
var Logger zerolog.Logger

// InitLogger initializes the global logger with appropriate configuration
// Uses LOG_LEVEL environment variable, defaulting to INFO
func InitLogger() {
	// Configure log level from environment, default to INFO
	logLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	if logLevel == "" {
		logLevel = "INFO"
	}
	InitLoggerWithLevel(logLevel)
}

// InitLoggerWithLevel initializes the global logger with a specific log level
func InitLoggerWithLevel(logLevelStr string) {
	logLevel := strings.ToUpper(logLevelStr)
	var level zerolog.Level
	
	switch logLevel {
	case "DEBUG":
		level = zerolog.DebugLevel
	case "INFO":
		level = zerolog.InfoLevel
	case "WARN":
		level = zerolog.WarnLevel
	case "ERROR":
		level = zerolog.ErrorLevel
	default:
		level = zerolog.InfoLevel
	}

	// Configure console output with colors if in terminal
	output := zerolog.ConsoleWriter{Out: os.Stdout}
	output.TimeFormat = "15:04:05"
	
	// Create logger with timestamp and level
	Logger = zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Logger()
	
	// Also set the global log package logger
	log.Logger = Logger
	
	Logger.Debug().
		Str("level", level.String()).
		Msg("Logger initialized")
}

// GetLogger returns the configured logger instance
func GetLogger() *zerolog.Logger {
	return &Logger
}

// WithPlatform creates a logger with platform context
func WithPlatform(platform string) *zerolog.Logger {
	logger := Logger.With().Str("platform", platform).Logger()
	return &logger
}

// WithHTTP creates a logger with HTTP context
func WithHTTP(method, url string) *zerolog.Logger {
	redactedURL := RedactSensitiveURL(url)
	logger := Logger.With().
		Str("http_method", method).
		Str("url", redactedURL).
		Logger()
	return &logger
}

// RedactSensitiveURL redacts sensitive information from URLs for logging
func RedactSensitiveURL(url string) string {
	// Redact Authorization tokens and passwords in query params
	re := regexp.MustCompile(`([?&])(password|token|access_token|refresh_token|bearer|authorization)=([^&]+)`)
	url = re.ReplaceAllString(url, "${1}${2}=***REDACTED***")
	
	// Redact any potential tokens in path segments (like JWT tokens)
	re = regexp.MustCompile(`/eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	url = re.ReplaceAllString(url, "/***JWT_TOKEN***")
	
	// Redact anything that looks like an app password (typically long alphanumeric strings)
	re = regexp.MustCompile(`([?&])(app[_-]?password|apppassword)=([^&]+)`)
	url = re.ReplaceAllString(url, "${1}${2}=***REDACTED***")
	
	return url
}

// WithOperation creates a logger with operation context
func WithOperation(operation string) *zerolog.Logger {
	logger := Logger.With().Str("operation", operation).Logger()
	return &logger
}

// LogHTTPRequest logs an HTTP request at DEBUG level with full URL and redaction
func LogHTTPRequest(method, url string) {
	redactedURL := RedactSensitiveURL(url)
	Logger.Debug().
		Str("http_method", method).
		Str("url", redactedURL).
		Msg("Making HTTP request")
}

// LogHTTPResponse logs an HTTP response at DEBUG level
func LogHTTPResponse(method, url string, statusCode int, status string) {
	redactedURL := RedactSensitiveURL(url)
	Logger.Debug().
		Str("http_method", method).
		Str("url", redactedURL).
		Int("status_code", statusCode).
		Str("status", status).
		Msg("HTTP request completed")
}