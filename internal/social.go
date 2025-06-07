package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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

	// FetchUserPostsPaginated retrieves posts with pagination support
	FetchUserPostsPaginated(username string, limit int, cursor string) ([]Post, string, error)

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

// HTTPClientConfig holds configuration for HTTP clients
type HTTPClientConfig struct {
	Timeout time.Duration
}

// CreateHTTPClient creates a standardized HTTP client with proper timeouts
func CreateHTTPClient(config HTTPClientConfig) *http.Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout: config.Timeout,
	}
}

// SessionManager provides common session management functionality
type SessionManager struct {
	credentials   *Credentials
	accessToken   string
	refreshToken  string
	sessionExpiry time.Time
	platform      string
}

// NewSessionManager creates a new session manager
func NewSessionManager(platform string) *SessionManager {
	return &SessionManager{
		platform: platform,
	}
}

// IsSessionValid checks if the current session is still valid
func (sm *SessionManager) IsSessionValid() bool {
	return sm.accessToken != "" && time.Now().Before(sm.sessionExpiry.Add(-5*time.Minute))
}

// HasCredentialsChanged checks if credentials have changed from the cached ones
func (sm *SessionManager) HasCredentialsChanged(creds *Credentials) bool {
	if sm.credentials == nil {
		return true
	}
	
	switch sm.platform {
	case "bluesky":
		return sm.credentials.Username != creds.Username || sm.credentials.AppPassword != creds.AppPassword
	case "mastodon":
		return sm.credentials.AccessToken != creds.AccessToken || sm.credentials.Instance != creds.Instance
	default:
		return true
	}
}

// UpdateSession updates the session with new tokens and expiry
func (sm *SessionManager) UpdateSession(accessToken, refreshToken string, expiry time.Time, creds *Credentials) {
	sm.accessToken = accessToken
	sm.refreshToken = refreshToken
	sm.sessionExpiry = expiry
	sm.credentials = &Credentials{
		Username:    creds.Username,
		AppPassword: creds.AppPassword,
		AccessToken: creds.AccessToken,
		Instance:    creds.Instance,
	}
}

// GetAccessToken returns the current access token
func (sm *SessionManager) GetAccessToken() string {
	return sm.accessToken
}

// GetRefreshToken returns the current refresh token
func (sm *SessionManager) GetRefreshToken() string {
	return sm.refreshToken
}

// ClearSession clears the current session
func (sm *SessionManager) ClearSession() {
	sm.credentials = nil
	sm.accessToken = ""
	sm.refreshToken = ""
	sm.sessionExpiry = time.Time{}
}

// AuthenticatedHTTPClient provides common authenticated HTTP request functionality
type AuthenticatedHTTPClient struct {
	client      *http.Client
	accessToken string
	baseURL     string
}

// NewAuthenticatedHTTPClient creates a new authenticated HTTP client
func NewAuthenticatedHTTPClient(accessToken, baseURL string, timeout time.Duration) *AuthenticatedHTTPClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	
	return &AuthenticatedHTTPClient{
		client:      &http.Client{Timeout: timeout},
		accessToken: accessToken,
		baseURL:     baseURL,
	}
}

// CreateRequest creates an HTTP request with authentication headers
func (ahc *AuthenticatedHTTPClient) CreateRequest(method, path string, body io.Reader) (*http.Request, error) {
	var url string
	if strings.HasPrefix(path, "http") {
		url = path
	} else {
		url = ahc.baseURL + path
	}
	
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	
	if ahc.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+ahc.accessToken)
	}
	req.Header.Set("Content-Type", "application/json")
	
	return req, nil
}

// DoRequest executes an HTTP request and returns the response
func (ahc *AuthenticatedHTTPClient) DoRequest(req *http.Request) (*http.Response, error) {
	LogHTTPRequest(req.Method, req.URL.String())
	
	resp, err := ahc.client.Do(req)
	if err != nil {
		logger := WithHTTP(req.Method, req.URL.String())
		logger.Error().Err(err).Msg("HTTP request failed")
		return nil, err
	}
	
	LogHTTPResponse(req.Method, req.URL.String(), resp.StatusCode, resp.Status)
	
	return resp, nil
}

