package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MastodonClient implements the SocialClient interface for Mastodon
type MastodonClient struct{}

// NewMastodonClient creates a new Mastodon client
func NewMastodonClient() *MastodonClient {
	return &MastodonClient{}
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
			post.OriginalAuthor = status.Reblog.Account.DisplayName
			post.OriginalHandle = status.Reblog.Account.Acct
			post.Content = c.stripHTML(status.Reblog.Content)
			// Create embedded original post
			post.OriginalPost = &Post{
				ID:        status.Reblog.ID,
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
				// We'd need additional API call to get the account info
				// For now, just store the ID
			}
		}
		
		posts = append(posts, post)
	}

	return posts, nil
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
	
	resp, err := http.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("failed to lookup account: %w", err)
	}
	defer resp.Body.Close()

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
	params.Add("exclude_reblogs", "true")
	
	fullURL := fmt.Sprintf("%s?%s", statusesURL, params.Encode())
	
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch statuses: %w", err)
	}
	defer resp.Body.Close()

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
	params.Add("exclude_reblogs", "true")
	
	fullURL := fmt.Sprintf("%s?%s", statusesURL, params.Encode())
	
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch statuses: %w", err)
	}
	defer resp.Body.Close()

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

// stripHTML removes HTML tags from content (basic implementation)
func (c *MastodonClient) stripHTML(content string) string {
	// Simple HTML tag removal - for production use, consider using a proper HTML parser
	result := content
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")
	result = strings.ReplaceAll(result, "</p>", "\n")
	
	// Remove all other HTML tags (simple regex would be better)
	for strings.Contains(result, "<") && strings.Contains(result, ">") {
		start := strings.Index(result, "<")
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}
	
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
						fmt.Printf("👍 Unliked post from %s: %s\n", post.CreatedAt.Format("2006-01-02"), c.truncateContent(post.Content, 50))
						result.UnlikedCount++
					}
				}
			} else if options.UnshareReposts && post.Type == PostTypeRepost {
				result.PostsToUnshare = append(result.PostsToUnshare, post)
				if !options.DryRun {
					// Add configurable delay to respect rate limits
					time.Sleep(options.RateLimitDelay)
					if err := c.unreblogPost(creds, post.ID); err != nil {
						fmt.Printf("❌ Failed to unreblog post from %s: %v\n", post.CreatedAt.Format("2006-01-02"), err)
						result.Errors = append(result.Errors, fmt.Sprintf("Failed to unreblog post %s: %v", post.ID, err))
						result.ErrorsCount++
					} else {
						fmt.Printf("🔄 Unshared reblog from %s: %s\n", post.CreatedAt.Format("2006-01-02"), c.truncateContent(post.Content, 50))
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
						fmt.Printf("🗑️  Deleted post from %s: %s\n", post.CreatedAt.Format("2006-01-02"), c.truncateContent(post.Content, 50))
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
	url := fmt.Sprintf("%s/api/v1/statuses/%s", creds.Instance, postID)
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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
	url := fmt.Sprintf("%s/api/v1/statuses/%s/unfavourite", creds.Instance, postID)
	
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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
	url := fmt.Sprintf("%s/api/v1/statuses/%s/unreblog", creds.Instance, postID)
	
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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

// truncateContent truncates content for display in progress messages
func (c *MastodonClient) truncateContent(content string, maxLen int) string {
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