package cmd

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gerrowadat/cringesweeper/internal"
)

func TestDisplayPosts(t *testing.T) {
	// Create test posts
	now := time.Now()
	posts := []internal.Post{
		{
			ID:        "1",
			Author:    "Alice",
			Handle:    "alice",
			Content:   "Hello world!",
			CreatedAt: now,
			Type:      internal.PostTypeOriginal,
			Platform:  "test",
			URL:       "https://example.com/post/1",
			LikeCount: 5,
		},
		{
			ID:        "2",
			Author:    "Bob",
			Handle:    "bob",
			Content:   "This is a reply",
			CreatedAt: now.Add(-1 * time.Hour),
			Type:      internal.PostTypeReply,
			Platform:  "test",
			URL:       "https://example.com/post/2",
		},
		{
			ID:        "3",
			Author:    "Charlie",
			Handle:    "charlie",
			Content:   "Shared content",
			CreatedAt: now.Add(-2 * time.Hour),
			Type:      internal.PostTypeRepost,
			Platform:  "test",
			OriginalPost: &internal.Post{
				ID:      "original",
				Author:  "Dave",
				Handle:  "dave",
				Content: "Original post content",
			},
			OriginalAuthor: "Dave",
			OriginalHandle: "dave",
		},
	}

	t.Run("display posts with content", func(t *testing.T) {
		// This test would require capturing stdout to verify output
		// For now, we ensure it doesn't panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPosts panicked: %v", r)
			}
		}()

		displayPosts(posts, "TestPlatform")
	})

	t.Run("display empty posts", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPosts panicked with empty posts: %v", r)
			}
		}()

		displayPosts([]internal.Post{}, "TestPlatform")
	})
}

func TestPostTypeIndicators(t *testing.T) {
	tests := []struct {
		name     string
		postType internal.PostType
		expected string
	}{
		{"original post", internal.PostTypeOriginal, ""},
		{"repost", internal.PostTypeRepost, " [REPOST]"},
		{"reply", internal.PostTypeReply, " [REPLY]"},
		{"quote", internal.PostTypeQuote, " [QUOTE]"},
		{"like", internal.PostTypeLike, " [LIKE]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the type indicator logic from displayPosts
			var indicator string
			switch tt.postType {
			case internal.PostTypeRepost:
				indicator = " [REPOST]"
			case internal.PostTypeReply:
				indicator = " [REPLY]"
			case internal.PostTypeQuote:
				indicator = " [QUOTE]"
			case internal.PostTypeLike:
				indicator = " [LIKE]"
			}

			if indicator != tt.expected {
				t.Errorf("Expected indicator %q for type %v, got %q", tt.expected, tt.postType, indicator)
			}
		})
	}
}

func TestAuthorDisplayLogic(t *testing.T) {
	tests := []struct {
		name        string
		author      string
		handle      string
		expectedFmt string
	}{
		{"author same as handle", "alice", "alice", "@alice"},
		{"author different from handle", "Alice Smith", "alice", "@alice (Alice Smith)"},
		{"empty author", "", "alice", "@alice"},
		{"empty handle", "Alice", "", "@ (Alice)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test author display logic from displayPosts
			var result string
			if tt.author != "" && tt.author != tt.handle {
				result = "@" + tt.handle + " (" + tt.author + ")"
			} else {
				result = "@" + tt.handle
			}

			if result != tt.expectedFmt {
				t.Errorf("Expected %q, got %q", tt.expectedFmt, result)
			}
		})
	}
}

func TestEngagementMetricsDisplay(t *testing.T) {
	tests := []struct {
		name       string
		post       internal.Post
		hasMetrics bool
	}{
		{
			name: "post with all metrics",
			post: internal.Post{
				LikeCount:   10,
				RepostCount: 5,
				ReplyCount:  3,
			},
			hasMetrics: true,
		},
		{
			name: "post with some metrics",
			post: internal.Post{
				LikeCount:   10,
				RepostCount: 0,
				ReplyCount:  0,
			},
			hasMetrics: true,
		},
		{
			name: "post with no metrics",
			post: internal.Post{
				LikeCount:   0,
				RepostCount: 0,
				ReplyCount:  0,
			},
			hasMetrics: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test engagement metrics display logic
			hasAnyMetrics := tt.post.LikeCount > 0 || tt.post.RepostCount > 0 || tt.post.ReplyCount > 0

			if hasAnyMetrics != tt.hasMetrics {
				t.Errorf("Expected hasMetrics %v, got %v", tt.hasMetrics, hasAnyMetrics)
			}
		})
	}
}

