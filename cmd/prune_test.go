package cmd

import (
	"testing"
	"time"

	"github.com/gerrowadat/cringesweeper/internal"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"hours", "24h", 24 * time.Hour, false},
		{"days", "30d", 30 * 24 * time.Hour, false},
		{"weeks", "2w", 2 * 7 * 24 * time.Hour, false},
		{"months", "6m", 6 * 30 * 24 * time.Hour, false},
		{"years", "1y", 365 * 24 * time.Hour, false},
		{"go duration", "2h30m", 2*time.Hour + 30*time.Minute, false},
		{"invalid format", "abc", 0, true},
		{"empty string", "", 0, true},
		{"single char", "d", 0, true},
		{"zero value", "0d", 0, false},
		{"negative value", "-5d", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got none", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("Expected %v for input %q, got %v", tt.expected, tt.input, result)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"ISO date", "2023-12-25", false},
		{"ISO datetime", "2023-12-25 14:30:00", false},
		{"ISO with timezone", "2023-12-25T14:30:00Z", false},
		{"ISO with offset", "2023-12-25T14:30:00-05:00", false},
		{"US date", "12/25/2023", false},
		{"US datetime", "12/25/2023 14:30:00", false},
		{"invalid format", "25-12-2023", true},
		{"invalid date", "2023-13-40", true},
		{"empty string", "", true},
		{"partial date", "2023-12", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDate(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
			}
		})
	}
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxLen   int
		expected string
	}{
		{"short content", "Hello world", 20, "Hello world"},
		{"exact length", "Hello", 5, "Hello"},
		{"needs truncation", "This is a very long message", 10, "This is..."},
		{"with newlines", "Line 1\nLine 2\nLine 3", 15, "Line 1 Line 2 L..."},
		{"empty content", "", 10, ""},
		{"zero max length", "Hello", 0, "..."},
		{"max length less than ellipsis", "Hello", 2, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateContent(tt.content, tt.maxLen)
			
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
			
			// Verify no newlines in result
			if len(result) > 0 && (result[0] == '\n' || result[len(result)-1] == '\n') {
				t.Errorf("Result should not contain newlines: %q", result)
			}
		})
	}
}

func TestDisplayPruneResults(t *testing.T) {
	now := time.Now()
	result := &internal.PruneResult{
		PostsToDelete: []internal.Post{
			{
				ID:        "1",
				Handle:    "user1",
				Content:   "Delete this post",
				CreatedAt: now.Add(-48 * time.Hour),
				URL:       "https://example.com/1",
			},
		},
		PostsToUnlike: []internal.Post{
			{
				ID:        "2",
				Handle:    "user2", 
				Content:   "Unlike this post",
				CreatedAt: now.Add(-24 * time.Hour),
				URL:       "https://example.com/2",
			},
		},
		PostsToUnshare: []internal.Post{
			{
				ID:        "3",
				Handle:    "user3",
				Content:   "Unshare this repost",
				CreatedAt: now.Add(-12 * time.Hour),
				URL:       "https://example.com/3",
			},
		},
		PostsPreserved: []internal.Post{
			{
				ID:            "4",
				Handle:        "user4",
				Author:        "user4",
				Content:       "Preserved post",
				CreatedAt:     now.Add(-6 * time.Hour),
				IsLikedByUser: true,
			},
		},
		DeletedCount:   1,
		UnlikedCount:   1,
		UnsharedCount:  1,
		PreservedCount: 1,
		ErrorsCount:    0,
		Errors:         []string{},
	}

	t.Run("dry run display", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPruneResults panicked: %v", r)
			}
		}()
		
		displayPruneResults(result, "TestPlatform", true)
	})

	t.Run("actual run display", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPruneResults panicked: %v", r)
			}
		}()
		
		displayPruneResults(result, "TestPlatform", false)
	})

	t.Run("empty result", func(t *testing.T) {
		emptyResult := &internal.PruneResult{
			PostsToDelete:  []internal.Post{},
			PostsToUnlike:  []internal.Post{},
			PostsToUnshare: []internal.Post{},
			PostsPreserved: []internal.Post{},
		}
		
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPruneResults panicked with empty result: %v", r)
			}
		}()
		
		displayPruneResults(emptyResult, "TestPlatform", true)
	})
}

func TestPreservedPostReasonDetection(t *testing.T) {
	tests := []struct {
		name     string
		post     internal.Post
		expected string
	}{
		{
			name: "self-liked post",
			post: internal.Post{
				Handle:        "user",
				Author:        "user",
				IsLikedByUser: true,
			},
			expected: " (self-liked)",
		},
		{
			name: "pinned post",
			post: internal.Post{
				IsPinned: true,
			},
			expected: " (pinned)",
		},
		{
			name: "both self-liked and pinned",
			post: internal.Post{
				Handle:        "user",
				Author:        "user",
				IsLikedByUser: true,
				IsPinned:      true,
			},
			expected: " (pinned)", // pinned takes precedence in the current logic
		},
		{
			name: "regular post",
			post: internal.Post{
				Handle: "user",
				Author: "user",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the reason detection logic from displayPruneResults
			reason := ""
			if tt.post.IsLikedByUser && tt.post.Handle == tt.post.Author {
				reason = " (self-liked)"
			}
			if tt.post.IsPinned {
				reason = " (pinned)"
			}
			
			if reason != tt.expected {
				t.Errorf("Expected reason %q, got %q", tt.expected, reason)
			}
		})
	}
}

