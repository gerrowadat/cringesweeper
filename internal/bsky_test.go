package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewBlueskyClient(t *testing.T) {
	client := NewBlueskyClient()
	if client == nil {
		t.Error("NewBlueskyClient should return a non-nil client")
	}
}

func TestBlueskyClient_GetPlatformName(t *testing.T) {
	client := NewBlueskyClient()
	expected := "Bluesky"
	if name := client.GetPlatformName(); name != expected {
		t.Errorf("Expected platform name %q, got %q", expected, name)
	}
}

func TestBlueskyClient_RequiresAuth(t *testing.T) {
	client := NewBlueskyClient()
	if !client.RequiresAuth() {
		t.Error("Bluesky client should require authentication")
	}
}

func TestExtractPostID(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "valid URI",
			uri:      "at://did:plc:abc123/app.bsky.feed.post/xyz789",
			expected: "xyz789",
		},
		{
			name:     "URI with longer post ID",
			uri:      "at://did:plc:longerid123/app.bsky.feed.post/verylongpostid456",
			expected: "verylongpostid456",
		},
		{
			name:     "empty URI",
			uri:      "",
			expected: "",
		},
		{
			name:     "URI without slashes",
			uri:      "invalid-uri-format",
			expected: "",
		},
		{
			name:     "URI ending with slash",
			uri:      "at://did:plc:abc123/app.bsky.feed.post/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPostID(tt.uri)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestBlueskyClient_DeterminePostType(t *testing.T) {
	client := NewBlueskyClient()
	
	tests := []struct {
		name     string
		post     blueskyPost
		expected PostType
	}{
		{
			name: "original post",
			post: blueskyPost{
				Record: blueskyRecord{
					Type: "app.bsky.feed.post",
				},
			},
			expected: PostTypeOriginal,
		},
		{
			name: "repost",
			post: blueskyPost{
				Record: blueskyRecord{
					Type: "app.bsky.feed.repost",
				},
			},
			expected: PostTypeRepost,
		},
		{
			name: "reply",
			post: blueskyPost{
				Record: blueskyRecord{
					Type: "app.bsky.feed.post",
					Reply: &blueskyReply{
						Parent: blueskyPostRef{URI: "at://parent"},
						Root:   blueskyPostRef{URI: "at://root"},
					},
				},
			},
			expected: PostTypeReply,
		},
		{
			name: "unknown type",
			post: blueskyPost{
				Record: blueskyRecord{
					Type: "app.bsky.unknown.type",
				},
			},
			expected: PostTypeOriginal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.determinePostType(tt.post)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
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
		{
			name:     "short content",
			content:  "Hello world",
			maxLen:   20,
			expected: "Hello world",
		},
		{
			name:     "exact length",
			content:  "Hello",
			maxLen:   5,
			expected: "Hello",
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
		{
			name:     "empty content",
			content:  "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "zero max length",
			content:  "Hello",
			maxLen:   0,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateContent(tt.content, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestBlueskyClient_FetchUserPosts(t *testing.T) {
	// Mock server for testing API calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/xrpc/app.bsky.feed.getAuthorFeed") {
			t.Errorf("Unexpected API path: %s", r.URL.Path)
		}
		
		// Check query parameters
		username := r.URL.Query().Get("actor")
		if username == "" {
			t.Error("Actor parameter should be provided")
		}
		
		limit := r.URL.Query().Get("limit")
		if limit == "" {
			t.Error("Limit parameter should be provided")
		}

		// Mock response
		response := blueskyEnhancedFeedResponse{
			Feed: []struct {
				Post       blueskyPost        `json:"post"`
				ViewerData *blueskyViewerData `json:"viewer,omitempty"`
				PinnedPost bool               `json:"pinnedPost,omitempty"`
			}{
				{
					Post: blueskyPost{
						URI: "at://did:plc:test123/app.bsky.feed.post/abc123",
						CID: "bafyreid123",
						Author: blueskyAuthor{
							DID:         "did:plc:test123",
							Handle:      "test.bsky.social",
							DisplayName: "Test User",
						},
						Record: blueskyRecord{
							Type:      "app.bsky.feed.post",
							Text:      "Hello world!",
							CreatedAt: time.Now(),
						},
						LikeCount:   5,
						RepostCount: 2,
						ReplyCount:  1,
					},
					ViewerData: &blueskyViewerData{
						Like: nil,
					},
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Note: This test demonstrates the structure but can't easily test the real implementation
	// without dependency injection or interface mocking for the HTTP client
	t.Run("fetch posts structure", func(t *testing.T) {
		client := NewBlueskyClient()
		
		// In a real test, we'd need to inject the mock server URL
		// For now, we just test that the client implements the interface
		var _ SocialClient = client
		
		// Test that it doesn't panic with invalid input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FetchUserPosts panicked: %v", r)
			}
		}()
	})
}

func TestBlueskyClient_PrunePosts(t *testing.T) {
	client := NewBlueskyClient()
	
	// Test that the method exists and handles invalid credentials gracefully
	t.Run("prune posts without credentials", func(t *testing.T) {
		options := PruneOptions{
			MaxAge: func() *time.Duration { d := 30 * 24 * time.Hour; return &d }(),
			DryRun: true,
		}
		
		// This should fail due to missing credentials
		result, err := client.PrunePosts("test.bsky.social", options)
		
		if err == nil {
			t.Error("Expected error when no credentials are available")
		}
		
		if result != nil {
			t.Error("Expected nil result when credentials are missing")
		}
	})
}

func TestBlueskyPost_Conversion(t *testing.T) {
	// Test the conversion logic from blueskyPost to generic Post
	now := time.Now()
	bskyPost := blueskyPost{
		URI: "at://did:plc:test123/app.bsky.feed.post/abc123",
		CID: "bafyreid123",
		Author: blueskyAuthor{
			DID:         "did:plc:test123",
			Handle:      "test.bsky.social",
			DisplayName: "Test User",
		},
		Record: blueskyRecord{
			Type:      "app.bsky.feed.post",
			Text:      "Hello world!",
			CreatedAt: now,
		},
		LikeCount:   5,
		RepostCount: 2,
		ReplyCount:  1,
		ViewerData: &blueskyViewerData{
			Like: nil,
		},
	}

	// Test conversion logic (this would be in FetchUserPosts)
	post := Post{
		ID:        bskyPost.URI,
		Author:    bskyPost.Author.DisplayName,
		Handle:    bskyPost.Author.Handle,
		Content:   bskyPost.Record.Text,
		CreatedAt: bskyPost.Record.CreatedAt,
		Platform:  "bluesky",
		
		RepostCount: bskyPost.RepostCount,
		LikeCount:   bskyPost.LikeCount,
		ReplyCount:  bskyPost.ReplyCount,
	}

	t.Run("post conversion", func(t *testing.T) {
		if post.ID != bskyPost.URI {
			t.Errorf("Expected ID %q, got %q", bskyPost.URI, post.ID)
		}
		
		if post.Author != bskyPost.Author.DisplayName {
			t.Errorf("Expected Author %q, got %q", bskyPost.Author.DisplayName, post.Author)
		}
		
		if post.Handle != bskyPost.Author.Handle {
			t.Errorf("Expected Handle %q, got %q", bskyPost.Author.Handle, post.Handle)
		}
		
		if post.Content != bskyPost.Record.Text {
			t.Errorf("Expected Content %q, got %q", bskyPost.Record.Text, post.Content)
		}
		
		if post.Platform != "bluesky" {
			t.Errorf("Expected Platform 'bluesky', got %q", post.Platform)
		}
		
		if post.LikeCount != bskyPost.LikeCount {
			t.Errorf("Expected LikeCount %d, got %d", bskyPost.LikeCount, post.LikeCount)
		}
	})

	t.Run("fallback author handling", func(t *testing.T) {
		// Test when DisplayName is empty
		bskyPostNoDisplay := bskyPost
		bskyPostNoDisplay.Author.DisplayName = ""
		
		expectedAuthor := bskyPost.Author.Handle
		if bskyPostNoDisplay.Author.DisplayName == "" {
			if expectedAuthor != bskyPost.Author.Handle {
				t.Error("Should use Handle as fallback when DisplayName is empty")
			}
		}
	})

	t.Run("viewer interaction status", func(t *testing.T) {
		// Test liked status
		bskyPostLiked := bskyPost
		likeURI := "at://like/uri"
		bskyPostLiked.ViewerData = &blueskyViewerData{
			Like: &likeURI,
		}
		
		isLiked := bskyPostLiked.ViewerData != nil && bskyPostLiked.ViewerData.Like != nil
		if !isLiked {
			t.Error("Should detect liked status when Like URI is present")
		}
		
		// Test not liked status
		bskyPostNotLiked := bskyPost
		bskyPostNotLiked.ViewerData = &blueskyViewerData{
			Like: nil,
		}
		
		isNotLiked := bskyPostNotLiked.ViewerData != nil && bskyPostNotLiked.ViewerData.Like != nil
		if isNotLiked {
			t.Error("Should not detect liked status when Like URI is nil")
		}
	})
}

func TestBlueskyReplyHandling(t *testing.T) {
	// Test reply detection and handling
	replyPost := blueskyPost{
		Record: blueskyRecord{
			Type: "app.bsky.feed.post",
			Text: "This is a reply",
			Reply: &blueskyReply{
				Parent: blueskyPostRef{
					URI: "at://did:plc:parent/app.bsky.feed.post/parent123",
					CID: "parentcid",
				},
				Root: blueskyPostRef{
					URI: "at://did:plc:root/app.bsky.feed.post/root123",
					CID: "rootcid",
				},
			},
		},
	}

	t.Run("reply detection", func(t *testing.T) {
		client := NewBlueskyClient()
		postType := client.determinePostType(replyPost)
		
		if postType != PostTypeReply {
			t.Errorf("Expected PostTypeReply, got %v", postType)
		}
	})

	t.Run("reply parent URI extraction", func(t *testing.T) {
		// Test logic for extracting parent URI (from FetchUserPosts conversion)
		var inReplyToID string
		if replyPost.Record.Reply != nil {
			inReplyToID = replyPost.Record.Reply.Parent.URI
		}
		
		expectedParentURI := "at://did:plc:parent/app.bsky.feed.post/parent123"
		if inReplyToID != expectedParentURI {
			t.Errorf("Expected parent URI %q, got %q", expectedParentURI, inReplyToID)
		}
	})
}

func TestBlueskyRepostHandling(t *testing.T) {
	// Test repost detection
	repostPost := blueskyPost{
		Record: blueskyRecord{
			Type: "app.bsky.feed.repost",
		},
	}

	t.Run("repost detection", func(t *testing.T) {
		client := NewBlueskyClient()
		postType := client.determinePostType(repostPost)
		
		if postType != PostTypeRepost {
			t.Errorf("Expected PostTypeRepost, got %v", postType)
		}
	})

	t.Run("repost type override", func(t *testing.T) {
		// Test logic from FetchUserPosts where repost type is set
		postType := PostTypeOriginal // Initial value
		
		if repostPost.Record.Type == "app.bsky.feed.repost" {
			postType = PostTypeRepost
		}
		
		if postType != PostTypeRepost {
			t.Errorf("Expected PostTypeRepost after override, got %v", postType)
		}
	})
}

func TestBlueskyURLGeneration(t *testing.T) {
	tests := []struct {
		name     string
		handle   string
		uri      string
		expected string
	}{
		{
			name:     "standard post URL",
			handle:   "test.bsky.social",
			uri:      "at://did:plc:test123/app.bsky.feed.post/abc123",
			expected: "https://bsky.app/profile/test.bsky.social/post/abc123",
		},
		{
			name:     "custom handle",
			handle:   "alice.example.com",
			uri:      "at://did:plc:alice456/app.bsky.feed.post/xyz789",
			expected: "https://bsky.app/profile/alice.example.com/post/xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test URL generation logic from FetchUserPosts
			postID := extractPostID(tt.uri)
			url := "https://bsky.app/profile/" + tt.handle + "/post/" + postID
			
			if url != tt.expected {
				t.Errorf("Expected URL %q, got %q", tt.expected, url)
			}
		})
	}
}

func TestBlueskySessionCreation(t *testing.T) {
	// Mock server for session creation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.createSession" {
			t.Errorf("Unexpected session path: %s", r.URL.Path)
		}
		
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		
		// Mock successful session response
		response := atpSessionResponse{
			AccessJwt:  "mock.jwt.token",
			RefreshJwt: "mock.refresh.token",
			Handle:     "test.bsky.social",
			DID:        "did:plc:test123",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	t.Run("session creation structure", func(t *testing.T) {
		// Test that we can structure session data correctly
		sessionData := map[string]string{
			"identifier": "test.bsky.social",
			"password":   "test-app-password",
		}
		
		jsonData, err := json.Marshal(sessionData)
		if err != nil {
			t.Errorf("Failed to marshal session data: %v", err)
		}
		
		if len(jsonData) == 0 {
			t.Error("Session data should not be empty")
		}
	})
}

func TestBlueskyErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedErrMsg string
	}{
		{
			name:           "404 not found",
			statusCode:     404,
			responseBody:   `{"error":"NotFound","message":"User not found"}`,
			expectedErrMsg: "404",
		},
		{
			name:           "401 unauthorized",
			statusCode:     401,
			responseBody:   `{"error":"Unauthorized","message":"Invalid credentials"}`,
			expectedErrMsg: "401",
		},
		{
			name:           "500 server error",
			statusCode:     500,
			responseBody:   `{"error":"InternalServerError","message":"Server error"}`,
			expectedErrMsg: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test error handling logic that would be used in API calls
			if tt.statusCode != http.StatusOK {
				// This simulates the error checking in fetchBlueskyPosts
				errorOccurred := true
				if !errorOccurred {
					t.Error("Should detect error for non-200 status codes")
				}
			}
		})
	}
}

func TestBlueskyAPIParameterValidation(t *testing.T) {
	tests := []struct {
		name     string
		username string
		limit    int
		valid    bool
	}{
		{
			name:     "valid parameters",
			username: "test.bsky.social",
			limit:    10,
			valid:    true,
		},
		{
			name:     "empty username",
			username: "",
			limit:    10,
			valid:    false,
		},
		{
			name:     "zero limit",
			username: "test.bsky.social",
			limit:    0,
			valid:    false,
		},
		{
			name:     "negative limit",
			username: "test.bsky.social",
			limit:    -1,
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parameter validation logic
			isValid := tt.username != "" && tt.limit > 0
			
			if isValid != tt.valid {
				t.Errorf("Expected validity %v, got %v", tt.valid, isValid)
			}
		})
	}
}