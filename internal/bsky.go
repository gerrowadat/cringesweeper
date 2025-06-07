package internal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BlueskyClient implements the SocialClient interface for Bluesky
type BlueskyClient struct {
	sessionManager *SessionManager
	session        *atpSessionResponse
}

// NewBlueskyClient creates a new Bluesky client
func NewBlueskyClient() *BlueskyClient {
	return &BlueskyClient{
		sessionManager: NewSessionManager("bluesky"),
	}
}

// GetPlatformName returns the platform name
func (c *BlueskyClient) GetPlatformName() string {
	return "Bluesky"
}

// RequiresAuth returns true if the platform requires authentication for deletion
func (c *BlueskyClient) RequiresAuth() bool {
	return true // Bluesky requires authentication for post deletion
}

// FetchUserPosts retrieves recent posts for a Bluesky user
func (c *BlueskyClient) FetchUserPosts(username string, limit int) ([]Post, error) {
	posts, err := c.fetchBlueskyPosts(username, limit)
	if err != nil {
		return nil, err
	}

	// Convert Bluesky posts to generic Post format
	var genericPosts []Post
	for _, bskyPost := range posts {
		post := Post{
			ID:        bskyPost.URI,
			Author:    bskyPost.Author.DisplayName,
			Handle:    bskyPost.Author.Handle,
			Content:   bskyPost.Record.Text,
			CreatedAt: bskyPost.Record.CreatedAt,
			URL:       fmt.Sprintf("https://bsky.app/profile/%s/post/%s", bskyPost.Author.Handle, extractPostID(bskyPost.URI)),
			Type:      c.determinePostType(bskyPost),
			Platform:  "bluesky",

			// Engagement metrics
			RepostCount: bskyPost.RepostCount,
			LikeCount:   bskyPost.LikeCount,
			ReplyCount:  bskyPost.ReplyCount,
		}

		// Use Author.Handle as fallback if DisplayName is empty
		if post.Author == "" {
			post.Author = bskyPost.Author.Handle
		}

		// Set viewer interaction status
		if bskyPost.ViewerData != nil {
			post.IsLikedByUser = bskyPost.ViewerData.Like != nil
			// Note: IsPinned would need to be determined from the feed metadata
		}

		// Handle reposts - these are the user's own repost records, not the original posts
		if bskyPost.Record.Type == "app.bsky.feed.repost" {
			post.Type = PostTypeRepost
			// For reposts, the ID should be the repost record URI, not the original post URI
			post.ID = bskyPost.URI // This is the user's repost record URI
		}

		// Handle replies
		if bskyPost.Record.Reply != nil {
			post.Type = PostTypeReply
			post.InReplyToID = bskyPost.Record.Reply.Parent.URI
		}

		// Note: Likes are not returned by getAuthorFeed - they need to be fetched separately
		// if we want to include them in the pruning process

		genericPosts = append(genericPosts, post)
	}

	return genericPosts, nil
}

// FetchUserPostsPaginated retrieves posts with cursor-based pagination
func (c *BlueskyClient) FetchUserPostsPaginated(username string, limit int, cursor string) ([]Post, string, error) {
	posts, nextCursor, err := c.fetchBlueskyPostsPaginated(username, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	// Convert Bluesky posts to generic Post format
	var genericPosts []Post
	for _, bskyPost := range posts {
		post := Post{
			ID:        bskyPost.URI,
			Author:    bskyPost.Author.DisplayName,
			Handle:    bskyPost.Author.Handle,
			Content:   bskyPost.Record.Text,
			CreatedAt: bskyPost.Record.CreatedAt,
			URL:       fmt.Sprintf("https://bsky.app/profile/%s/post/%s", bskyPost.Author.Handle, extractPostID(bskyPost.URI)),
			Type:      c.determinePostType(bskyPost),
			Platform:  "bluesky",

			// Engagement metrics
			RepostCount: bskyPost.RepostCount,
			LikeCount:   bskyPost.LikeCount,
			ReplyCount:  bskyPost.ReplyCount,
		}

		// Use Author.Handle as fallback if DisplayName is empty
		if post.Author == "" {
			post.Author = bskyPost.Author.Handle
		}

		// Set viewer interaction status
		if bskyPost.ViewerData != nil {
			post.IsLikedByUser = bskyPost.ViewerData.Like != nil
			// Note: IsPinned would need to be determined from the feed metadata
		}

		// Handle reposts - these are the user's own repost records, not the original posts
		if bskyPost.Record.Type == "app.bsky.feed.repost" {
			post.Type = PostTypeRepost
			// For reposts, the ID should be the repost record URI, not the original post URI
			post.ID = bskyPost.URI // This is the user's repost record URI
		}

		// Handle replies
		if bskyPost.Record.Reply != nil {
			post.Type = PostTypeReply
			post.InReplyToID = bskyPost.Record.Reply.Parent.URI
		}

		// Note: Likes are not returned by getAuthorFeed - they need to be fetched separately
		// if we want to include them in the pruning process

		genericPosts = append(genericPosts, post)
	}

	return genericPosts, nextCursor, nil
}

func (c *BlueskyClient) fetchBlueskyPostsPaginated(username string, limit int, cursor string) ([]blueskyPost, string, error) {
	baseURL := "https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed"
	params := url.Values{}
	params.Add("actor", username)
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("include_pins", "true")         // Include pinned posts
	params.Add("filter", "posts_with_replies") // Get user's own posts and replies
	
	if cursor != "" {
		params.Add("cursor", cursor)
	}

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	LogHTTPRequest("GET", fullURL)
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch posts: %w", err)
	}
	defer resp.Body.Close()

	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	var feedResponse blueskyEnhancedFeedResponse
	if err := json.Unmarshal(body, &feedResponse); err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}

	var posts []blueskyPost
	for _, item := range feedResponse.Feed {
		// Enhance post with viewer information
		item.Post.ViewerData = item.ViewerData
		posts = append(posts, item.Post)
	}

	nextCursor := ""
	if feedResponse.Cursor != nil {
		nextCursor = *feedResponse.Cursor
	}

	return posts, nextCursor, nil
}

