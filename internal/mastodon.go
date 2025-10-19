package internal

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MastodonClient implements the SocialClient interface for Mastodon
type MastodonClient struct {
	sessionManager      *SessionManager
	authenticatedClient *AuthenticatedHTTPClient
	instanceURL         string
}

// NewMastodonClient creates a new Mastodon client
func NewMastodonClient() *MastodonClient {
	return &MastodonClient{
		sessionManager: NewSessionManager("mastodon"),
	}
}

// GetPlatformName returns the platform name
func (c *MastodonClient) GetPlatformName() string {
	return "Mastodon"
}

// RequiresAuth returns true if the platform requires authentication for deletion
func (c *MastodonClient) RequiresAuth() bool {
	return true // Mastodon requires authentication for post deletion
}

// FetchUserPosts retrieves recent posts for a Mastodon user
func (c *MastodonClient) FetchUserPosts(username string, limit int) ([]Post, error) {
	instanceURL, acct, err := c.parseUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid username format: %w", err)
	}

	// First, get the account ID
	accountID, err := c.getAccountID(instanceURL, acct)
	if err != nil {
		return nil, fmt.Errorf("failed to get account ID: %w", err)
	}

	// Check if we have authentication for enhanced data
	var statuses []mastodonStatus
	creds, authErr := GetCredentialsForPlatform("mastodon")
	if authErr == nil && ValidateCredentials(creds) == nil {
		// Use authenticated fetch for viewer interaction data
		statuses, err = c.fetchUserStatusesAuthenticated(instanceURL, accountID, limit, creds)
	} else {
		// Use public fetch without viewer data
		statuses, err = c.fetchUserStatuses(instanceURL, accountID, limit)
		creds = nil // Ensure no credentials used for public fetch
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch statuses: %w", err)
	}

	// Convert to generic Post format
	var posts []Post
	for _, status := range statuses {
		post := Post{
			ID:        status.ID,
			Author:    status.Account.DisplayName,
			Handle:    status.Account.Acct,
			Content:   c.stripHTML(status.Content),
			CreatedAt: status.CreatedAt,
			URL:       status.URL,
			Type:      c.determinePostType(status),
			Platform:  "mastodon",

			// Engagement metrics
			RepostCount: status.ReblogsCount,
			LikeCount:   status.FavouritesCount,
			ReplyCount:  status.RepliesCount,

			// Viewer interaction status
			IsLikedByUser: status.Favourited != nil && *status.Favourited,
			IsPinned:      status.Pinned != nil && *status.Pinned,
		}

		// Handle reblogs/reposts
		if status.Reblog != nil {
			post.Type = PostTypeRepost
			// IMPORTANT: For reblogs, post.ID should be the reblog status ID (for unrebogging)
			// status.ID is the reblog action ID, status.Reblog.ID is the original post ID
			post.ID = status.ID // This is the reblog action ID we need to unreblog
			post.OriginalAuthor = status.Reblog.Account.DisplayName
			post.OriginalHandle = status.Reblog.Account.Acct
			post.Content = c.stripHTML(status.Reblog.Content)
			// Create embedded original post
			post.OriginalPost = &Post{
				ID:        status.Reblog.ID, // Original post ID
				Author:    status.Reblog.Account.DisplayName,
				Handle:    status.Reblog.Account.Acct,
				Content:   c.stripHTML(status.Reblog.Content),
				CreatedAt: status.Reblog.CreatedAt,
				URL:       status.Reblog.URL,
				Type:      PostTypeOriginal,
				Platform:  "mastodon",
			}
		}

		// Handle replies
		if status.InReplyToID != nil {
			post.Type = PostTypeReply
			post.InReplyToID = *status.InReplyToID
			if status.InReplyToAccountID != nil {
				// Fetch reply author information
				if replyAccount, err := c.fetchAccountInfo(instanceURL, *status.InReplyToAccountID, creds); err == nil {
					post.InReplyToAuthor = replyAccount.DisplayName
					if post.InReplyToAuthor == "" {
						post.InReplyToAuthor = replyAccount.Acct
					}
				}
				// Continue silently if account fetch fails to avoid disrupting the main operation
			}
		}

		posts = append(posts, post)
	}

	return posts, nil
}