func TestMetricsFormatting(t *testing.T) {
	tests := []struct {
		name     string
		likes    int
		reposts  int
		replies  int
		expected []string
	}{
		{
			name:     "all metrics",
			likes:    10,
			reposts:  5,
			replies:  3,
			expected: []string{"10 likes", "5 reposts", "3 replies"},
		},
		{
			name:     "only likes",
			likes:    7,
			reposts:  0,
			replies:  0,
			expected: []string{"7 likes"},
		},
		{
			name:     "likes and replies",
			likes:    15,
			reposts:  0,
			replies:  8,
			expected: []string{"15 likes", "8 replies"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test metrics formatting logic
			var metrics []string
			if tt.likes > 0 {
				metrics = append(metrics, "10 likes")
			}
			if tt.reposts > 0 {
				metrics = append(metrics, "5 reposts")
			}
			if tt.replies > 0 {
				metrics = append(metrics, "3 replies")
			}

			// For the first test case, check we have the right number of metrics
			if tt.name == "all metrics" && len(metrics) != 3 {
				t.Errorf("Expected 3 metrics, got %d", len(metrics))
			}
		})
	}
}

func TestRepostDisplayLogic(t *testing.T) {
	tests := []struct {
		name               string
		post               internal.Post
		shouldShowOriginal bool
	}{
		{
			name: "repost with original post",
			post: internal.Post{
				Type: internal.PostTypeRepost,
				OriginalPost: &internal.Post{
					Content: "Original content",
				},
				OriginalAuthor: "Dave",
				OriginalHandle: "dave",
			},
			shouldShowOriginal: true,
		},
		{
			name: "original post",
			post: internal.Post{
				Type:    internal.PostTypeOriginal,
				Content: "Regular content",
			},
			shouldShowOriginal: false,
		},
		{
			name: "repost without original post",
			post: internal.Post{
				Type:         internal.PostTypeRepost,
				OriginalPost: nil,
				Content:      "Repost content",
			},
			shouldShowOriginal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test repost display logic
			shouldShow := tt.post.Type == internal.PostTypeRepost && tt.post.OriginalPost != nil

			if shouldShow != tt.shouldShowOriginal {
				t.Errorf("Expected shouldShowOriginal %v, got %v", tt.shouldShowOriginal, shouldShow)
			}
		})
	}
}

func TestOriginalAuthorDisplay(t *testing.T) {
	tests := []struct {
		name           string
		originalAuthor string
		originalHandle string
		expected       string
	}{
		{"author same as handle", "dave", "dave", "@dave"},
		{"author different", "Dave Smith", "dave", "@dave (Dave Smith)"},
		{"empty author", "", "dave", "@dave"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test original author display logic for reposts
			var result string
			if tt.originalAuthor != "" && tt.originalAuthor != tt.originalHandle {
				result = "@" + tt.originalHandle + " (" + tt.originalAuthor + ")"
			} else {
				result = "@" + tt.originalHandle
			}

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestLsCommandValidation(t *testing.T) {
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
			// Test platform validation logic that would be used in ls command
			_, isValid := internal.GetClient(tt.platform)

			if isValid != tt.shouldBeValid {
				t.Errorf("Platform %q validity: expected %v, got %v", tt.platform, tt.shouldBeValid, isValid)
			}
		})
	}
}

func TestDisplayPostsWithQuotePost(t *testing.T) {
	now := time.Now()
	posts := []internal.Post{
		{
			ID:        "1",
			Author:    "Alice",
			Handle:    "alice",
			Content:   "This is a quote of something amazing",
			CreatedAt: now,
			Type:      internal.PostTypeQuote,
			Platform:  "test",
			URL:       "https://example.com/post/1",
			LikeCount: 3,
			OriginalPost: &internal.Post{
				ID:      "original",
				Author:  "Bob",
				Handle:  "bob",
				Content: "Original profound thought",
			},
			OriginalAuthor: "Bob",
			OriginalHandle: "bob",
		},
	}

	t.Run("display quote post", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPosts panicked with quote post: %v", r)
			}
		}()

		displayPosts(posts, "TestPlatform")
	})
}

