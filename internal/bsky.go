package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BlueskyClient implements the SocialClient interface for Bluesky
type BlueskyClient struct{}

// NewBlueskyClient creates a new Bluesky client
func NewBlueskyClient() *BlueskyClient {
	return &BlueskyClient{}
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

		// Handle reposts/quotes
		if bskyPost.Record.Type == "app.bsky.feed.repost" {
			post.Type = PostTypeRepost
			// Note: Full repost data would require additional API calls to get original post
		}

		// Handle replies
		if bskyPost.Record.Reply != nil {
			post.Type = PostTypeReply
			post.InReplyToID = bskyPost.Record.Reply.Parent.URI
		}

		genericPosts = append(genericPosts, post)
	}

	return genericPosts, nil
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
	params.Add("filter", "posts_with_replies") // Include replies for complete view

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts: %w", err)
	}
	defer resp.Body.Close()

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

	// Fetch user's posts (we'd need to fetch more than 10 for real pruning)
	posts, err := c.FetchUserPosts(username, 100) // Fetch more posts for pruning
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts: %w", err)
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
			// Determine action based on flags and post type
			if options.UnlikePosts && post.IsLikedByUser {
				result.PostsToUnlike = append(result.PostsToUnlike, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					if err := c.unlikePost(creds, post.ID); err != nil {
						fmt.Printf("❌ Failed to unlike post from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to unlike post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						fmt.Printf("👍 Unliked post from %s: %s\n", post.CreatedAt.Format("2006-01-02"), truncateContent(post.Content, 50))
						result.UnlikedCount++
					}
				}
			} else if options.UnshareReposts && post.Type == PostTypeRepost {
				result.PostsToUnshare = append(result.PostsToUnshare, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					if err := c.unrepost(creds, post.ID); err != nil {
						fmt.Printf("❌ Failed to unrepost from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to unrepost post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						fmt.Printf("🔄 Unshared repost from %s: %s\n", post.CreatedAt.Format("2006-01-02"), truncateContent(post.Content, 50))
						result.UnsharedCount++
					}
				}
			} else {
				result.PostsToDelete = append(result.PostsToDelete, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					if err := c.deletePost(creds, post.ID); err != nil {
						fmt.Printf("❌ Failed to delete post from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to delete post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						fmt.Printf("🗑️  Deleted post from %s: %s\n", post.CreatedAt.Format("2006-01-02"), truncateContent(post.Content, 50))
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
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("session request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("session creation failed with status %d: %s", resp.StatusCode, string(body))
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
	session, err := c.createSession(creds)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
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

	deleteURL := "https://bsky.social/xrpc/com.atproto.repo.deleteRecord"

	deleteData := map[string]string{
		"repo":       did,
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
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// unlikePost removes a like from a Bluesky post
func (c *BlueskyClient) unlikePost(creds *Credentials, postURI string) error {
	session, err := c.createSession(creds)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
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
	resp, err := client.Do(req)
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
	session, err := c.createSession(creds)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
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
	resp, err := client.Do(req)
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
		resp, err := client.Do(req)
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
		resp, err := client.Do(req)
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

// truncateContent truncates content for display in progress messages
func truncateContent(content string, maxLen int) string {
	// Replace newlines with spaces for display
	content = strings.ReplaceAll(content, "\n", " ")
	if len(content) <= maxLen {
		return content
	}
	if maxLen <= 3 {
		return "..."
	}
	return content[:maxLen-3] + "..."
}