// FetchUserPostsPaginated retrieves posts with pagination support using max_id
func (c *MastodonClient) FetchUserPostsPaginated(username string, limit int, cursor string) ([]Post, string, error) {
	instanceURL, acct, err := c.parseUsername(username)
	if err != nil {
		return nil, "", fmt.Errorf("invalid username format: %w", err)
	}

	// First, get the account ID
	accountID, err := c.getAccountID(instanceURL, acct)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get account ID: %w", err)
	}

	// Check if we have authentication for enhanced data
	var statuses []mastodonStatus
	var nextCursor string
	creds, authErr := GetCredentialsForPlatform("mastodon")
	if authErr == nil && ValidateCredentials(creds) == nil {
		// Use authenticated fetch for viewer interaction data
		statuses, nextCursor, err = c.fetchUserStatusesPaginated(instanceURL, accountID, limit, cursor, creds)
	} else {
		// Use public fetch without viewer data
		statuses, nextCursor, err = c.fetchUserStatusesPaginatedPublic(instanceURL, accountID, limit, cursor)
		creds = nil // Ensure no credentials used for public fetch
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch statuses: %w", err)
	}

	// Convert to generic Post format (same logic as FetchUserPosts)
	var posts []Post
	for _, status := range statuses {
		post := Post{
			ID:        status.ID,
			Author:    status.Account.DisplayName,
			Handle:    status.Account.Acct,
			Content:   c.stripHTML(status.Content),
			CreatedAt: status.CreatedAt,
			URL:       status.URL,
			Type:      c.determinePostType(status),
			Platform:  "mastodon",

			// Engagement metrics
			RepostCount: status.ReblogsCount,
			LikeCount:   status.FavouritesCount,
			ReplyCount:  status.RepliesCount,

			// Viewer interaction status
			IsLikedByUser: status.Favourited != nil && *status.Favourited,
			IsPinned:      status.Pinned != nil && *status.Pinned,
		}

		// Handle reblogs/reposts
		if status.Reblog != nil {
			post.Type = PostTypeRepost
			// IMPORTANT: For reblogs, post.ID should be the reblog status ID (for unrebogging)
			// status.ID is the reblog action ID, status.Reblog.ID is the original post ID
			post.ID = status.ID // This is the reblog action ID we need to unreblog
			post.OriginalAuthor = status.Reblog.Account.DisplayName
			post.OriginalHandle = status.Reblog.Account.Acct
			post.Content = c.stripHTML(status.Reblog.Content)
			// Create embedded original post
			post.OriginalPost = &Post{
				ID:        status.Reblog.ID, // Original post ID
				Author:    status.Reblog.Account.DisplayName,
				Handle:    status.Reblog.Account.Acct,
				Content:   c.stripHTML(status.Reblog.Content),
				CreatedAt: status.Reblog.CreatedAt,
				URL:       status.Reblog.URL,
				Type:      PostTypeOriginal,
				Platform:  "mastodon",
			}
		}

		// Handle replies
		if status.InReplyToID != nil {
			post.Type = PostTypeReply
			post.InReplyToID = *status.InReplyToID
			if status.InReplyToAccountID != nil {
				// Fetch reply author information
				if replyAccount, err := c.fetchAccountInfo(instanceURL, *status.InReplyToAccountID, creds); err == nil {
					post.InReplyToAuthor = replyAccount.DisplayName
					if post.InReplyToAuthor == "" {
						post.InReplyToAuthor = replyAccount.Acct
					}
				}
				// Continue silently if account fetch fails to avoid disrupting the main operation
			}
		}

		posts = append(posts, post)
	}

	return posts, nextCursor, nil
}

