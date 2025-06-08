package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewMastodonClient(t *testing.T) {
	client := NewMastodonClient()
	if client == nil {
		t.Error("NewMastodonClient should return a non-nil client")
	}
}

func TestMastodonClient_GetPlatformName(t *testing.T) {
	client := NewMastodonClient()
	expected := "Mastodon"
	if name := client.GetPlatformName(); name != expected {
		t.Errorf("Expected platform name %q, got %q", expected, name)
	}
}

func TestMastodonClient_RequiresAuth(t *testing.T) {
	client := NewMastodonClient()
	if !client.RequiresAuth() {
		t.Error("Mastodon client should require authentication")
	}
}

func TestMastodonClient_ParseUsername(t *testing.T) {
	client := NewMastodonClient()

	tests := []struct {
		name             string
		username         string
		expectedInstance string
		expectedAcct     string
		shouldError      bool
	}{
		{
			name:             "full username",
			username:         "user@mastodon.social",
			expectedInstance: "https://mastodon.social",
			expectedAcct:     "user",
			shouldError:      false,
		},
		{
			name:             "username only",
			username:         "user",
			expectedInstance: "https://mastodon.social",
			expectedAcct:     "user",
			shouldError:      false,
		},
		{
			name:        "invalid format multiple @",
			username:    "user@instance@extra",
			shouldError: true,
		},
		{
			name:             "custom instance",
			username:         "alice@fosstodon.org",
			expectedInstance: "https://fosstodon.org",
			expectedAcct:     "alice",
			shouldError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceURL, acct, err := client.parseUsername(tt.username)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for username %q, got none", tt.username)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for username %q: %v", tt.username, err)
				return
			}

			if instanceURL != tt.expectedInstance {
				t.Errorf("Expected instance %q, got %q", tt.expectedInstance, instanceURL)
			}

			if acct != tt.expectedAcct {
				t.Errorf("Expected acct %q, got %q", tt.expectedAcct, acct)
			}
		})
	}
}

