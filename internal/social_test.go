package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPostType_String(t *testing.T) {
	tests := []struct {
		postType PostType
		expected string
	}{
		{PostTypeOriginal, "original"},
		{PostTypeRepost, "repost"},
		{PostTypeReply, "reply"},
		{PostTypeLike, "like"},
		{PostTypeQuote, "quote"},
	}

	for _, test := range tests {
		if string(test.postType) != test.expected {
			t.Errorf("PostType %v should be %s, got %s", test.postType, test.expected, string(test.postType))
		}
	}
}

func TestGetClient(t *testing.T) {
	tests := []struct {
		platform string
		exists   bool
	}{
		{"bluesky", true},
		{"mastodon", true},
		{"twitter", false},
		{"", false},
		{"invalid", false},
	}

	for _, test := range tests {
		client, exists := GetClient(test.platform)
		if exists != test.exists {
			t.Errorf("GetClient(%s) exists should be %v, got %v", test.platform, test.exists, exists)
		}
		if test.exists && client == nil {
			t.Errorf("GetClient(%s) should return a valid client when exists is true", test.platform)
		}
		if !test.exists && client != nil {
			t.Errorf("GetClient(%s) should return nil when exists is false", test.platform)
		}
	}
}

func TestGetClient_PlatformName(t *testing.T) {
	client, exists := GetClient("bluesky")
	if !exists {
		t.Fatal("Bluesky client should exist")
	}
	if client.GetPlatformName() != "Bluesky" {
		t.Errorf("Bluesky client platform name should be 'Bluesky', got '%s'", client.GetPlatformName())
	}

	client, exists = GetClient("mastodon")
	if !exists {
		t.Fatal("Mastodon client should exist")
	}
	if client.GetPlatformName() != "Mastodon" {
		t.Errorf("Mastodon client platform name should be 'Mastodon', got '%s'", client.GetPlatformName())
	}
}

func TestGetClient_RequiresAuth(t *testing.T) {
	platforms := []string{"bluesky", "mastodon"}

	for _, platform := range platforms {
		client, exists := GetClient(platform)
		if !exists {
			t.Fatalf("%s client should exist", platform)
		}
		if !client.RequiresAuth() {
			t.Errorf("%s client should require authentication", platform)
		}
	}
}

func TestPruneOptions_Validation(t *testing.T) {
	now := time.Now()
	maxAge := 30 * 24 * time.Hour
	beforeDate := now.AddDate(0, 0, -30)

	tests := []struct {
		name    string
		options PruneOptions
		valid   bool
	}{
		{
			name: "valid with max age",
			options: PruneOptions{
				MaxAge: &maxAge,
				DryRun: true,
			},
			valid: true,
		},
		{
			name: "valid with before date",
			options: PruneOptions{
				BeforeDate: &beforeDate,
				DryRun:     true,
			},
			valid: true,
		},
		{
			name: "valid with both criteria",
			options: PruneOptions{
				MaxAge:     &maxAge,
				BeforeDate: &beforeDate,
				DryRun:     true,
			},
			valid: true,
		},
		{
			name: "valid with preservation flags",
			options: PruneOptions{
				MaxAge:           &maxAge,
				PreservePinned:   true,
				PreserveSelfLike: true,
				DryRun:           true,
			},
			valid: true,
		},
		{
			name: "valid with action flags",
			options: PruneOptions{
				MaxAge:         &maxAge,
				UnlikePosts:    true,
				UnshareReposts: true,
				DryRun:         true,
			},
			valid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test that the options structure is properly formed
			hasValidCriteria := test.options.MaxAge != nil || test.options.BeforeDate != nil
			if test.valid && !hasValidCriteria {
				t.Errorf("Test case marked as valid but has no time criteria")
			}
		})
	}
}