func (c *MastodonClient) fetchUserStatusesPaginatedPublic(instanceURL, accountID string, limit int, maxID string) ([]mastodonStatus, string, error) {
	statusesURL := fmt.Sprintf("%s/api/v1/accounts/%s/statuses", instanceURL, accountID)

	params := url.Values{}
	params.Add("limit", strconv.Itoa(limit))
	params.Add("exclude_replies", "true")
	// Include reblogs so we can manage the user's own reblog actions
	params.Add("exclude_reblogs", "false")
	
	if maxID != "" {
		params.Add("max_id", maxID)
	}

	fullURL := fmt.Sprintf("%s?%s", statusesURL, params.Encode())

	LogHTTPRequest("GET", fullURL)
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch statuses: %w", err)
	}
	defer resp.Body.Close()

	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("statuses request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read statuses response: %w", err)
	}

	var statuses []mastodonStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, "", fmt.Errorf("failed to parse statuses response: %w", err)
	}

	// Determine next cursor from Link header or last status ID
	nextCursor := ""
	if len(statuses) > 0 {
		// Use the ID of the last status as the max_id for the next request
		nextCursor = statuses[len(statuses)-1].ID
	}

	return statuses, nextCursor, nil
}

func (c *MastodonClient) fetchUserStatusesPaginated(instanceURL, accountID string, limit int, maxID string, creds *Credentials) ([]mastodonStatus, string, error) {
	statusesURL := fmt.Sprintf("%s/api/v1/accounts/%s/statuses", instanceURL, accountID)

	params := url.Values{}
	params.Add("limit", strconv.Itoa(limit))
	params.Add("exclude_replies", "true")
	// Include reblogs so we can manage the user's own reblog actions
	params.Add("exclude_reblogs", "false")
	
	if maxID != "" {
		params.Add("max_id", maxID)
	}

	fullURL := fmt.Sprintf("%s?%s", statusesURL, params.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	LogHTTPRequest("GET", fullURL)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch statuses: %w", err)
	}
	defer resp.Body.Close()

	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("statuses request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read statuses response: %w", err)
	}

	var statuses []mastodonStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, "", fmt.Errorf("failed to parse statuses response: %w", err)
	}

	// Determine next cursor from last status ID
	nextCursor := ""
	if len(statuses) > 0 {
		// Use the ID of the last status as the max_id for the next request
		nextCursor = statuses[len(statuses)-1].ID
	}

	return statuses, nextCursor, nil
}

// parseUsername extracts instance URL and account from username
// Supports formats: user@instance.social or just user (assumes MASTODON_INSTANCE env var)
func (c *MastodonClient) parseUsername(username string) (instanceURL, acct string, err error) {
	if strings.Contains(username, "@") {
		parts := strings.Split(username, "@")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("username must be in format user@instance.social")
		}
		acct = parts[0]
		instanceURL = "https://" + parts[1]
	} else {
		// Just username provided, need instance from environment or default
		acct = username
		instanceURL = "https://mastodon.social" // Default instance
	}

	return instanceURL, acct, nil
}

// Mastodon API types
type mastodonAccount struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Acct        string `json:"acct"`
	DisplayName string `json:"display_name"`
}

type mastodonStatus struct {
	ID                 string          `json:"id"`
	URL                string          `json:"url"`
	Content            string          `json:"content"`
	CreatedAt          time.Time       `json:"created_at"`
	Account            mastodonAccount `json:"account"`
	InReplyToID        *string         `json:"in_reply_to_id"`
	InReplyToAccountID *string         `json:"in_reply_to_account_id"`
	Reblog             *mastodonStatus `json:"reblog"`
	ReblogsCount       int             `json:"reblogs_count"`
	FavouritesCount    int             `json:"favourites_count"`
	RepliesCount       int             `json:"replies_count"`

	// Viewer interaction fields
	Favourited *bool `json:"favourited,omitempty"` // Whether the authenticated user has favorited this status
	Reblogged  *bool `json:"reblogged,omitempty"`  // Whether the authenticated user has reblogged this status
	Pinned     *bool `json:"pinned,omitempty"`     // Whether this is a pinned status
}