// ParseErrorResponse extracts error information from HTTP response
func ParseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	err := fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	
	logger := WithHTTP("RESPONSE", resp.Request.URL.String())
	logger.Error().
		Int("status_code", resp.StatusCode).
		Str("response_body", string(body)).
		Msg("API request failed")
	
	return err
}

// TruncateContent truncates content for display in progress messages
func TruncateContent(content string, maxLen int) string {
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

// RateLimiter provides common rate limiting functionality
type RateLimiter struct {
	delay time.Duration
}

// NewRateLimiter creates a new rate limiter with the specified delay
func NewRateLimiter(delay time.Duration) *RateLimiter {
	return &RateLimiter{delay: delay}
}

// Wait sleeps for the configured delay
func (rl *RateLimiter) Wait() {
	if rl.delay > 0 {
		time.Sleep(rl.delay)
	}
}

// APIListRequest represents a common pattern for paginated list requests
type APIListRequest struct {
	URL        string
	Params     url.Values
	Limit      int
	Collection string // For AT Protocol
	Repo       string // For AT Protocol
}

// ExecuteListRequest executes a paginated list request and returns the response body
func ExecuteListRequest(client *AuthenticatedHTTPClient, request APIListRequest) ([]byte, error) {
	logger := WithOperation("list_request")
	logger.Debug().
		Str("url", request.URL).
		Int("limit", request.Limit).
		Str("collection", request.Collection).
		Str("repo", request.Repo).
		Msg("Executing list request")
	
	params := url.Values{}
	for k, v := range request.Params {
		params[k] = v
	}
	
	if request.Limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", request.Limit))
	}
	
	if request.Collection != "" {
		params.Add("collection", request.Collection)
	}
	
	if request.Repo != "" {
		params.Add("repo", request.Repo)
	}
	
	fullURL := request.URL
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}
	
	req, err := client.CreateRequest("GET", fullURL, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create list request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := client.DoRequest(req)
	if err != nil {
		logger.Error().Err(err).Msg("List request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, ParseErrorResponse(resp)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read list response body")
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	logger.Debug().
		Int("response_size", len(body)).
		Msg("List request completed successfully")
	
	return body, nil
}

// DeleteRecordRequest represents a common delete record request
type DeleteRecordRequest struct {
	Repo       string
	Collection string
	RKey       string
}

// ExecuteDeleteRequest executes a delete record request
func ExecuteDeleteRequest(client *AuthenticatedHTTPClient, deleteURL string, request DeleteRecordRequest) error {
	logger := WithOperation("delete_request")
	logger.Info().
		Str("repo", request.Repo).
		Str("collection", request.Collection).
		Str("rkey", request.RKey).
		Msg("Executing delete request")
	
	deleteData := map[string]string{
		"repo":       request.Repo,
		"collection": request.Collection,
		"rkey":       request.RKey,
	}
	
	jsonData, err := json.Marshal(deleteData)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal delete data")
		return fmt.Errorf("failed to marshal delete data: %w", err)
	}
	
	req, err := client.CreateRequest("POST", deleteURL, strings.NewReader(string(jsonData)))
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create delete request")
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	
	resp, err := client.DoRequest(req)
	if err != nil {
		logger.Error().Err(err).Msg("Delete request failed")
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return ParseErrorResponse(resp)
	}
	
	logger.Info().Msg("Delete request completed successfully")
	return nil
}

// ExtractPostIDFromURI extracts the post ID from a URI
func ExtractPostIDFromURI(uri string) string {
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

// ValidateURIOwnership validates that a URI belongs to the specified owner DID/ID
func ValidateURIOwnership(uri, ownerID string) error {
	parts := strings.Split(uri, "/")
	if len(parts) < 3 {
		return fmt.Errorf("invalid URI format: %s", uri)
	}
	
	uriOwner := parts[2]
	if uriOwner != ownerID {
		return fmt.Errorf("URI owner %s does not match expected owner %s", uriOwner, ownerID)
	}
	
	return nil
}