// Bluesky-specific types
type blueskyPost struct {
	URI        string             `json:"uri"`
	CID        string             `json:"cid"`
	Author     blueskyAuthor      `json:"author"`
	Record     blueskyRecord      `json:"record"`
	IndexedAt  time.Time          `json:"indexedAt"`
	ViewerData *blueskyViewerData `json:"-"` // Added separately from API response

	// Engagement metrics
	RepostCount int `json:"repostCount,omitempty"`
	LikeCount   int `json:"likeCount,omitempty"`
	ReplyCount  int `json:"replyCount,omitempty"`
}

type blueskyAuthor struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName,omitempty"`
}

type blueskyRecord struct {
	Type      string        `json:"$type"`
	Text      string        `json:"text"`
	CreatedAt time.Time     `json:"createdAt"`
	Reply     *blueskyReply `json:"reply,omitempty"`
}

type blueskyReply struct {
	Parent blueskyPostRef `json:"parent"`
	Root   blueskyPostRef `json:"root"`
}

type blueskyPostRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type blueskyViewerData struct {
	Like   *string `json:"like,omitempty"`   // URI of like record if liked by viewer
	Repost *string `json:"repost,omitempty"` // URI of repost record if reposted by viewer
}

type blueskyEnhancedFeedResponse struct {
	Feed []struct {
		Post       blueskyPost        `json:"post"`
		ViewerData *blueskyViewerData `json:"viewer,omitempty"`
		// Additional fields for pinned posts and other metadata
		PinnedPost bool `json:"pinnedPost,omitempty"`
	} `json:"feed"`
	Cursor *string `json:"cursor,omitempty"`
}

func (c *BlueskyClient) fetchBlueskyPosts(username string, limit int) ([]blueskyPost, error) {
	baseURL := "https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed"
	params := url.Values{}
	params.Add("actor", username)
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("include_pins", "true")         // Include pinned posts
	params.Add("filter", "posts_with_replies") // Get user's own posts and replies

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	LogHTTPRequest("GET", fullURL)
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts: %w", err)
	}
	defer resp.Body.Close()

	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var feedResponse blueskyEnhancedFeedResponse
	if err := json.Unmarshal(body, &feedResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var posts []blueskyPost
	for _, item := range feedResponse.Feed {
		// Enhance post with viewer information
		item.Post.ViewerData = item.ViewerData
		posts = append(posts, item.Post)
	}

	return posts, nil
}

// determinePostType determines the type of Bluesky post
func (c *BlueskyClient) determinePostType(post blueskyPost) PostType {
	switch post.Record.Type {
	case "app.bsky.feed.repost":
		return PostTypeRepost
	case "app.bsky.feed.post":
		if post.Record.Reply != nil {
			return PostTypeReply
		}
		return PostTypeOriginal
	default:
		return PostTypeOriginal
	}
}