// getAccountID looks up account ID by username
func (c *MastodonClient) getAccountID(instanceURL, acct string) (string, error) {
	lookupURL := fmt.Sprintf("%s/api/v1/accounts/lookup", instanceURL)

	params := url.Values{}
	params.Add("acct", acct)

	fullURL := fmt.Sprintf("%s?%s", lookupURL, params.Encode())

	LogHTTPRequest("GET", fullURL)
	resp, err := http.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("failed to lookup account: %w", err)
	}
	defer resp.Body.Close()

	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("account lookup failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read lookup response: %w", err)
	}

	var account mastodonAccount
	if err := json.Unmarshal(body, &account); err != nil {
		return "", fmt.Errorf("failed to parse account response: %w", err)
	}

	return account.ID, nil
}

// fetchUserStatuses gets statuses for an account ID
func (c *MastodonClient) fetchUserStatuses(instanceURL, accountID string, limit int) ([]mastodonStatus, error) {
	statusesURL := fmt.Sprintf("%s/api/v1/accounts/%s/statuses", instanceURL, accountID)

	params := url.Values{}
	params.Add("limit", strconv.Itoa(limit))
	params.Add("exclude_replies", "true")
	// Include reblogs so we can manage the user's own reblog actions
	params.Add("exclude_reblogs", "false")

	fullURL := fmt.Sprintf("%s?%s", statusesURL, params.Encode())

	LogHTTPRequest("GET", fullURL)
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch statuses: %w", err)
	}
	defer resp.Body.Close()

	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("statuses request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read statuses response: %w", err)
	}

	var statuses []mastodonStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, fmt.Errorf("failed to parse statuses response: %w", err)
	}

	return statuses, nil
}

// fetchUserStatusesAuthenticated gets statuses with viewer interaction data
func (c *MastodonClient) fetchUserStatusesAuthenticated(instanceURL, accountID string, limit int, creds *Credentials) ([]mastodonStatus, error) {
	statusesURL := fmt.Sprintf("%s/api/v1/accounts/%s/statuses", instanceURL, accountID)

	params := url.Values{}
	params.Add("limit", strconv.Itoa(limit))
	params.Add("exclude_replies", "true")
	// Include reblogs so we can manage the user's own reblog actions
	params.Add("exclude_reblogs", "false")

	fullURL := fmt.Sprintf("%s?%s", statusesURL, params.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	LogHTTPRequest("GET", fullURL)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch statuses: %w", err)
	}
	defer resp.Body.Close()

	LogHTTPResponse("GET", fullURL, resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("statuses request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read statuses response: %w", err)
	}

	var statuses []mastodonStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, fmt.Errorf("failed to parse statuses response: %w", err)
	}

	return statuses, nil
}

// stripHTML removes HTML tags from content using proper HTML parsing and regex
func (c *MastodonClient) stripHTML(content string) string {
	if content == "" {
		return ""
	}

	// First, handle common HTML entities
	result := html.UnescapeString(content)

	// Convert common block-level elements to newlines
	blockElements := []string{
		"</p>", "</div>", "</article>", "</section>", 
		"</header>", "</footer>", "</main>", "</aside>",
		"</h1>", "</h2>", "</h3>", "</h4>", "</h5>", "</h6>",
		"</li>", "</dd>", "</dt>",
	}
	for _, element := range blockElements {
		result = strings.ReplaceAll(result, element, element+"\n")
	}

	// Convert line break elements to newlines
	lineBreaks := []string{"<br>", "<br/>", "<br />", "<br>\n", "<br/>\n", "<br />\n"}
	for _, br := range lineBreaks {
		result = strings.ReplaceAll(result, br, "\n")
	}

	// Use regex to remove all remaining HTML tags
	// This is more robust than the previous loop-based approach
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	result = htmlTagRegex.ReplaceAllString(result, "")

	// Clean up whitespace
	// Replace multiple consecutive newlines with double newlines (paragraph breaks)
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	// Replace multiple consecutive spaces with single spaces
	multipleSpaces := regexp.MustCompile(`[ \t]+`)
	result = multipleSpaces.ReplaceAllString(result, " ")

	// Trim leading/trailing whitespace from each line
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	// Remove empty lines at the beginning and end, but preserve internal structure
	result = strings.Join(lines, "\n")
	return strings.TrimSpace(result)
}