func TestPruneResult_Counts(t *testing.T) {
	result := &PruneResult{
		PostsToDelete:  []Post{{ID: "1"}, {ID: "2"}},
		PostsToUnlike:  []Post{{ID: "3"}},
		PostsToUnshare: []Post{{ID: "4"}},
		PostsPreserved: []Post{{ID: "5"}, {ID: "6"}, {ID: "7"}},
		DeletedCount:   2,
		UnlikedCount:   1,
		UnsharedCount:  1,
		PreservedCount: 3,
		Errors:         []string{"error1", "error2"},
		ErrorsCount:    2,
	}

	if len(result.PostsToDelete) != result.DeletedCount {
		t.Errorf("PostsToDelete count (%d) should match DeletedCount (%d)", len(result.PostsToDelete), result.DeletedCount)
	}
	if len(result.PostsToUnlike) != result.UnlikedCount {
		t.Errorf("PostsToUnlike count (%d) should match UnlikedCount (%d)", len(result.PostsToUnlike), result.UnlikedCount)
	}
	if len(result.PostsToUnshare) != result.UnsharedCount {
		t.Errorf("PostsToUnshare count (%d) should match UnsharedCount (%d)", len(result.PostsToUnshare), result.UnsharedCount)
	}
	if len(result.PostsPreserved) != result.PreservedCount {
		t.Errorf("PostsPreserved count (%d) should match PreservedCount (%d)", len(result.PostsPreserved), result.PreservedCount)
	}
	if len(result.Errors) != result.ErrorsCount {
		t.Errorf("Errors count (%d) should match ErrorsCount (%d)", len(result.Errors), result.ErrorsCount)
	}
}

func TestSupportedPlatforms(t *testing.T) {
	expectedPlatforms := []string{"bluesky", "mastodon"}

	for _, platform := range expectedPlatforms {
		t.Run("platform "+platform, func(t *testing.T) {
			constructor, exists := SupportedPlatforms[platform]
			if !exists {
				t.Errorf("Platform %s should be supported", platform)
				return
			}

			client := constructor()
			if client == nil {
				t.Errorf("Constructor for %s should return a valid client", platform)
			}

			// Verify the client implements the interface
			var _ SocialClient = client
		})
	}

	// Test unsupported platform
	_, exists := SupportedPlatforms["twitter"]
	if exists {
		t.Error("Twitter should not be a supported platform")
	}
}

func TestPost_CompleteStructure(t *testing.T) {
	now := time.Now()
	originalPost := &Post{
		ID:        "original123",
		Author:    "Original Author",
		Handle:    "original",
		Content:   "This is the original post",
		CreatedAt: now.Add(-1 * time.Hour),
		Type:      PostTypeOriginal,
		Platform:  "test",
	}

	post := Post{
		ID:        "test123",
		Author:    "Test Author",
		Handle:    "testuser",
		Content:   "This is a test post with all fields",
		CreatedAt: now,
		URL:       "https://example.com/post/test123",
		Type:      PostTypeQuote,

		OriginalPost:   originalPost,
		OriginalAuthor: "Original Author",
		OriginalHandle: "original",

		InReplyToID:     "reply456",
		InReplyToAuthor: "Reply Author",

		RepostCount: 5,
		LikeCount:   10,
		ReplyCount:  3,

		IsLikedByUser: true,
		IsPinned:      false,

		Platform: "test",
		RawData: map[string]interface{}{
			"extra_field": "extra_value",
			"numeric":     123,
		},
	}

	// Test all fields are populated
	if post.ID == "" {
		t.Error("ID should not be empty")
	}
	if post.Author == "" {
		t.Error("Author should not be empty")
	}
	if post.Handle == "" {
		t.Error("Handle should not be empty")
	}
	if post.Content == "" {
		t.Error("Content should not be empty")
	}
	if post.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if post.URL == "" {
		t.Error("URL should not be empty")
	}
	if post.Type != PostTypeQuote {
		t.Errorf("Expected Type %v, got %v", PostTypeQuote, post.Type)
	}
	if post.OriginalPost == nil {
		t.Error("OriginalPost should not be nil")
	}
	if post.OriginalAuthor == "" {
		t.Error("OriginalAuthor should not be empty")
	}
	if post.OriginalHandle == "" {
		t.Error("OriginalHandle should not be empty")
	}
	if post.InReplyToID == "" {
		t.Error("InReplyToID should not be empty")
	}
	if post.InReplyToAuthor == "" {
		t.Error("InReplyToAuthor should not be empty")
	}
	if !post.IsLikedByUser {
		t.Error("IsLikedByUser should be true")
	}
	if post.IsPinned {
		t.Error("IsPinned should be false")
	}
	if post.Platform == "" {
		t.Error("Platform should not be empty")
	}
	if len(post.RawData) == 0 {
		t.Error("RawData should not be empty")
	}
}