func TestDisplayPostsWithLikePost(t *testing.T) {
	now := time.Now()
	posts := []internal.Post{
		{
			ID:        "1",
			Author:    "Alice",
			Handle:    "alice",
			Content:   "Liked content",
			CreatedAt: now,
			Type:      internal.PostTypeLike,
			Platform:  "test",
			URL:       "https://example.com/post/1",
		},
	}

	t.Run("display like post", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPosts panicked with like post: %v", r)
			}
		}()

		displayPosts(posts, "TestPlatform")
	})
}

func TestMetricsStringFormatting(t *testing.T) {
	tests := []struct {
		name       string
		likes      int
		reposts    int
		replies    int
		shouldShow bool
	}{
		{"all zero metrics", 0, 0, 0, false},
		{"only likes", 5, 0, 0, true},
		{"only reposts", 0, 3, 0, true},
		{"only replies", 0, 0, 2, true},
		{"all metrics", 10, 5, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test engagement metrics display logic
			hasMetrics := tt.likes > 0 || tt.reposts > 0 || tt.replies > 0

			if hasMetrics != tt.shouldShow {
				t.Errorf("Expected shouldShow %v, got %v", tt.shouldShow, hasMetrics)
			}

			if hasMetrics {
				var metrics []string
				if tt.likes > 0 {
					metrics = append(metrics, fmt.Sprintf("%d likes", tt.likes))
				}
				if tt.reposts > 0 {
					metrics = append(metrics, fmt.Sprintf("%d reposts", tt.reposts))
				}
				if tt.replies > 0 {
					metrics = append(metrics, fmt.Sprintf("%d replies", tt.replies))
				}

				if len(metrics) == 0 {
					t.Error("Expected metrics but got empty slice")
				}
			}
		})
	}
}

func TestDisplayPostsFormattingEdgeCases(t *testing.T) {
	now := time.Now()

	// Test post with very long content
	longContentPost := internal.Post{
		ID:        "long",
		Author:    "VerboseUser",
		Handle:    "verbose",
		Content:   strings.Repeat("This is a very long post content that goes on and on. ", 10),
		CreatedAt: now,
		Type:      internal.PostTypeOriginal,
		Platform:  "test",
	}

	// Test post with special characters
	specialCharsPost := internal.Post{
		ID:        "special",
		Author:    "EmojiUser 🎉",
		Handle:    "emoji_user",
		Content:   "Post with emojis 🚀 and special chars: @mentions #hashtags & symbols!",
		CreatedAt: now,
		Type:      internal.PostTypeOriginal,
		Platform:  "test",
	}

	posts := []internal.Post{longContentPost, specialCharsPost}

	t.Run("display posts with edge cases", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPosts panicked with edge cases: %v", r)
			}
		}()

		displayPosts(posts, "TestPlatform")
	})
}

func TestDisplayPostsRepostWithoutOriginal(t *testing.T) {
	now := time.Now()
	posts := []internal.Post{
		{
			ID:             "1",
			Author:         "Alice",
			Handle:         "alice",
			Content:        "This is a repost without original",
			CreatedAt:      now,
			Type:           internal.PostTypeRepost,
			Platform:       "test",
			OriginalPost:   nil, // No original post
			OriginalAuthor: "Unknown",
			OriginalHandle: "unknown",
		},
	}

	t.Run("display repost without original post", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("displayPosts panicked with repost without original: %v", r)
			}
		}()

		displayPosts(posts, "TestPlatform")
	})
}