// PrunePosts deletes posts according to specified criteria
func (c *MastodonClient) PrunePosts(username string, options PruneOptions) (*PruneResult, error) {
	// Get authentication credentials
	creds, err := GetCredentialsForPlatform("mastodon")
	if err != nil {
		return nil, fmt.Errorf("authentication required: %w", err)
	}

	if err := ValidateCredentials(creds); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	// Parse username to get instance URL
	instanceURL, _, err := c.parseUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid username format: %w", err)
	}

	// Fetch ALL user's posts using pagination to ensure we process posts older than 60 days
	var allPosts []Post
	cursor := ""
	batchSize := 100
	
	for {
		posts, nextCursor, err := c.FetchUserPostsPaginated(username, batchSize, cursor)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch posts: %w", err)
		}
		
		if len(posts) == 0 {
			break // No more posts to fetch
		}
		
		allPosts = append(allPosts, posts...)
		
		// Check if we should continue fetching based on age criteria
		shouldContinue := false
		if options.MaxAge != nil || options.BeforeDate != nil {
			for _, post := range posts {
				// If any post in this batch matches the age criteria, continue fetching
				if options.MaxAge != nil && time.Now().Sub(post.CreatedAt) > *options.MaxAge {
					shouldContinue = true
					break
				}
				if options.BeforeDate != nil && post.CreatedAt.Before(*options.BeforeDate) {
					shouldContinue = true
					break
				}
			}
		}
		
		if nextCursor == "" || !shouldContinue {
			break // No more pages or no posts match age criteria
		}
		
		cursor = nextCursor
	}
	
	posts := allPosts

	// If user wants to unlike posts, also fetch their favorited posts
	if options.UnlikePosts {
		favoriteIDs, err := c.fetchFavoriteIDs(instanceURL, creds, 100)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to fetch favorited posts: %v\n", err)
		} else {
			// Convert favorite IDs to Post structs for processing
			for _, favoriteID := range favoriteIDs {
				favoritePost := Post{
					ID:        favoriteID,
					Type:      PostTypeLike,
					Platform:  "mastodon",
					CreatedAt: time.Now(), // We don't have the actual favorite time
					Content:   fmt.Sprintf("Favorited status: %s", favoriteID),
				}
				posts = append(posts, favoritePost)
			}
		}
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
				// Handle favorite records - unfavorite them
				result.PostsToUnlike = append(result.PostsToUnlike, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					logger := WithPlatform("mastodon").With().Str("post_id", post.ID).Logger()
					if err := c.unlikePost(creds, post.ID); err != nil {
						logger.Error().Err(err).Msg("Failed to unfavorite post")
						fmt.Printf("❌ Failed to unfavorite post: %v\n", err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to unfavorite post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						logger.Info().Str("content", TruncateContent(post.Content, 50)).Msg("Post unfavorited successfully")
						fmt.Printf("👍 Unfavorited post: %s\n", TruncateContent(post.Content, 50))
						result.UnlikedCount++
					}
				}
			} else if post.Type == PostTypeRepost {
				// Always unreblog for reblog records - these are the user's own reblog actions
				result.PostsToUnshare = append(result.PostsToUnshare, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					logger := WithPlatform("mastodon").With().Str("post_id", post.ID).Logger()
					if err := c.unreblogPost(creds, post.ID); err != nil {
						logger.Error().Err(err).Msg("Failed to unreblog post")
						fmt.Printf("❌ Failed to unreblog post from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to unreblog post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						logger.Info().Str("content", TruncateContent(post.Content, 50)).Msg("Reblog unshared successfully")
						fmt.Printf("🔄 Unshared reblog from %s: %s\n", post.CreatedAt.Format("2006-01-02"), TruncateContent(post.Content, 50))
						result.UnsharedCount++
					}
				}
			} else if post.Type == PostTypeOriginal || post.Type == PostTypeReply {
				// Only delete the user's own original posts and replies
				result.PostsToDelete = append(result.PostsToDelete, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					logger := WithPlatform("mastodon").With().Str("post_id", post.ID).Logger()
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

// deletePost deletes a Mastodon post
func (c *MastodonClient) deletePost(creds *Credentials, postID string) error {
	c.ensureAuthenticated(creds, creds.Instance)
	url := fmt.Sprintf("%s/api/v1/statuses/%s", creds.Instance, postID)

	req, err := c.authenticatedClient.CreateRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.authenticatedClient.DoRequest(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// unlikePost unlikes (unfavourites) a Mastodon post
func (c *MastodonClient) unlikePost(creds *Credentials, postID string) error {
	c.ensureAuthenticated(creds, creds.Instance)
	url := fmt.Sprintf("%s/api/v1/statuses/%s/unfavourite", creds.Instance, postID)

	req, err := c.authenticatedClient.CreateRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.authenticatedClient.DoRequest(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// unreblogPost unreblogs (unshares) a Mastodon post
func (c *MastodonClient) unreblogPost(creds *Credentials, postID string) error {
	c.ensureAuthenticated(creds, creds.Instance)
	url := fmt.Sprintf("%s/api/v1/statuses/%s/unreblog", creds.Instance, postID)

	req, err := c.authenticatedClient.CreateRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.authenticatedClient.DoRequest(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// determinePostType determines the type of Mastodon post
func (c *MastodonClient) determinePostType(status mastodonStatus) PostType {
	if status.Reblog != nil {
		return PostTypeRepost
	}
	if status.InReplyToID != nil {
		return PostTypeReply
	}
	return PostTypeOriginal
}

// ensureAuthenticated ensures we have cached authentication details
func (c *MastodonClient) ensureAuthenticated(creds *Credentials, instanceURL string) {
	// Cache credentials and instance URL for reuse
	logger := WithPlatform("mastodon")
	if c.sessionManager.HasCredentialsChanged(creds) || c.instanceURL != instanceURL {
		if c.sessionManager.HasCredentialsChanged(creds) {
			logger.Debug().Str("instance", instanceURL).Msg("Setting up Mastodon authentication")
			fmt.Printf("🔐 Setting up Mastodon authentication for %s...\n", instanceURL)
		}
		c.sessionManager.UpdateSession(creds.AccessToken, "", time.Now().Add(24*time.Hour), creds)
		c.authenticatedClient = NewAuthenticatedHTTPClient(creds.AccessToken, instanceURL, 30*time.Second)
		c.instanceURL = instanceURL
	}
}


// fetchAccountInfo fetches account information by account ID
func (c *MastodonClient) fetchAccountInfo(instanceURL, accountID string, creds *Credentials) (*mastodonAccount, error) {
	accountURL := fmt.Sprintf("%s/api/v1/accounts/%s", instanceURL, accountID)

	req, err := http.NewRequest("GET", accountURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header if credentials are provided
	if creds != nil {
		req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	}

	LogHTTPRequest("GET", accountURL)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}
	defer resp.Body.Close()

	LogHTTPResponse("GET", accountURL, resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("account request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read account response: %w", err)
	}

	var account mastodonAccount
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("failed to parse account response: %w", err)
	}

	return &account, nil
}

// fetchFavoriteIDs fetches IDs of posts that the user has favorited
func (c *MastodonClient) fetchFavoriteIDs(instanceURL string, creds *Credentials, limit int) ([]string, error) {
	c.ensureAuthenticated(creds, instanceURL)
	favoritesURL := fmt.Sprintf("%s/api/v1/favourites", instanceURL)

	params := url.Values{}
	params.Add("limit", strconv.Itoa(limit))

	fullURL := fmt.Sprintf("%s?%s", favoritesURL, params.Encode())

	req, err := c.authenticatedClient.CreateRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.authenticatedClient.DoRequest(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
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

	var statuses []mastodonStatus
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var favoriteIDs []string
	for _, status := range statuses {
		favoriteIDs = append(favoriteIDs, status.ID)
	}

	return favoriteIDs, nil
}