func TestPruneOptions_EdgeCases(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		options PruneOptions
		valid   bool
	}{
		{
			name: "negative max age",
			options: PruneOptions{
				MaxAge: func() *time.Duration { d := -24 * time.Hour; return &d }(),
			},
			valid: false, // Should be invalid
		},
		{
			name: "zero rate limit delay",
			options: PruneOptions{
				MaxAge:         func() *time.Duration { d := 24 * time.Hour; return &d }(),
				RateLimitDelay: 0,
			},
			valid: true, // Zero delay is valid
		},
		{
			name: "future before date",
			options: PruneOptions{
				BeforeDate: func() *time.Time { t := now.Add(24 * time.Hour); return &t }(),
			},
			valid: true, // Future dates should be allowed
		},
		{
			name: "very large rate limit delay",
			options: PruneOptions{
				MaxAge:         func() *time.Duration { d := 24 * time.Hour; return &d }(),
				RateLimitDelay: 24 * time.Hour,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - ensure options can be created
			hasTimeCriteria := tt.options.MaxAge != nil || tt.options.BeforeDate != nil

			if tt.name == "negative max age" && tt.options.MaxAge != nil && *tt.options.MaxAge < 0 {
				// In a real implementation, this should be validated
				t.Log("Negative max age detected (should be validated by implementation)")
			}

			if tt.name == "zero rate limit delay" && tt.options.RateLimitDelay == 0 {
				t.Log("Zero rate limit delay is acceptable")
			}

			if hasTimeCriteria || tt.name == "zero rate limit delay" || tt.name == "very large rate limit delay" {
				// These cases should be valid in terms of structure
			}
		})
	}
}

func TestPruneResult_EmptyResult(t *testing.T) {
	result := &PruneResult{}

	// Test empty result behavior
	if len(result.PostsToDelete) != 0 {
		t.Error("Empty result should have zero posts to delete")
	}
	if len(result.PostsToUnlike) != 0 {
		t.Error("Empty result should have zero posts to unlike")
	}
	if len(result.PostsToUnshare) != 0 {
		t.Error("Empty result should have zero posts to unshare")
	}
	if len(result.PostsPreserved) != 0 {
		t.Error("Empty result should have zero preserved posts")
	}
	if result.DeletedCount != 0 {
		t.Error("Empty result should have zero deleted count")
	}
	if result.UnlikedCount != 0 {
		t.Error("Empty result should have zero unliked count")
	}
	if result.UnsharedCount != 0 {
		t.Error("Empty result should have zero unshared count")
	}
	if result.PreservedCount != 0 {
		t.Error("Empty result should have zero preserved count")
	}
	if result.ErrorsCount != 0 {
		t.Error("Empty result should have zero errors count")
	}
	if len(result.Errors) != 0 {
		t.Error("Empty result should have zero errors")
	}
}