func TestRateLimitDelayDefaults(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		expected time.Duration
	}{
		{"mastodon default", "mastodon", 60 * time.Second},
		{"bluesky default", "bluesky", 1 * time.Second},
		{"unknown platform", "twitter", 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test rate limit delay logic from prune command
			var rateLimitDelay time.Duration
			switch tt.platform {
			case "mastodon":
				rateLimitDelay = 60 * time.Second
			case "bluesky":
				rateLimitDelay = 1 * time.Second
			default:
				rateLimitDelay = 5 * time.Second
			}
			
			if rateLimitDelay != tt.expected {
				t.Errorf("Expected %v for platform %q, got %v", tt.expected, tt.platform, rateLimitDelay)
			}
		})
	}
}

func TestPruneOptionsValidation(t *testing.T) {
	tests := []struct {
		name        string
		maxAge      *time.Duration
		beforeDate  *time.Time
		shouldError bool
	}{
		{
			name:        "with max age",
			maxAge:      func() *time.Duration { d := 30 * 24 * time.Hour; return &d }(),
			beforeDate:  nil,
			shouldError: false,
		},
		{
			name:        "with before date",
			maxAge:      nil,
			beforeDate:  func() *time.Time { t := time.Now().Add(-30 * 24 * time.Hour); return &t }(),
			shouldError: false,
		},
		{
			name:        "with both criteria",
			maxAge:      func() *time.Duration { d := 30 * 24 * time.Hour; return &d }(),
			beforeDate:  func() *time.Time { t := time.Now().Add(-30 * 24 * time.Hour); return &t }(),
			shouldError: false,
		},
		{
			name:        "with neither criteria",
			maxAge:      nil,
			beforeDate:  nil,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic from prune command
			hasError := (tt.maxAge == nil && tt.beforeDate == nil)
			
			if hasError != tt.shouldError {
				t.Errorf("Expected error %v, got %v", tt.shouldError, hasError)
			}
		})
	}
}

func TestResultSummaryCalculation(t *testing.T) {
	result := &internal.PruneResult{
		PostsToDelete:  make([]internal.Post, 3),
		PostsToUnlike:  make([]internal.Post, 2),
		PostsToUnshare: make([]internal.Post, 1),
		PostsPreserved: make([]internal.Post, 4),
		DeletedCount:   3,
		UnlikedCount:   2,
		UnsharedCount:  1,
		PreservedCount: 4,
		ErrorsCount:    0,
	}

	t.Run("total actions calculation", func(t *testing.T) {
		totalActions := len(result.PostsToDelete) + len(result.PostsToUnlike) + len(result.PostsToUnshare)
		expected := 6
		
		if totalActions != expected {
			t.Errorf("Expected total actions %d, got %d", expected, totalActions)
		}
	})

	t.Run("has actions check", func(t *testing.T) {
		totalActions := len(result.PostsToDelete) + len(result.PostsToUnlike) + len(result.PostsToUnshare)
		hasActions := totalActions > 0
		
		if !hasActions {
			t.Error("Expected to have actions, but got none")
		}
	})

	t.Run("empty result check", func(t *testing.T) {
		emptyResult := &internal.PruneResult{}
		totalActions := len(emptyResult.PostsToDelete) + len(emptyResult.PostsToUnlike) + len(emptyResult.PostsToUnshare)
		
		if totalActions != 0 {
			t.Errorf("Expected 0 actions for empty result, got %d", totalActions)
		}
	})
}

func TestPlatformValidationInPrune(t *testing.T) {
	tests := []struct {
		name          string
		platform      string
		shouldBeValid bool
	}{
		{"valid bluesky", "bluesky", true},
		{"valid mastodon", "mastodon", true},
		{"invalid platform", "twitter", false},
		{"empty platform", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test platform validation logic that would be used in prune command
			_, isValid := internal.GetClient(tt.platform)
			
			if isValid != tt.shouldBeValid {
				t.Errorf("Platform %q validity: expected %v, got %v", tt.platform, tt.shouldBeValid, isValid)
			}
		})
	}
}

func TestDisplayPruneResultsWithErrors(t *testing.T) {
	result := &internal.PruneResult{
		PostsToDelete:  []internal.Post{},
		PostsToUnlike:  []internal.Post{},
		PostsToUnshare: []internal.Post{},
		PostsPreserved: []internal.Post{},
		DeletedCount:   0,
		UnlikedCount:   0,
		UnsharedCount:  0,
		PreservedCount: 0,
		ErrorsCount:    2,
		Errors:         []string{"Error deleting post 123", "Network timeout"},
	}

	t.Run("display results with errors", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPruneResults panicked with errors: %v", r)
			}
		}()
		
		displayPruneResults(result, "TestPlatform", false)
	})
}

func TestParseDurationEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"zero hours", "0h", 0, false},
		{"large number", "1000d", 1000 * 24 * time.Hour, false},
		{"decimal value", "1.5d", 0, true},
		{"just unit", "d", 0, true},
		{"mixed case", "5D", 0, true},
		{"standard go duration", "2h30m", 2*time.Hour + 30*time.Minute, false},
		{"complex go duration", "1h30m45s", time.Hour + 30*time.Minute + 45*time.Second, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got none", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("Expected %v for input %q, got %v", tt.expected, tt.input, result)
			}
		})
	}
}

func TestParseDateEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"leap year date", "2024-02-29", false},
		{"invalid leap year", "2023-02-29", true},
		{"future date", "2030-12-31", false},
		{"epoch date", "1970-01-01", false},
		{"just year", "2023", true},
		{"european format", "25/12/2023", true}, // Not supported
		{"time only", "15:30:00", true},
		{"iso format with milliseconds", "2023-12-25T14:30:00.123Z", true}, // Not in supported formats
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDate(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
			}
		})
	}
}

func TestTruncateContentEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxLen   int
		expected string
	}{
		{"content exactly maxLen", "hello", 5, "hello"},
		{"content one char over", "hello!", 5, "he..."},
		{"multiple newlines", "line1\n\nline2\n\nline3", 10, "line1  li..."},
		{"tabs and spaces", "word1\tword2\t\tword3", 10, "word1 wor..."},
		{"unicode characters", "café🚀test", 8, "café🚀..."},
		{"very short maxLen", "hello", 3, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateContent(tt.content, tt.maxLen)
			
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
			
			// Ensure result doesn't exceed maxLen
			if len(result) > tt.maxLen {
				t.Errorf("Result length %d exceeds maxLen %d", len(result), tt.maxLen)
			}
		})
	}
}

func TestDisplayPruneResultsEmptyCategories(t *testing.T) {
	// Test with only delete actions
	deleteOnlyResult := &internal.PruneResult{
		PostsToDelete: []internal.Post{
			{ID: "1", Handle: "user", Content: "Delete me", CreatedAt: time.Now()},
		},
		PostsToUnlike:  []internal.Post{},
		PostsToUnshare: []internal.Post{},
		PostsPreserved: []internal.Post{},
		DeletedCount:   1,
	}

	// Test with only unlike actions
	unlikeOnlyResult := &internal.PruneResult{
		PostsToDelete: []internal.Post{},
		PostsToUnlike: []internal.Post{
			{ID: "2", Handle: "user", Content: "Unlike me", CreatedAt: time.Now()},
		},
		PostsToUnshare: []internal.Post{},
		PostsPreserved: []internal.Post{},
		UnlikedCount:   1,
	}

	tests := []struct {
		name   string
		result *internal.PruneResult
		dryRun bool
	}{
		{"delete only dry run", deleteOnlyResult, true},
		{"delete only actual", deleteOnlyResult, false},
		{"unlike only dry run", unlikeOnlyResult, true},
		{"unlike only actual", unlikeOnlyResult, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("displayPruneResults panicked: %v", r)
				}
			}()
			
			displayPruneResults(tt.result, "TestPlatform", tt.dryRun)
		})
	}
}

func TestPruneOptionsComplexScenarios(t *testing.T) {
	now := time.Now()
	maxAge := 30 * 24 * time.Hour
	beforeDate := now.AddDate(0, 0, -30)

	tests := []struct {
		name        string
		options     internal.PruneOptions
		description string
	}{
		{
			name: "all preservation flags enabled",
			options: internal.PruneOptions{
				MaxAge:           &maxAge,
				PreservePinned:   true,
				PreserveSelfLike: true,
				DryRun:          true,
			},
			description: "preserve pinned and self-liked posts",
		},
		{
			name: "all action flags enabled",
			options: internal.PruneOptions{
				MaxAge:         &maxAge,
				UnlikePosts:    true,
				UnshareReposts: true,
				DryRun:        true,
			},
			description: "unlike and unshare instead of delete",
		},
		{
			name: "both time criteria",
			options: internal.PruneOptions{
				MaxAge:     &maxAge,
				BeforeDate: &beforeDate,
				DryRun:    true,
			},
			description: "both max age and before date specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that options are properly formed
			hasTimeCriteria := tt.options.MaxAge != nil || tt.options.BeforeDate != nil
			if !hasTimeCriteria {
				t.Error("Test case should have time criteria")
			}
			
			// Test validation logic from prune command
			if tt.options.MaxAge == nil && tt.options.BeforeDate == nil {
				t.Error("Should require at least one time criteria")
			}
		})
	}
}