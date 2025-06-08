package internal

import (
	"strings"
	"testing"
	"net/url"
)

func TestRedactSensitiveURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "password parameter",
			input:    "https://example.com/api?password=secret123&other=value",
			expected: "https://example.com/api?password=***REDACTED***&other=value",
		},
		{
			name:     "token parameter",
			input:    "https://example.com/api?token=abc123&normal=param",
			expected: "https://example.com/api?token=***REDACTED***&normal=param",
		},
		{
			name:     "access_token parameter",
			input:    "https://example.com/api?access_token=xyz789",
			expected: "https://example.com/api?access_token=***REDACTED***",
		},
		{
			name:     "bearer parameter",
			input:    "https://example.com/api?bearer=bearer123",
			expected: "https://example.com/api?bearer=***REDACTED***",
		},
		{
			name:     "app_password parameter",
			input:    "https://example.com/api?app_password=app123",
			expected: "https://example.com/api?app_password=***REDACTED***",
		},
		{
			name:     "JWT token in path",
			input:    "https://example.com/path/eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature/more",
			expected: "https://example.com/path/***JWT_TOKEN***/more",
		},
		{
			name:     "multiple sensitive parameters",
			input:    "https://example.com/api?password=secret&token=abc&normal=value",
			expected: "https://example.com/api?password=***REDACTED***&token=***REDACTED***&normal=value",
		},
		{
			name:     "no sensitive parameters",
			input:    "https://example.com/api?normal=value&other=param",
			expected: "https://example.com/api?normal=value&other=param",
		},
		{
			name:     "empty URL",
			input:    "",
			expected: "",
		},
		{
			name:     "malformed URL",
			input:    "not-a-url",
			expected: "not-a-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSensitiveURL(tt.input)
			if result != tt.expected {
				t.Errorf("RedactSensitiveURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWithPlatform(t *testing.T) {
	// Initialize logger for testing
	InitLogger()
	
	logger := WithPlatform("bluesky")
	
	// Test that logger context can be created without error
	if logger == nil {
		t.Error("WithPlatform should return a logger instance")
	}
}

func TestWithHTTP(t *testing.T) {
	// Initialize logger for testing
	InitLogger()
	
	logger := WithHTTP("GET", "https://example.com/api")
	
	// Test that logger context can be created without error
	if logger == nil {
		t.Error("WithHTTP should return a logger instance")
	}
}

func TestLogHTTPRequest(t *testing.T) {
	// Initialize logger for testing
	InitLogger()
	
	// Test that function doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("LogHTTPRequest panicked: %v", r)
		}
	}()
	
	LogHTTPRequest("GET", "https://example.com/api")
}

func TestLogHTTPResponse(t *testing.T) {
	// Initialize logger for testing
	InitLogger()
	
	// Test that function doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("LogHTTPResponse panicked: %v", r)
		}
	}()
	
	LogHTTPResponse("GET", "https://example.com/api", 200, "OK")
}

func TestRedactSensitiveURLEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "case insensitive parameters",
			input:    "https://example.com/api?PASSWORD=secret&TOKEN=abc",
			expected: "https://example.com/api?PASSWORD=***REDACTED***&TOKEN=***REDACTED***",
		},
		{
			name:     "URL with fragment",
			input:    "https://example.com/api?password=secret#fragment",
			expected: "https://example.com/api?password=***REDACTED***#fragment",
		},
		{
			name:     "URL with port",
			input:    "https://example.com:8080/api?token=secret",
			expected: "https://example.com:8080/api?token=***REDACTED***",
		},
		{
			name:     "JWT token at beginning of path",
			input:    "https://example.com/eyJhbGciOiJIUzI1NiJ9.payload.sig",
			expected: "https://example.com/***JWT_TOKEN***",
		},
		{
			name:     "partial JWT pattern should not match",
			input:    "https://example.com/eyJ",
			expected: "https://example.com/eyJ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSensitiveURL(tt.input)
			if result != tt.expected {
				t.Errorf("RedactSensitiveURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInitLoggerWithLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
		{"invalid level", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that function doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("InitLoggerWithLevel panicked with level %q: %v", tt.level, r)
				}
			}()
			
			InitLoggerWithLevel(tt.level)
		})
	}
}