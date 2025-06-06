package internal

import "time"

// PostType represents the type of social media post
type PostType string

const (
	PostTypeOriginal PostType = "original" // User's own original content
	PostTypeRepost   PostType = "repost"   // Retweet/Reblog/Repost of another user's content
	PostTypeReply    PostType = "reply"    // Reply to another post
	PostTypeLike     PostType = "like"     // Like/Favorite (if platform shows these in timeline)
	PostTypeQuote    PostType = "quote"    // Quote post/retweet with comment
)

// Post represents a generic social media post
type Post struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`        // Display name of the post author
	Handle    string    `json:"handle"`        // Username/handle of the post author
	Content   string    `json:"content"`       // Text content of the post
	CreatedAt time.Time `json:"created_at"`    // When the post was created
	URL       string    `json:"url,omitempty"` // Direct URL to the post

	// Post type and interaction metadata
	Type PostType `json:"type"` // Type of post (original, repost, reply, etc.)

	// Original post information (for reposts/quotes)
	OriginalPost   *Post  `json:"original_post,omitempty"`   // The original post being shared
	OriginalAuthor string `json:"original_author,omitempty"` // Display name of original author
	OriginalHandle string `json:"original_handle,omitempty"` // Handle of original author

	// Reply metadata
	InReplyToID     string `json:"in_reply_to_id,omitempty"`     // ID of post being replied to
	InReplyToAuthor string `json:"in_reply_to_author,omitempty"` // Author of post being replied to

	// Engagement metrics
	RepostCount int `json:"repost_count,omitempty"` // Number of reposts/retweets
	LikeCount   int `json:"like_count,omitempty"`   // Number of likes/favorites
	ReplyCount  int `json:"reply_count,omitempty"`  // Number of replies

	// Post status flags
	IsLikedByUser bool `json:"is_liked_by_user,omitempty"` // Whether the viewing user has liked this post
	IsPinned      bool `json:"is_pinned,omitempty"`        // Whether this post is pinned by the author

	// Platform-specific metadata
	Platform string                 `json:"platform"`           // Which platform this post is from
	RawData  map[string]interface{} `json:"raw_data,omitempty"` // Platform-specific raw data
}

// PruneOptions defines criteria for pruning posts
type PruneOptions struct {
	MaxAge           *time.Duration `json:"max_age,omitempty"`     // Delete posts older than this duration
	BeforeDate       *time.Time     `json:"before_date,omitempty"` // Delete posts created before this date
	PreserveSelfLike bool           `json:"preserve_self_like"`    // Don't delete user's own posts they've liked
	PreservePinned   bool           `json:"preserve_pinned"`       // Don't delete pinned posts
	UnlikePosts      bool           `json:"unlike_posts"`          // Unlike posts instead of deleting them
	UnshareReposts   bool           `json:"unshare_reposts"`       // Unshare/unrepost instead of deleting reposts
	DryRun           bool           `json:"dry_run"`               // Only show what would be deleted
	RateLimitDelay   time.Duration  `json:"rate_limit_delay"`      // Delay between API requests to respect rate limits
}

// PruneResult represents the result of a pruning operation
type PruneResult struct {
	PostsToDelete  []Post   `json:"posts_to_delete"`
	PostsToUnlike  []Post   `json:"posts_to_unlike"`
	PostsToUnshare []Post   `json:"posts_to_unshare"`
	PostsPreserved []Post   `json:"posts_preserved"`
	DeletedCount   int      `json:"deleted_count"`
	UnlikedCount   int      `json:"unliked_count"`
	UnsharedCount  int      `json:"unshared_count"`
	PreservedCount int      `json:"preserved_count"`
	ErrorsCount    int      `json:"errors_count"`
	Errors         []string `json:"errors,omitempty"`
}

// SocialClient defines the interface for social media platforms
type SocialClient interface {
	// FetchUserPosts retrieves recent posts for a given username
	FetchUserPosts(username string, limit int) ([]Post, error)

	// GetPlatformName returns the name of the social platform
	GetPlatformName() string

	// PrunePosts deletes posts according to specified criteria
	PrunePosts(username string, options PruneOptions) (*PruneResult, error)

	// RequiresAuth returns true if the platform requires authentication for deletion
	RequiresAuth() bool
}

// SupportedPlatforms maps platform names to their client constructors
var SupportedPlatforms = map[string]func() SocialClient{
	"bluesky":  func() SocialClient { return NewBlueskyClient() },
	"mastodon": func() SocialClient { return NewMastodonClient() },
}

// GetClient returns a social client for the specified platform
func GetClient(platform string) (SocialClient, bool) {
	constructor, exists := SupportedPlatforms[platform]
	if !exists {
		return nil, false
	}
	return constructor(), true
}
