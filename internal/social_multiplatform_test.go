package internal

import (
	"strings"
	"testing"
)

func TestParsePlatforms(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []string
		shouldError bool
	}{
		{
			name:     "single platform",
			input:    "bluesky",
			expected: []string{"bluesky"},
		},
		{
			name:     "multiple platforms",
			input:    "bluesky,mastodon",
			expected: []string{"bluesky", "mastodon"},
		},
		{
			name:     "all platforms",
			input:    "all",
			expected: []string{"bluesky", "mastodon"},
		},
		{
			name:     "platforms with spaces",
			input:    "bluesky, mastodon",
			expected: []string{"bluesky", "mastodon"},
		},
		{
			name:     "duplicate platforms",
			input:    "bluesky,bluesky,mastodon",
			expected: []string{"bluesky", "mastodon"},
		},
		{
			name:        "invalid platform",
			input:       "twitter",
			shouldError: true,
		},
		{
			name:        "mixed valid and invalid",
			input:       "bluesky,twitter",
			shouldError: true,
		},
		{
			name:        "empty string",
			input:       "",
			shouldError: true,
		},
		{
			name:        "only commas",
			input:       ",,",
			shouldError: true,
		},
		{
			name:     "case insensitive",
			input:    "BLUESKY,Mastodon",
			expected: []string{"bluesky", "mastodon"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePlatforms(tt.input)
			
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				return
			}
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d platforms, got %d", len(tt.expected), len(result))
				return
			}
			
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected platform %q at index %d, got %q", expected, i, result[i])
				}
			}
		})
	}
}

func TestGetAllPlatformNames(t *testing.T) {
	platforms := GetAllPlatformNames()
	
	// Should contain at least bluesky and mastodon
	expectedPlatforms := []string{"bluesky", "mastodon"}
	
	if len(platforms) < len(expectedPlatforms) {
		t.Errorf("Expected at least %d platforms, got %d", len(expectedPlatforms), len(platforms))
	}
	
	for _, expected := range expectedPlatforms {
		found := false
		for _, platform := range platforms {
			if platform == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected platform %q not found in platform list", expected)
		}
	}
}

func TestParsePlatformsErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		errorContains string
	}{
		{
			name:          "unsupported platform",
			input:         "twitter",
			errorContains: "unsupported platform",
		},
		{
			name:          "empty platform",
			input:         "",
			errorContains: "empty",
		},
		{
			name:          "mixed valid and invalid",
			input:         "bluesky,facebook,mastodon",
			errorContains: "facebook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePlatforms(tt.input)
			
			if err == nil {
				t.Errorf("Expected error for input %q", tt.input)
				return
			}
			
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errorContains)) {
				t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
			}
		})
	}
}

func TestParsePlatformsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single platform with trailing comma",
			input:    "bluesky,",
			expected: []string{"bluesky"},
		},
		{
			name:     "single platform with leading comma",
			input:    ",bluesky",
			expected: []string{"bluesky"},
		},
		{
			name:     "platforms with multiple commas",
			input:    "bluesky,,mastodon",
			expected: []string{"bluesky", "mastodon"},
		},
		{
			name:     "platforms with tabs and spaces",
			input:    "bluesky\t,\t mastodon ",
			expected: []string{"bluesky", "mastodon"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePlatforms(tt.input)
			
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				return
			}
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d platforms, got %d", len(tt.expected), len(result))
				return
			}
			
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected platform %q at index %d, got %q", expected, i, result[i])
				}
			}
		})
	}
}

func TestParsePlatformsConsistency(t *testing.T) {
	// Test that ParsePlatforms("all") returns the same platforms as GetAllPlatformNames()
	allFromParse, err := ParsePlatforms("all")
	if err != nil {
		t.Fatalf("ParsePlatforms('all') should not error: %v", err)
	}
	
	allFromGetter := GetAllPlatformNames()
	
	if len(allFromParse) != len(allFromGetter) {
		t.Errorf("ParsePlatforms('all') returned %d platforms, GetAllPlatformNames() returned %d", 
			len(allFromParse), len(allFromGetter))
	}
	
	for i, platform := range allFromParse {
		if i >= len(allFromGetter) || platform != allFromGetter[i] {
			t.Errorf("Platform mismatch at index %d: ParsePlatforms='%s', GetAllPlatformNames='%s'", 
				i, platform, allFromGetter[i])
		}
	}
}

func TestPlatformValidation(t *testing.T) {
	// Test that all platforms returned by GetAllPlatformNames() are valid
	allPlatforms := GetAllPlatformNames()
	
	for _, platform := range allPlatforms {
		t.Run("validate_"+platform, func(t *testing.T) {
			// Each platform should have a client
			_, exists := GetClient(platform)
			if !exists {
				t.Errorf("Platform %q should have a client implementation", platform)
			}
			
			// Each platform should be parseable individually
			parsed, err := ParsePlatforms(platform)
			if err != nil {
				t.Errorf("Platform %q should be parseable: %v", platform, err)
			}
			
			if len(parsed) != 1 || parsed[0] != platform {
				t.Errorf("Parsing platform %q should return [%q], got %v", platform, platform, parsed)
			}
		})
	}
}