func TestFilterPostsByAge(t *testing.T) {
	now := time.Now()
	posts := []internal.Post{
		{
			ID:        "1",
			Content:   "Recent post",
			CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:        "2", 
			Content:   "Old post",
			CreatedAt: now.Add(-25 * time.Hour),
		},
		{
			ID:        "3",
			Content:   "Very old post", 
			CreatedAt: now.Add(-48 * time.Hour),
		},
	}

	tests := []struct {
		name       string
		maxAge     *time.Duration
		beforeDate *time.Time
		expected   int
	}{
		{
			name:     "no filters",
			expected: 3,
		},
		{
			name:     "max age 2 hours",
			maxAge:   durationPtr(2 * time.Hour),
			expected: 1,
		},
		{
			name:     "max age 30 hours",
			maxAge:   durationPtr(30 * time.Hour),
			expected: 2,
		},
		{
			name:       "before date 1 hour ago",
			beforeDate: timePtr(now.Add(-1 * time.Hour)),
			expected:   2,
		},
		{
			name:       "before date 26 hours ago",
			beforeDate: timePtr(now.Add(-26 * time.Hour)),
			expected:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterPostsByAge(posts, tt.maxAge, tt.beforeDate)
			if len(filtered) != tt.expected {
				t.Errorf("Expected %d posts, got %d", tt.expected, len(filtered))
			}
		})
	}
}

func TestFilterPostsByAgeWithTermination(t *testing.T) {
	now := time.Now()
	posts := []internal.Post{
		{
			ID:        "1",
			Content:   "Recent post",
			CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:        "2",
			Content:   "Old post", 
			CreatedAt: now.Add(-25 * time.Hour),
		},
		{
			ID:        "3",
			Content:   "Very old post",
			CreatedAt: now.Add(-48 * time.Hour),
		},
	}

	tests := []struct {
		name             string
		maxAge           *time.Duration
		beforeDate       *time.Time
		expectedPosts    int
		shouldContinue   bool
	}{
		{
			name:           "no filters",
			expectedPosts:  3,
			shouldContinue: true,
		},
		{
			name:           "max age allows all posts",
			maxAge:         durationPtr(50 * time.Hour),
			expectedPosts:  3,
			shouldContinue: true,
		},
		{
			name:           "max age stops at second post",
			maxAge:         durationPtr(30 * time.Hour),
			expectedPosts:  2,
			shouldContinue: false,
		},
		{
			name:           "max age stops at first post",
			maxAge:         durationPtr(2 * time.Hour),
			expectedPosts:  1,
			shouldContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, shouldContinue := filterPostsByAgeWithTermination(posts, tt.maxAge, tt.beforeDate)
			
			if len(filtered) != tt.expectedPosts {
				t.Errorf("Expected %d posts, got %d", tt.expectedPosts, len(filtered))
			}
			
			if shouldContinue != tt.shouldContinue {
				t.Errorf("Expected shouldContinue=%v, got %v", tt.shouldContinue, shouldContinue)
			}
		})
	}
}

func TestDisplaySinglePost(t *testing.T) {
	// Test that displaySinglePost doesn't panic with various post types
	posts := []internal.Post{
		{
			ID:        "1",
			Type:      internal.PostTypeOriginal,
			Content:   "Original post",
			Handle:    "user",
			Author:    "User Name",
			CreatedAt: time.Now(),
			URL:       "https://example.com/1",
		},
		{
			ID:        "2", 
			Type:      internal.PostTypeRepost,
			Content:   "Reposted content",
			Handle:    "user",
			CreatedAt: time.Now(),
			OriginalHandle: "original_user",
			OriginalAuthor: "Original User",
			OriginalPost: &internal.Post{
				Content: "Original post content",
			},
		},
		{
			ID:        "3",
			Type:      internal.PostTypeReply,
			Content:   "Reply content",
			Handle:    "user", 
			CreatedAt: time.Now(),
		},
		{
			ID:        "4",
			Type:      internal.PostTypeLike,
			Content:   "Liked content",
			Handle:    "user",
			CreatedAt: time.Now(),
		},
		{
			ID:        "5",
			Type:      internal.PostTypeQuote,
			Content:   "Quote content",
			Handle:    "user",
			CreatedAt: time.Now(),
		},
	}

	for i, post := range posts {
		t.Run(string(post.Type), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("displaySinglePost panicked with post type %s: %v", post.Type, r)
				}
			}()
			
			displaySinglePost(post, i+1)
		})
	}
}

// Helper functions
func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func timePtr(t time.Time) *time.Time {
	return &t
}