// PrunePosts deletes posts according to specified criteria
func (c *BlueskyClient) PrunePosts(username string, options PruneOptions) (*PruneResult, error) {
	// Get authentication credentials
	creds, err := GetCredentialsForPlatform("bluesky")
	if err != nil {
		return nil, fmt.Errorf("authentication required: %w", err)
	}

	if err := ValidateCredentials(creds); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	// Create session to get authenticated user's DID
	session, err := c.ensureValidSession(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Bluesky: %w. This may indicate invalid credentials or DID resolution issues", err)
	}

	// Fetch user's posts (we'd need to fetch more than 10 for real pruning)
	posts, err := c.FetchUserPosts(username, 100) // Fetch more posts for pruning
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts: %w", err)
	}

	// If user wants to unlike posts, also fetch their liked posts
	if options.UnlikePosts {
		likedPosts, err := c.fetchLikedPosts(session, 100)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to fetch liked posts: %v\n", err)
		} else {
			posts = append(posts, likedPosts...)
		}
	}

	// Always fetch the user's repost records separately to ensure we get the correct repost URIs
	repostPosts, err := c.fetchRepostPosts(session, 100)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to fetch repost records: %v\n", err)
	} else {
		posts = append(posts, repostPosts...)
	}

	result := &PruneResult{
		PostsToDelete:  []Post{},
		PostsToUnlike:  []Post{},
		PostsToUnshare: []Post{},
		PostsPreserved: []Post{},
		Errors:         []string{},
	}

	now := time.Now()

	for _, post := range posts {
		shouldProcess := false
		preserveReason := ""

		// Check age criteria
		if options.MaxAge != nil {
			if now.Sub(post.CreatedAt) > *options.MaxAge {
				shouldProcess = true
			}
		}

		// Check date criteria
		if options.BeforeDate != nil {
			if post.CreatedAt.Before(*options.BeforeDate) {
				shouldProcess = true
			}
		}

		if !shouldProcess {
			continue
		}

		// Check preservation rules
		if options.PreservePinned && post.IsPinned {
			preserveReason = "pinned"
		} else if options.PreserveSelfLike && post.IsLikedByUser && post.Type == PostTypeOriginal {
			preserveReason = "self-liked"
		}

		if preserveReason != "" {
			result.PostsPreserved = append(result.PostsPreserved, post)
			result.PreservedCount++
		} else {
			// Determine action based on post type
			if post.Type == PostTypeLike {
				// Handle like records - delete the like record directly
				result.PostsToUnlike = append(result.PostsToUnlike, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					logger := WithPlatform("bluesky").With().Str("post_id", post.ID).Logger()
					if err := c.deleteLikeRecord(creds, post.ID); err != nil {
						logger.Error().Err(err).Msg("Failed to unlike post")
						fmt.Printf("❌ Failed to unlike post from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to unlike post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						logger.Info().Str("content", TruncateContent(post.Content, 50)).Msg("Post unliked successfully")
						fmt.Printf("👍 Unliked post from %s: %s\n", post.CreatedAt.Format("2006-01-02"), TruncateContent(post.Content, 50))
						result.UnlikedCount++
					}
				}
			} else if post.Type == PostTypeRepost {
				// Always unrepost for repost records - these are the user's own repost actions
				result.PostsToUnshare = append(result.PostsToUnshare, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					// For reposts, we need to delete the repost record directly
					logger := WithPlatform("bluesky").With().Str("post_id", post.ID).Logger()
					if err := c.deleteRepostRecord(creds, post.ID); err != nil {
						logger.Error().Err(err).Msg("Failed to unrepost")
						fmt.Printf("❌ Failed to unrepost from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to unrepost post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						logger.Info().Str("content", TruncateContent(post.Content, 50)).Msg("Repost unshared successfully")
						fmt.Printf("🔄 Unshared repost from %s: %s\n", post.CreatedAt.Format("2006-01-02"), TruncateContent(post.Content, 50))
						result.UnsharedCount++
					}
				}
			} else if post.Type == PostTypeOriginal || post.Type == PostTypeReply {
				// Validate that the post belongs to the authenticated user
				if err := c.validatePostURI(post.ID, session.DID); err != nil {
					fmt.Printf("⚠️  Skipping post from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
					result.Errors = append(result.Errors, fmt.Sprintf("Post validation failed %s: %v", post.ID, err))
					result.ErrorsCount++
					continue
				}

				result.PostsToDelete = append(result.PostsToDelete, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					logger := WithPlatform("bluesky").With().Str("post_id", post.ID).Logger()
					if err := c.deletePost(creds, post.ID); err != nil {
						logger.Error().Err(err).Msg("Failed to delete post")
						fmt.Printf("❌ Failed to delete post from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to delete post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						logger.Info().Str("content", TruncateContent(post.Content, 50)).Msg("Post deleted successfully")
						fmt.Printf("🗑️  Deleted post from %s: %s\n", post.CreatedAt.Format("2006-01-02"), TruncateContent(post.Content, 50))
						result.DeletedCount++
					}
				}
			}
		}
	}

	return result, nil
}

// extractPostID extracts the post ID from a Bluesky URI
func extractPostID(uri string) string {
	// URI format: at://did:plc:xxx/app.bsky.feed.post/postid
	// We want just the postid part
	if len(uri) > 0 {
		parts := []rune(uri)
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == '/' {
				return string(parts[i+1:])
			}
		}
	}
	return ""
}

// AT Protocol session management and API types
type atpSessionResponse struct {
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	Handle     string `json:"handle"`
	DID        string `json:"did"`
}

// invalidateSession clears the current session (useful when credentials change)
func (c *BlueskyClient) invalidateSession() {
	c.session = nil
	c.sessionManager.ClearSession()
}

// ensureValidSession ensures we have a valid session, creating/refreshing as needed
func (c *BlueskyClient) ensureValidSession(creds *Credentials) (*atpSessionResponse, error) {
	logger := WithPlatform("bluesky")
	
	// If we don't have a session or credentials changed, create new session
	if c.session == nil || c.sessionManager.HasCredentialsChanged(creds) {
		if c.sessionManager.HasCredentialsChanged(creds) {
			logger.Debug().Msg("Credentials changed, creating new Bluesky session")
			fmt.Printf("🔄 Credentials changed, creating new Bluesky session...\n")
		} else {
			logger.Debug().Msg("Creating new Bluesky session")
			fmt.Printf("🔐 Creating new Bluesky session...\n")
		}
		return c.createNewSession(creds)
	}

	// If session is expired or about to expire, try to refresh
	if !c.sessionManager.IsSessionValid() {
		logger.Debug().Msg("Session expired, refreshing using refresh token")
		fmt.Printf("🔄 Refreshing Bluesky session using refresh token...\n")
		refreshedSession, err := c.refreshSession()
		if err != nil {
			// If refresh fails, fall back to creating a new session
			logger.Debug().Err(err).Msg("Session refresh failed, creating new session")
			fmt.Printf("⚠️  Refresh failed, creating new session: %v\n", err)
			return c.createNewSession(creds)
		}
		return refreshedSession, nil
	}

	// Reusing existing session
	logger.Debug().Msg("Reusing existing valid session")
	return c.session, nil
}

// refreshSession uses the refresh token to extend the current session
func (c *BlueskyClient) refreshSession() (*atpSessionResponse, error) {
	if c.session == nil || c.session.RefreshJwt == "" {
		return nil, fmt.Errorf("no valid refresh token available")
	}

	refreshURL := "https://bsky.social/xrpc/com.atproto.server.refreshSession"

	req, err := http.NewRequest("POST", refreshURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	// Use the refresh token as authorization
	req.Header.Set("Authorization", "Bearer "+c.session.RefreshJwt)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("POST", refreshURL)
	resp, err := client.Do(req)
	LogHTTPResponse("POST", refreshURL, resp.StatusCode, resp.Status)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("session refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	var refreshedSession atpSessionResponse
	if err := json.Unmarshal(body, &refreshedSession); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Update stored session with new tokens
	c.session = &refreshedSession

	// Try to parse actual expiration from refreshed JWT, fall back to 24 hours
	logger := WithPlatform("bluesky")
	if expTime, err := c.parseJWTExpiration(refreshedSession.AccessJwt); err == nil {
		c.sessionManager.UpdateSession(refreshedSession.AccessJwt, refreshedSession.RefreshJwt, expTime, &Credentials{})
		logger.Debug().Time("expires_at", expTime).Msg("Session refreshed with parsed expiration")
		fmt.Printf("✅ Session refreshed, expires at %s\n", expTime.Format("15:04:05"))
	} else {
		// Fallback to default 24 hours
		expTime := time.Now().Add(24 * time.Hour)
		c.sessionManager.UpdateSession(refreshedSession.AccessJwt, refreshedSession.RefreshJwt, expTime, &Credentials{})
		logger.Debug().Time("expires_at", expTime).Msg("Session refreshed with default 24h expiration")
		fmt.Printf("✅ Session refreshed with default 24h expiration\n")
	}

	return &refreshedSession, nil
}

// parseJWTExpiration extracts expiration time from JWT token
func (c *BlueskyClient) parseJWTExpiration(token string) (time.Time, error) {
	// JWT format: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part)
	payload := parts[1]
	// Add padding if necessary for base64 decoding
	if len(payload)%4 != 0 {
		payload += strings.Repeat("=", 4-len(payload)%4)
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims struct {
		Exp int64 `json:"exp"` // Unix timestamp
	}

	if err := json.Unmarshal(decoded, &claims); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	if claims.Exp == 0 {
		// No expiration in token, fall back to default
		return time.Now().Add(24 * time.Hour), nil
	}

	return time.Unix(claims.Exp, 0), nil
}

// createNewSession creates a fresh session and stores it
func (c *BlueskyClient) createNewSession(creds *Credentials) (*atpSessionResponse, error) {
	session, err := c.createSession(creds)
	if err != nil {
		return nil, err
	}

	// Store session and credentials for reuse
	c.session = session

	// Try to parse actual expiration from JWT, fall back to 24 hours
	logger := WithPlatform("bluesky")
	if expTime, err := c.parseJWTExpiration(session.AccessJwt); err == nil {
		c.sessionManager.UpdateSession(session.AccessJwt, session.RefreshJwt, expTime, creds)
		logger.Debug().Time("expires_at", expTime).Msg("Session created with parsed expiration")
		fmt.Printf("✅ Session created, expires at %s\n", expTime.Format("15:04:05"))
	} else {
		// Fallback to default 24 hours
		expTime := time.Now().Add(24 * time.Hour)
		c.sessionManager.UpdateSession(session.AccessJwt, session.RefreshJwt, expTime, creds)
		logger.Debug().Time("expires_at", expTime).Msg("Session created with default 24h expiration")
		fmt.Printf("✅ Session created with default 24h expiration\n")
	}

	return session, nil
}

// createSession authenticates with AT Protocol and returns access token
func (c *BlueskyClient) createSession(creds *Credentials) (*atpSessionResponse, error) {
	sessionURL := "https://bsky.social/xrpc/com.atproto.server.createSession"

	sessionData := map[string]string{
		"identifier": creds.Username,
		"password":   creds.AppPassword,
	}

	jsonData, err := json.Marshal(sessionData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session data: %w", err)
	}

	req, err := http.NewRequest("POST", sessionURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create session request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("POST", sessionURL)
	resp, err := client.Do(req)
	LogHTTPResponse("POST", sessionURL, resp.StatusCode, resp.Status)
	if err != nil {
		return nil, fmt.Errorf("session request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("session creation failed with status %d: %s. This may indicate invalid credentials or DID resolution issues", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read session response: %w", err)
	}

	var session atpSessionResponse
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session response: %w", err)
	}

	return &session, nil
}

// deletePost deletes a Bluesky post using AT Protocol
func (c *BlueskyClient) deletePost(creds *Credentials, postURI string) error {
	session, err := c.ensureValidSession(creds)
	if err != nil {
		return fmt.Errorf("failed to ensure valid session: %w", err)
	}

	// Extract collection and rkey from URI
	// URI format: at://did:plc:xxx/app.bsky.feed.post/rkey
	parts := strings.Split(postURI, "/")
	if len(parts) < 5 {
		return fmt.Errorf("invalid post URI format: %s", postURI)
	}

	did := parts[2]
	collection := strings.Join(parts[3:len(parts)-1], "/")
	rkey := parts[len(parts)-1]

	// Verify that the DID from the post URI matches the authenticated user's DID
	if did != session.DID {
		return fmt.Errorf("DID mismatch: post DID %s does not match authenticated user DID %s. This suggests the post belongs to a different user or there's a DID resolution issue", did, session.DID)
	}

	deleteURL := "https://bsky.social/xrpc/com.atproto.repo.deleteRecord"

	deleteData := map[string]string{
		"repo":       session.DID, // Use authenticated user's DID instead of post DID
		"collection": collection,
		"rkey":       rkey,
	}

	jsonData, err := json.Marshal(deleteData)
	if err != nil {
		return fmt.Errorf("failed to marshal delete data: %w", err)
	}

	req, err := http.NewRequest("POST", deleteURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("POST", deleteURL)
	resp, err := client.Do(req)
	LogHTTPResponse("POST", deleteURL, resp.StatusCode, resp.Status)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete request failed with status %d: %s. DID used: %s, rkey: %s", resp.StatusCode, string(body), session.DID, rkey)
	}

	return nil
}

// unlikePost removes a like from a Bluesky post
func (c *BlueskyClient) unlikePost(creds *Credentials, postURI string) error {
	session, err := c.ensureValidSession(creds)
	if err != nil {
		return fmt.Errorf("failed to ensure valid session: %w", err)
	}

	// First, we need to find the like record for this post
	// This would require listing the user's likes and finding the one for this URI
	// For now, we'll use a simplified approach that attempts to delete based on the post URI

	// In AT Protocol, likes are stored as app.bsky.feed.like records
	// We need to find the specific like record's rkey for this post
	likeRkey, err := c.findLikeRecord(session, postURI)
	if err != nil {
		return fmt.Errorf("failed to find like record: %w", err)
	}

	if likeRkey == "" {
		return fmt.Errorf("no like record found for post %s", postURI)
	}

	deleteURL := "https://bsky.social/xrpc/com.atproto.repo.deleteRecord"

	deleteData := map[string]string{
		"repo":       session.DID,
		"collection": "app.bsky.feed.like",
		"rkey":       likeRkey,
	}

	jsonData, err := json.Marshal(deleteData)
	if err != nil {
		return fmt.Errorf("failed to marshal unlike data: %w", err)
	}

	req, err := http.NewRequest("POST", deleteURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create unlike request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("POST", deleteURL)
	resp, err := client.Do(req)
	LogHTTPResponse("POST", deleteURL, resp.StatusCode, resp.Status)
	if err != nil {
		return fmt.Errorf("unlike request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unlike request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// unrepost removes a repost from Bluesky
func (c *BlueskyClient) unrepost(creds *Credentials, postURI string) error {
	session, err := c.ensureValidSession(creds)
	if err != nil {
		return fmt.Errorf("failed to ensure valid session: %w", err)
	}

	// Find the repost record for this post
	repostRkey, err := c.findRepostRecord(session, postURI)
	if err != nil {
		return fmt.Errorf("failed to find repost record: %w", err)
	}

	if repostRkey == "" {
		return fmt.Errorf("no repost record found for post %s", postURI)
	}

	deleteURL := "https://bsky.social/xrpc/com.atproto.repo.deleteRecord"

	deleteData := map[string]string{
		"repo":       session.DID,
		"collection": "app.bsky.feed.repost",
		"rkey":       repostRkey,
	}

	jsonData, err := json.Marshal(deleteData)
	if err != nil {
		return fmt.Errorf("failed to marshal unrepost data: %w", err)
	}

	req, err := http.NewRequest("POST", deleteURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create unrepost request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("POST", deleteURL)
	resp, err := client.Do(req)
	LogHTTPResponse("POST", deleteURL, resp.StatusCode, resp.Status)
	if err != nil {
		return fmt.Errorf("unrepost request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unrepost request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// findLikeRecord finds the rkey for a like record of a specific post
func (c *BlueskyClient) findLikeRecord(session *atpSessionResponse, postURI string) (string, error) {
	listURL := "https://bsky.social/xrpc/com.atproto.repo.listRecords"

	params := url.Values{}
	params.Add("repo", session.DID)
	params.Add("collection", "app.bsky.feed.like")
	params.Add("limit", "100") // Start with reasonable limit

	var cursor string

	for {
		currentParams := url.Values{}
		for k, v := range params {
			currentParams[k] = v
		}
		if cursor != "" {
			currentParams.Add("cursor", cursor)
		}

		fullURL := fmt.Sprintf("%s?%s", listURL, currentParams.Encode())

		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create list request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

		client := &http.Client{Timeout: 30 * time.Second}
		LogHTTPRequest("GET", fullURL)
		resp, err := client.Do(req)
		LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)
		if err != nil {
			return "", fmt.Errorf("list request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("list request failed with status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read list response: %w", err)
		}

		var listResponse struct {
			Records []struct {
				URI   string `json:"uri"`
				Value struct {
					Subject struct {
						URI string `json:"uri"`
					} `json:"subject"`
				} `json:"value"`
			} `json:"records"`
			Cursor *string `json:"cursor,omitempty"`
		}

		if err := json.Unmarshal(body, &listResponse); err != nil {
			return "", fmt.Errorf("failed to parse list response: %w", err)
		}

		// Search for like record matching the post URI
		for _, record := range listResponse.Records {
			if record.Value.Subject.URI == postURI {
				// Extract rkey from like record URI
				parts := strings.Split(record.URI, "/")
				if len(parts) > 0 {
					return parts[len(parts)-1], nil
				}
			}
		}

		// If there's no cursor, we've reached the end
		if listResponse.Cursor == nil {
			break
		}
		cursor = *listResponse.Cursor
	}

	return "", fmt.Errorf("no like record found for post %s", postURI)
}

// findRepostRecord finds the rkey for a repost record of a specific post
func (c *BlueskyClient) findRepostRecord(session *atpSessionResponse, postURI string) (string, error) {
	listURL := "https://bsky.social/xrpc/com.atproto.repo.listRecords"

	params := url.Values{}
	params.Add("repo", session.DID)
	params.Add("collection", "app.bsky.feed.repost")
	params.Add("limit", "100") // Start with reasonable limit

	var cursor string

	for {
		currentParams := url.Values{}
		for k, v := range params {
			currentParams[k] = v
		}
		if cursor != "" {
			currentParams.Add("cursor", cursor)
		}

		fullURL := fmt.Sprintf("%s?%s", listURL, currentParams.Encode())

		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create list request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

		client := &http.Client{Timeout: 30 * time.Second}
		LogHTTPRequest("GET", fullURL)
		resp, err := client.Do(req)
		LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)
		if err != nil {
			return "", fmt.Errorf("list request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("list request failed with status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read list response: %w", err)
		}

		var listResponse struct {
			Records []struct {
				URI   string `json:"uri"`
				Value struct {
					Subject struct {
						URI string `json:"uri"`
					} `json:"subject"`
				} `json:"value"`
			} `json:"records"`
			Cursor *string `json:"cursor,omitempty"`
		}

		if err := json.Unmarshal(body, &listResponse); err != nil {
			return "", fmt.Errorf("failed to parse list response: %w", err)
		}

		// Search for repost record matching the post URI
		for _, record := range listResponse.Records {
			if record.Value.Subject.URI == postURI {
				// Extract rkey from repost record URI
				parts := strings.Split(record.URI, "/")
				if len(parts) > 0 {
					return parts[len(parts)-1], nil
				}
			}
		}

		// If there's no cursor, we've reached the end
		if listResponse.Cursor == nil {
			break
		}
		cursor = *listResponse.Cursor
	}

	return "", fmt.Errorf("no repost record found for post %s", postURI)
}

// resolveDID attempts to resolve a DID to verify it exists and get current information
func (c *BlueskyClient) resolveDID(did string) error {
	resolveURL := fmt.Sprintf("https://bsky.social/xrpc/com.atproto.identity.resolveHandle?handle=%s", did)

	LogHTTPRequest("GET", resolveURL)
	resp, err := http.Get(resolveURL)
	LogHTTPResponse("GET", resolveURL, resp.StatusCode, resp.Status)
	if err != nil {
		return fmt.Errorf("failed to resolve DID %s: %w", did, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DID resolution failed for %s with status %d: %s", did, resp.StatusCode, string(body))
	}

	return nil
}

// validatePostURI validates that a post URI belongs to the authenticated user
func (c *BlueskyClient) validatePostURI(postURI string, userDID string) error {
	parts := strings.Split(postURI, "/")
	if len(parts) < 5 {
		return fmt.Errorf("invalid post URI format: %s", postURI)
	}

	postDID := parts[2]
	if postDID != userDID {
		return fmt.Errorf("post DID %s does not match authenticated user DID %s", postDID, userDID)
	}

	return nil
}

// fetchRepostPosts fetches the user's own repost records
func (c *BlueskyClient) fetchRepostPosts(session *atpSessionResponse, limit int) ([]Post, error) {
	listURL := "https://bsky.social/xrpc/com.atproto.repo.listRecords"

	params := url.Values{}
	params.Add("repo", session.DID)
	params.Add("collection", "app.bsky.feed.repost")
	params.Add("limit", fmt.Sprintf("%d", limit))

	fullURL := fmt.Sprintf("%s?%s", listURL, params.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("GET", fullURL)
	resp, err := client.Do(req)
	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)
	if err != nil {
		return nil, fmt.Errorf("list request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list response: %w", err)
	}

	var listResponse struct {
		Records []struct {
			URI   string `json:"uri"`
			Value struct {
				Subject struct {
					URI string `json:"uri"`
				} `json:"subject"`
				CreatedAt time.Time `json:"createdAt"`
			} `json:"value"`
		} `json:"records"`
	}

	if err := json.Unmarshal(body, &listResponse); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	var repostPosts []Post
	for _, record := range listResponse.Records {
		post := Post{
			ID:        record.URI, // This is the repost record URI, not the original post
			Type:      PostTypeRepost,
			Platform:  "bluesky",
			CreatedAt: record.Value.CreatedAt,
			Content:   fmt.Sprintf("Reposted: %s", record.Value.Subject.URI), // Show what was reposted
		}
		repostPosts = append(repostPosts, post)
	}

	return repostPosts, nil
}

// fetchLikedPosts fetches posts that the user has liked
func (c *BlueskyClient) fetchLikedPosts(session *atpSessionResponse, limit int) ([]Post, error) {
	listURL := "https://bsky.social/xrpc/com.atproto.repo.listRecords"

	params := url.Values{}
	params.Add("repo", session.DID)
	params.Add("collection", "app.bsky.feed.like")
	params.Add("limit", fmt.Sprintf("%d", limit))

	fullURL := fmt.Sprintf("%s?%s", listURL, params.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("GET", fullURL)
	resp, err := client.Do(req)
	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)
	if err != nil {
		return nil, fmt.Errorf("list request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list response: %w", err)
	}

	var listResponse struct {
		Records []struct {
			URI   string `json:"uri"`
			Value struct {
				Subject struct {
					URI string `json:"uri"`
				} `json:"subject"`
				CreatedAt time.Time `json:"createdAt"`
			} `json:"value"`
		} `json:"records"`
	}

	if err := json.Unmarshal(body, &listResponse); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	var likedPosts []Post
	for _, record := range listResponse.Records {
		post := Post{
			ID:        record.URI, // This is the like record URI, not the original post
			Type:      PostTypeLike,
			Platform:  "bluesky",
			CreatedAt: record.Value.CreatedAt,
			Content:   fmt.Sprintf("Liked: %s", record.Value.Subject.URI), // Show what was liked
		}
		likedPosts = append(likedPosts, post)
	}

	return likedPosts, nil
}

// deleteLikeRecord deletes a like record directly
func (c *BlueskyClient) deleteLikeRecord(creds *Credentials, likeURI string) error {
	session, err := c.ensureValidSession(creds)
	if err != nil {
		return fmt.Errorf("failed to ensure valid session: %w", err)
	}

	// Extract collection and rkey from like URI
	// URI format: at://did:plc:xxx/app.bsky.feed.like/rkey
	parts := strings.Split(likeURI, "/")
	if len(parts) < 5 {
		return fmt.Errorf("invalid like URI format: %s", likeURI)
	}

	did := parts[2]
	collection := strings.Join(parts[3:len(parts)-1], "/")
	rkey := parts[len(parts)-1]

	// Verify that the DID from the like URI matches the authenticated user's DID
	if did != session.DID {
		return fmt.Errorf("DID mismatch: like DID %s does not match authenticated user DID %s", did, session.DID)
	}

	deleteURL := "https://bsky.social/xrpc/com.atproto.repo.deleteRecord"

	deleteData := map[string]string{
		"repo":       session.DID,
		"collection": collection,
		"rkey":       rkey,
	}

	jsonData, err := json.Marshal(deleteData)
	if err != nil {
		return fmt.Errorf("failed to marshal delete data: %w", err)
	}

	req, err := http.NewRequest("POST", deleteURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("POST", deleteURL)
	resp, err := client.Do(req)
	LogHTTPResponse("POST", deleteURL, resp.StatusCode, resp.Status)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete like failed with status %d: %s. DID used: %s, rkey: %s", resp.StatusCode, string(body), session.DID, rkey)
	}

	return nil
}

// deleteRepostRecord deletes a repost record directly (simpler than unrepost)
func (c *BlueskyClient) deleteRepostRecord(creds *Credentials, repostURI string) error {
	session, err := c.ensureValidSession(creds)
	if err != nil {
		return fmt.Errorf("failed to ensure valid session: %w", err)
	}

	// Extract collection and rkey from repost URI
	// URI format: at://did:plc:xxx/app.bsky.feed.repost/rkey
	parts := strings.Split(repostURI, "/")
	if len(parts) < 5 {
		return fmt.Errorf("invalid repost URI format: %s", repostURI)
	}

	did := parts[2]
	collection := strings.Join(parts[3:len(parts)-1], "/")
	rkey := parts[len(parts)-1]

	// Verify that the DID from the repost URI matches the authenticated user's DID
	if did != session.DID {
		return fmt.Errorf("DID mismatch: repost DID %s does not match authenticated user DID %s", did, session.DID)
	}

	deleteURL := "https://bsky.social/xrpc/com.atproto.repo.deleteRecord"

	deleteData := map[string]string{
		"repo":       session.DID,
		"collection": collection,
		"rkey":       rkey,
	}

	jsonData, err := json.Marshal(deleteData)
	if err != nil {
		return fmt.Errorf("failed to marshal delete data: %w", err)
	}

	req, err := http.NewRequest("POST", deleteURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	LogHTTPRequest("POST", deleteURL)
	resp, err := client.Do(req)
	LogHTTPResponse("POST", deleteURL, resp.StatusCode, resp.Status)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete repost failed with status %d: %s. DID used: %s, rkey: %s", resp.StatusCode, string(body), session.DID, rkey)
	}

	return nil
}