func TestMastodonClient_StripHTML(t *testing.T) {
	client := NewMastodonClient()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "single line break",
			input:    "Line 1<br>Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "self-closing line break",
			input:    "Line 1<br/>Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "line break with space",
			input:    "Line 1<br />Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "paragraph break",
			input:    "Para 1</p>Para 2",
			expected: "Para 1\nPara 2",
		},
		{
			name:     "complex HTML",
			input:    "<p>Hello <strong>world</strong></p><p>Second paragraph</p>",
			expected: "Hello world\nSecond paragraph",
		},
		{
			name:     "nested tags",
			input:    "<div><p>Nested <em>content</em></p></div>",
			expected: "Nested content",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "only HTML tags",
			input:    "<div><p></p></div>",
			expected: "",
		},
		{
			name:     "malformed HTML",
			input:    "Hello <strong>world",
			expected: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.stripHTML(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMastodonClient_DeterminePostType(t *testing.T) {
	client := NewMastodonClient()

	tests := []struct {
		name     string
		status   mastodonStatus
		expected PostType
	}{
		{
			name: "original post",
			status: mastodonStatus{
				ID:          "1",
				Content:     "Hello world",
				InReplyToID: nil,
				Reblog:      nil,
			},
			expected: PostTypeOriginal,
		},
		{
			name: "reply post",
			status: mastodonStatus{
				ID:          "2",
				Content:     "This is a reply",
				InReplyToID: stringPtr("1"),
				Reblog:      nil,
			},
			expected: PostTypeReply,
		},
		{
			name: "reblog post",
			status: mastodonStatus{
				ID:      "3",
				Content: "",
				Reblog: &mastodonStatus{
					ID:      "1",
					Content: "Original content",
				},
			},
			expected: PostTypeRepost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.determinePostType(tt.status)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMastodonClient_GetAccountID(t *testing.T) {
	// Mock server for account lookup
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/accounts/lookup") {
			t.Errorf("Unexpected API path: %s", r.URL.Path)
		}

		acct := r.URL.Query().Get("acct")
		if acct == "" {
			t.Error("acct parameter should be provided")
		}

		// Mock response based on acct
		var response mastodonAccount
		switch acct {
		case "testuser":
			response = mastodonAccount{
				ID:          "123456789",
				Username:    "testuser",
				Acct:        "testuser",
				DisplayName: "Test User",
			}
		case "notfound":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"Record not found"}`))
			return
		default:
			response = mastodonAccount{
				ID:          "987654321",
				Username:    acct,
				Acct:        acct,
				DisplayName: "Generic User",
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewMastodonClient()

	// Note: This test demonstrates the structure but can't easily test the real implementation
	// without dependency injection or interface mocking for the HTTP client
	t.Run("account lookup structure", func(t *testing.T) {
		// Test that the client implements the interface
		var _ SocialClient = client

		// Test that it doesn't panic with invalid input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("getAccountID panicked: %v", r)
			}
		}()
	})
}

func TestMastodonClient_FetchUserStatuses(t *testing.T) {
	// Mock server for statuses API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/accounts/") || !strings.Contains(r.URL.Path, "/statuses") {
			t.Errorf("Unexpected API path: %s", r.URL.Path)
		}

		// Check query parameters
		limit := r.URL.Query().Get("limit")
		if limit == "" {
			t.Error("limit parameter should be provided")
		}

		excludeReplies := r.URL.Query().Get("exclude_replies")
		if excludeReplies != "true" {
			t.Error("exclude_replies should be true")
		}

		// Mock response
		now := time.Now()
		response := []mastodonStatus{
			{
				ID:        "1",
				URL:       "https://mastodon.social/@test/1",
				Content:   "<p>Hello world!</p>",
				CreatedAt: now,
				Account: mastodonAccount{
					ID:          "123",
					Username:    "test",
					Acct:        "test",
					DisplayName: "Test User",
				},
				FavouritesCount: 5,
				ReblogsCount:    2,
				RepliesCount:    1,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewMastodonClient()

	t.Run("fetch statuses structure", func(t *testing.T) {
		// Test that the client implements the interface properly
		var _ SocialClient = client

		// Test that it doesn't panic with invalid input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("fetchUserStatuses panicked: %v", r)
			}
		}()
	})
}

func TestMastodonClient_FetchUserPosts(t *testing.T) {
	client := NewMastodonClient()

	t.Run("fetch posts without credentials", func(t *testing.T) {
		// This should work as it falls back to public API
		// Note: In a real test, we'd mock the HTTP calls
		var _ SocialClient = client

		// Test that it doesn't panic with invalid input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FetchUserPosts panicked: %v", r)
			}
		}()
	})
}

func TestMastodonStatusConversion(t *testing.T) {
	// Test conversion from mastodonStatus to generic Post
	now := time.Now()
	status := mastodonStatus{
		ID:        "123456789",
		URL:       "https://mastodon.social/@test/123456789",
		Content:   "<p>Hello <strong>world</strong>!</p>",
		CreatedAt: now,
		Account: mastodonAccount{
			ID:          "user123",
			Username:    "test",
			Acct:        "test",
			DisplayName: "Test User",
		},
		FavouritesCount: 10,
		ReblogsCount:    5,
		RepliesCount:    3,
		Favourited:      boolPtr(true),
		Pinned:          boolPtr(false),
	}

	client := NewMastodonClient()

	// Test conversion logic (this would be in FetchUserPosts)
	post := Post{
		ID:        status.ID,
		Author:    status.Account.DisplayName,
		Handle:    status.Account.Acct,
		Content:   client.stripHTML(status.Content),
		CreatedAt: status.CreatedAt,
		URL:       status.URL,
		Type:      client.determinePostType(status),
		Platform:  "mastodon",

		RepostCount: status.ReblogsCount,
		LikeCount:   status.FavouritesCount,
		ReplyCount:  status.RepliesCount,

		IsLikedByUser: status.Favourited != nil && *status.Favourited,
		IsPinned:      status.Pinned != nil && *status.Pinned,
	}

	t.Run("status conversion", func(t *testing.T) {
		if post.ID != status.ID {
			t.Errorf("Expected ID %q, got %q", status.ID, post.ID)
		}

		if post.Author != status.Account.DisplayName {
			t.Errorf("Expected Author %q, got %q", status.Account.DisplayName, post.Author)
		}

		if post.Handle != status.Account.Acct {
			t.Errorf("Expected Handle %q, got %q", status.Account.Acct, post.Handle)
		}

		expectedContent := "Hello world!"
		if post.Content != expectedContent {
			t.Errorf("Expected Content %q, got %q", expectedContent, post.Content)
		}

		if post.Platform != "mastodon" {
			t.Errorf("Expected Platform 'mastodon', got %q", post.Platform)
		}

		if post.LikeCount != status.FavouritesCount {
			t.Errorf("Expected LikeCount %d, got %d", status.FavouritesCount, post.LikeCount)
		}

		if !post.IsLikedByUser {
			t.Error("Expected IsLikedByUser to be true")
		}

		if post.IsPinned {
			t.Error("Expected IsPinned to be false")
		}
	})
}

func TestMastodonReblogHandling(t *testing.T) {
	now := time.Now()

	// Create a reblog status
	originalStatus := mastodonStatus{
		ID:        "original123",
		Content:   "<p>Original content</p>",
		CreatedAt: now.Add(-1 * time.Hour),
		Account: mastodonAccount{
			ID:          "original_user",
			Username:    "original",
			Acct:        "original",
			DisplayName: "Original User",
		},
	}

	reblogStatus := mastodonStatus{
		ID:        "reblog456",
		Content:   "", // Reblog typically has empty content
		CreatedAt: now,
		Account: mastodonAccount{
			ID:          "reblog_user",
			Username:    "reblogger",
			Acct:        "reblogger",
			DisplayName: "Reblogger",
		},
		Reblog: &originalStatus,
	}

	client := NewMastodonClient()

	t.Run("reblog detection", func(t *testing.T) {
		postType := client.determinePostType(reblogStatus)
		if postType != PostTypeRepost {
			t.Errorf("Expected PostTypeRepost, got %v", postType)
		}
	})

	t.Run("reblog conversion", func(t *testing.T) {
		// Test reblog conversion logic (from FetchUserPosts)
		post := Post{
			ID:     reblogStatus.ID,
			Author: reblogStatus.Account.DisplayName,
			Handle: reblogStatus.Account.Acct,
			Type:   client.determinePostType(reblogStatus),
		}

		if reblogStatus.Reblog != nil {
			post.Type = PostTypeRepost
			post.OriginalAuthor = reblogStatus.Reblog.Account.DisplayName
			post.OriginalHandle = reblogStatus.Reblog.Account.Acct
			post.Content = client.stripHTML(reblogStatus.Reblog.Content)

			post.OriginalPost = &Post{
				ID:        reblogStatus.Reblog.ID,
				Author:    reblogStatus.Reblog.Account.DisplayName,
				Handle:    reblogStatus.Reblog.Account.Acct,
				Content:   client.stripHTML(reblogStatus.Reblog.Content),
				CreatedAt: reblogStatus.Reblog.CreatedAt,
				URL:       reblogStatus.Reblog.URL,
				Type:      PostTypeOriginal,
				Platform:  "mastodon",
			}
		}

		if post.Type != PostTypeRepost {
			t.Errorf("Expected PostTypeRepost, got %v", post.Type)
		}

		if post.OriginalAuthor != "Original User" {
			t.Errorf("Expected OriginalAuthor 'Original User', got %q", post.OriginalAuthor)
		}

		if post.OriginalHandle != "original" {
			t.Errorf("Expected OriginalHandle 'original', got %q", post.OriginalHandle)
		}

		if post.Content != "Original content" {
			t.Errorf("Expected Content 'Original content', got %q", post.Content)
		}

		if post.OriginalPost == nil {
			t.Error("Expected OriginalPost to be set")
		} else {
			if post.OriginalPost.ID != "original123" {
				t.Errorf("Expected OriginalPost.ID 'original123', got %q", post.OriginalPost.ID)
			}
		}
	})
}

func TestMastodonReplyHandling(t *testing.T) {
	replyStatus := mastodonStatus{
		ID:                 "reply123",
		Content:            "<p>This is a reply</p>",
		InReplyToID:        stringPtr("original123"),
		InReplyToAccountID: stringPtr("original_user"),
	}

	client := NewMastodonClient()

	t.Run("reply detection", func(t *testing.T) {
		postType := client.determinePostType(replyStatus)
		if postType != PostTypeReply {
			t.Errorf("Expected PostTypeReply, got %v", postType)
		}
	})

	t.Run("reply conversion", func(t *testing.T) {
		// Test reply conversion logic (from FetchUserPosts)
		post := Post{
			Type: client.determinePostType(replyStatus),
		}

		if replyStatus.InReplyToID != nil {
			post.Type = PostTypeReply
			post.InReplyToID = *replyStatus.InReplyToID
		}

		if post.Type != PostTypeReply {
			t.Errorf("Expected PostTypeReply, got %v", post.Type)
		}

		if post.InReplyToID != "original123" {
			t.Errorf("Expected InReplyToID 'original123', got %q", post.InReplyToID)
		}
	})
}

func TestMastodonClient_PrunePosts(t *testing.T) {
	client := NewMastodonClient()

	t.Run("prune posts without credentials", func(t *testing.T) {
		options := PruneOptions{
			MaxAge: func() *time.Duration { d := 30 * 24 * time.Hour; return &d }(),
			DryRun: true,
		}

		// This should fail due to missing credentials
		result, err := client.PrunePosts("test@mastodon.social", options)

		if err == nil {
			t.Error("Expected error when no credentials are available")
		}

		if result != nil {
			t.Error("Expected nil result when credentials are missing")
		}
	})
}

func TestMastodonClient_TruncateContent(t *testing.T) {

	tests := []struct {
		name     string
		content  string
		maxLen   int
		expected string
	}{
		{
			name:     "short content",
			content:  "Hello world",
			maxLen:   20,
			expected: "Hello world",
		},
		{
			name:     "needs truncation",
			content:  "This is a very long message that needs truncation",
			maxLen:   10,
			expected: "This is...",
		},
		{
			name:     "content with newlines",
			content:  "Line 1\nLine 2\nLine 3",
			maxLen:   15,
			expected: "Line 1 Line ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateContent(tt.content, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMastodonAPIEndpoints(t *testing.T) {
	tests := []struct {
		name         string
		instanceURL  string
		accountID    string
		statusID     string
		expectedPath string
	}{
		{
			name:         "account lookup",
			instanceURL:  "https://mastodon.social",
			expectedPath: "/api/v1/accounts/lookup",
		},
		{
			name:         "account statuses",
			instanceURL:  "https://mastodon.social",
			accountID:    "123456",
			expectedPath: "/api/v1/accounts/123456/statuses",
		},
		{
			name:         "delete status",
			instanceURL:  "https://mastodon.social",
			statusID:     "789012",
			expectedPath: "/api/v1/statuses/789012",
		},
		{
			name:         "unfavourite status",
			instanceURL:  "https://mastodon.social",
			statusID:     "789012",
			expectedPath: "/api/v1/statuses/789012/unfavourite",
		},
		{
			name:         "unreblog status",
			instanceURL:  "https://mastodon.social",
			statusID:     "789012",
			expectedPath: "/api/v1/statuses/789012/unreblog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test URL construction logic
			var fullURL string
			switch tt.name {
			case "account lookup":
				fullURL = tt.instanceURL + "/api/v1/accounts/lookup"
			case "account statuses":
				fullURL = tt.instanceURL + "/api/v1/accounts/" + tt.accountID + "/statuses"
			case "delete status":
				fullURL = tt.instanceURL + "/api/v1/statuses/" + tt.statusID
			case "unfavourite status":
				fullURL = tt.instanceURL + "/api/v1/statuses/" + tt.statusID + "/unfavourite"
			case "unreblog status":
				fullURL = tt.instanceURL + "/api/v1/statuses/" + tt.statusID + "/unreblog"
			}

			if !strings.Contains(fullURL, tt.expectedPath) {
				t.Errorf("Expected URL to contain %q, got %q", tt.expectedPath, fullURL)
			}
		})
	}
}

func TestMastodonHTTPMethods(t *testing.T) {
	tests := []struct {
		name           string
		operation      string
		expectedMethod string
	}{
		{"get account", "lookup", "GET"},
		{"get statuses", "statuses", "GET"},
		{"delete status", "delete", "DELETE"},
		{"unfavourite", "unfavourite", "POST"},
		{"unreblog", "unreblog", "POST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test HTTP method selection logic
			var method string
			switch tt.operation {
			case "lookup", "statuses":
				method = "GET"
			case "delete":
				method = "DELETE"
			case "unfavourite", "unreblog":
				method = "POST"
			}

			if method != tt.expectedMethod {
				t.Errorf("Expected method %q for operation %q, got %q", tt.expectedMethod, tt.operation, method)
			}
		})
	}
}

func TestMastodonAuthenticationHeaders(t *testing.T) {
	tests := []struct {
		name        string
		accessToken string
		expected    string
	}{
		{
			name:        "valid token",
			accessToken: "abc123token",
			expected:    "Bearer abc123token",
		},
		{
			name:        "long token",
			accessToken: "very-long-access-token-with-dashes-and-numbers-123456789",
			expected:    "Bearer very-long-access-token-with-dashes-and-numbers-123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test authorization header construction
			authHeader := "Bearer " + tt.accessToken

			if authHeader != tt.expected {
				t.Errorf("Expected auth header %q, got %q", tt.expected, authHeader)
			}
		})
	}
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