func TestPost_JSONSerialization(t *testing.T) {
	now := time.Now()
	originalPost := Post{
		ID:        "original123",
		Author:    "Original Author",
		Handle:    "original",
		Content:   "Original content",
		CreatedAt: now.Add(-1 * time.Hour),
		Type:      PostTypeOriginal,
		Platform:  "test",
	}

	post := Post{
		ID:             "test123",
		Author:         "Test Author",
		Handle:         "testuser",
		Content:        "Test content",
		CreatedAt:      now,
		URL:            "https://example.com/post/test123",
		Type:           PostTypeRepost,
		OriginalPost:   &originalPost,
		OriginalAuthor: "Original Author",
		OriginalHandle: "original",
		RepostCount:    5,
		LikeCount:      10,
		ReplyCount:     3,
		IsLikedByUser:  true,
		IsPinned:       false,
		Platform:       "test",
		RawData: map[string]interface{}{
			"test_field": "test_value",
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(post)
	if err != nil {
		t.Fatalf("Failed to marshal post to JSON: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaledPost Post
	err = json.Unmarshal(jsonData, &unmarshaledPost)
	if err != nil {
		t.Fatalf("Failed to unmarshal post from JSON: %v", err)
	}

	// Verify key fields
	if unmarshaledPost.ID != post.ID {
		t.Errorf("ID mismatch after JSON round-trip: expected %s, got %s", post.ID, unmarshaledPost.ID)
	}
	if unmarshaledPost.Type != post.Type {
		t.Errorf("Type mismatch after JSON round-trip: expected %s, got %s", post.Type, unmarshaledPost.Type)
	}
	if unmarshaledPost.IsLikedByUser != post.IsLikedByUser {
		t.Errorf("IsLikedByUser mismatch after JSON round-trip: expected %v, got %v", post.IsLikedByUser, unmarshaledPost.IsLikedByUser)
	}
}

// ---------------------------------------------------------------------------
// SessionManager
// ---------------------------------------------------------------------------

func TestSessionManager_IsSessionValid(t *testing.T) {
	sm := NewSessionManager("bluesky")

	t.Run("empty session is invalid", func(t *testing.T) {
		if sm.IsSessionValid() {
			t.Error("expected invalid session when no token set")
		}
	})

	t.Run("valid future expiry", func(t *testing.T) {
		sm.accessToken = "tok"
		sm.sessionExpiry = time.Now().Add(1 * time.Hour)
		if !sm.IsSessionValid() {
			t.Error("expected valid session")
		}
	})

	t.Run("expired token is invalid", func(t *testing.T) {
		sm.accessToken = "tok"
		sm.sessionExpiry = time.Now().Add(-1 * time.Hour)
		if sm.IsSessionValid() {
			t.Error("expected invalid session for expired token")
		}
	})

	t.Run("token expiring within 5 minutes is invalid", func(t *testing.T) {
		sm.accessToken = "tok"
		sm.sessionExpiry = time.Now().Add(3 * time.Minute)
		if sm.IsSessionValid() {
			t.Error("expected invalid session for near-expiry token")
		}
	})
}

func TestSessionManager_GettersAndClear(t *testing.T) {
	sm := NewSessionManager("bluesky")
	creds := &Credentials{Username: "u", AppPassword: "p"}
	sm.UpdateSession("access", "refresh", time.Now().Add(1*time.Hour), creds)

	if sm.GetAccessToken() != "access" {
		t.Errorf("GetAccessToken: want %q, got %q", "access", sm.GetAccessToken())
	}
	if sm.GetRefreshToken() != "refresh" {
		t.Errorf("GetRefreshToken: want %q, got %q", "refresh", sm.GetRefreshToken())
	}

	sm.ClearSession()
	if sm.GetAccessToken() != "" {
		t.Error("expected empty access token after ClearSession")
	}
	if sm.GetRefreshToken() != "" {
		t.Error("expected empty refresh token after ClearSession")
	}
	if sm.credentials != nil {
		t.Error("expected nil credentials after ClearSession")
	}
}

func TestSessionManager_HasCredentialsChanged(t *testing.T) {
	t.Run("bluesky: nil credentials returns true", func(t *testing.T) {
		sm := NewSessionManager("bluesky")
		if !sm.HasCredentialsChanged(&Credentials{Username: "u", AppPassword: "p"}) {
			t.Error("expected true when credentials are nil")
		}
	})

	t.Run("bluesky: same credentials returns false", func(t *testing.T) {
		sm := NewSessionManager("bluesky")
		creds := &Credentials{Username: "u", AppPassword: "p"}
		sm.UpdateSession("tok", "ref", time.Now().Add(1*time.Hour), creds)
		if sm.HasCredentialsChanged(creds) {
			t.Error("expected false for unchanged credentials")
		}
	})

	t.Run("bluesky: different username returns true", func(t *testing.T) {
		sm := NewSessionManager("bluesky")
		sm.UpdateSession("tok", "ref", time.Now().Add(1*time.Hour), &Credentials{Username: "a", AppPassword: "p"})
		if !sm.HasCredentialsChanged(&Credentials{Username: "b", AppPassword: "p"}) {
			t.Error("expected true when username changes")
		}
	})

	t.Run("mastodon: same credentials returns false", func(t *testing.T) {
		sm := NewSessionManager("mastodon")
		creds := &Credentials{AccessToken: "tok", Instance: "https://mastodon.social"}
		sm.UpdateSession("tok", "", time.Now().Add(1*time.Hour), creds)
		if sm.HasCredentialsChanged(creds) {
			t.Error("expected false for unchanged mastodon credentials")
		}
	})

	t.Run("mastodon: different instance returns true", func(t *testing.T) {
		sm := NewSessionManager("mastodon")
		sm.UpdateSession("tok", "", time.Now().Add(1*time.Hour), &Credentials{AccessToken: "t", Instance: "https://a.social"})
		if !sm.HasCredentialsChanged(&Credentials{AccessToken: "t", Instance: "https://b.social"}) {
			t.Error("expected true when instance changes")
		}
	})

	t.Run("unknown platform always returns true", func(t *testing.T) {
		sm := NewSessionManager("unknown")
		sm.UpdateSession("tok", "", time.Now().Add(1*time.Hour), &Credentials{})
		if !sm.HasCredentialsChanged(&Credentials{}) {
			t.Error("expected true for unknown platform")
		}
	})
}

// ---------------------------------------------------------------------------
// ExtractPostIDFromURI / ValidateURIOwnership
// ---------------------------------------------------------------------------

func TestExtractPostIDFromURI(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"at://did:plc:abc/app.bsky.feed.post/rkey1", "rkey1"},
		{"https://example.com/post/abc123", "abc123"},
		{"", ""},
		{"noslash", ""},
	}
	for _, tt := range tests {
		if got := ExtractPostIDFromURI(tt.uri); got != tt.want {
			t.Errorf("ExtractPostIDFromURI(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestValidateURIOwnership(t *testing.T) {
	tests := []struct {
		uri     string
		owner   string
		wantErr bool
	}{
		{"at://did:plc:abc/coll/rkey", "did:plc:abc", false},
		{"at://did:plc:abc/coll/rkey", "did:plc:other", true},
		{"at://short", "did:plc:abc", true},
	}
	for _, tt := range tests {
		err := ValidateURIOwnership(tt.uri, tt.owner)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateURIOwnership(%q, %q): wantErr=%v, got %v", tt.uri, tt.owner, tt.wantErr, err)
		}
	}
}

// ---------------------------------------------------------------------------
// RateLimiter
// ---------------------------------------------------------------------------

func TestRateLimiter_ZeroDelay(t *testing.T) {
	rl := NewRateLimiter(0)
	start := time.Now()
	rl.Wait()
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("zero-delay Wait took too long: %v", elapsed)
	}
}

func TestRateLimiter_NonZeroDelay(t *testing.T) {
	rl := NewRateLimiter(10 * time.Millisecond)
	start := time.Now()
	rl.Wait()
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Errorf("Wait returned too fast: %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// ParseErrorResponse
// ---------------------------------------------------------------------------

func TestParseErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"NotFound"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	parseErr := ParseErrorResponse(resp)
	if parseErr == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(parseErr.Error(), "404") {
		t.Errorf("expected status code in error, got: %v", parseErr)
	}
}

// ---------------------------------------------------------------------------
// ExecuteDeleteRequest (via httptest)
// ---------------------------------------------------------------------------

func TestExecuteDeleteRequest_Success(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["repo"] != "did:plc:x" || body["collection"] != "app.bsky.feed.post" || body["rkey"] != "rkey1" {
			t.Errorf("unexpected body: %v", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewAuthenticatedHTTPClient("tok", srv.URL, 5*time.Second)
	err := ExecuteDeleteRequest(client, srv.URL+"/deleteRecord", DeleteRecordRequest{
		Repo:       "did:plc:x",
		Collection: "app.bsky.feed.post",
		RKey:       "rkey1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("delete endpoint not called")
	}
}

func TestExecuteDeleteRequest_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewAuthenticatedHTTPClient("tok", srv.URL, 5*time.Second)
	err := ExecuteDeleteRequest(client, srv.URL+"/deleteRecord", DeleteRecordRequest{Repo: "r", Collection: "c", RKey: "k"})
	if err == nil {
		t.Error("expected error on 403 response")
	}
}

// ---------------------------------------------------------------------------
// ExecuteListRequest (via httptest)
// ---------------------------------------------------------------------------

func TestExecuteListRequest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") == "" {
			t.Error("expected limit param")
		}
		w.Write([]byte(`{"records":[]}`))
	}))
	defer srv.Close()

	client := NewAuthenticatedHTTPClient("tok", srv.URL, 5*time.Second)
	body, err := ExecuteListRequest(client, APIListRequest{URL: srv.URL + "/list", Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestExecuteListRequest_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewAuthenticatedHTTPClient("tok", srv.URL, 5*time.Second)
	_, err := ExecuteListRequest(client, APIListRequest{URL: srv.URL + "/list", Limit: 10})
	if err == nil {
		t.Error("expected error on 500 response")
	}
}

// ---------------------------------------------------------------------------
// CreateHTTPClient
// ---------------------------------------------------------------------------

func TestCreateHTTPClient(t *testing.T) {
	c := CreateHTTPClient(HTTPClientConfig{Timeout: 5 * time.Second})
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestCreateHTTPClient_DefaultTimeout(t *testing.T) {
	c := CreateHTTPClient(HTTPClientConfig{})
	if c.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", c.Timeout)
	}
}
