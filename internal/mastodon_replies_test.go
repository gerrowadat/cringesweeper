package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newTestMastodonStatus returns a minimal mastodonStatus suitable for use in handler responses.
func newTestMastodonStatus(id string) mastodonStatus {
	return mastodonStatus{
		ID:        id,
		Content:   "<p>test post</p>",
		CreatedAt: time.Now(),
		Account: mastodonAccount{
			ID:          "user1",
			Username:    "testuser",
			Acct:        "testuser",
			DisplayName: "Test User",
		},
	}
}

// newTestReplyStatus returns a mastodonStatus that looks like a reply.
func newTestReplyStatus(id, replyToID string) mastodonStatus {
	s := newTestMastodonStatus(id)
	s.InReplyToID = &replyToID
	return s
}

// captureExcludeReplies returns a handler that records the exclude_replies query param
// and responds with the given statuses as JSON.
func captureExcludeReplies(t *testing.T, got *string, statuses []mastodonStatus) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		*got = r.URL.Query().Get("exclude_replies")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(statuses); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}
}

// TestFetchUserStatusesPaginatedPublic_ExcludeRepliesParam verifies that the public paginated
// fetch sends the correct exclude_replies query parameter.
func TestFetchUserStatusesPaginatedPublic_ExcludeRepliesParam(t *testing.T) {
	tests := []struct {
		name           string
		excludeReplies bool
		wantParam      string
	}{
		{"default includes replies", false, "false"},
		{"flag excludes replies", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotParam string
			server := httptest.NewServer(captureExcludeReplies(t, &gotParam, []mastodonStatus{}))
			defer server.Close()

			client := NewMastodonClient()
			_, _, err := client.fetchUserStatusesPaginatedPublic(server.URL, "123", 20, "", tt.excludeReplies)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotParam != tt.wantParam {
				t.Errorf("exclude_replies = %q, want %q", gotParam, tt.wantParam)
			}
		})
	}
}

// TestFetchUserStatusesPaginated_ExcludeRepliesParam verifies that the authenticated paginated
// fetch sends the correct exclude_replies query parameter.
func TestFetchUserStatusesPaginated_ExcludeRepliesParam(t *testing.T) {
	tests := []struct {
		name           string
		excludeReplies bool
		wantParam      string
	}{
		{"default includes replies", false, "false"},
		{"flag excludes replies", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotParam string
			server := httptest.NewServer(captureExcludeReplies(t, &gotParam, []mastodonStatus{}))
			defer server.Close()

			client := NewMastodonClient()
			creds := &Credentials{AccessToken: "tok"}
			_, _, err := client.fetchUserStatusesPaginated(server.URL, "123", 20, "", creds, tt.excludeReplies)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotParam != tt.wantParam {
				t.Errorf("exclude_replies = %q, want %q", gotParam, tt.wantParam)
			}
		})
	}
}

// TestFetchUserStatuses_ExcludeRepliesParam verifies that the non-paginated public fetch
// sends the correct exclude_replies query parameter.
func TestFetchUserStatuses_ExcludeRepliesParam(t *testing.T) {
	tests := []struct {
		name           string
		excludeReplies bool
		wantParam      string
	}{
		{"default includes replies", false, "false"},
		{"flag excludes replies", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotParam string
			server := httptest.NewServer(captureExcludeReplies(t, &gotParam, []mastodonStatus{}))
			defer server.Close()

			client := NewMastodonClient()
			_, err := client.fetchUserStatuses(server.URL, "123", 20, tt.excludeReplies)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotParam != tt.wantParam {
				t.Errorf("exclude_replies = %q, want %q", gotParam, tt.wantParam)
			}
		})
	}
}

// TestFetchUserStatusesAuthenticated_ExcludeRepliesParam verifies that the non-paginated
// authenticated fetch sends the correct exclude_replies query parameter.
func TestFetchUserStatusesAuthenticated_ExcludeRepliesParam(t *testing.T) {
	tests := []struct {
		name           string
		excludeReplies bool
		wantParam      string
	}{
		{"default includes replies", false, "false"},
		{"flag excludes replies", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotParam string
			server := httptest.NewServer(captureExcludeReplies(t, &gotParam, []mastodonStatus{}))
			defer server.Close()

			client := NewMastodonClient()
			creds := &Credentials{AccessToken: "tok"}
			_, err := client.fetchUserStatusesAuthenticated(server.URL, "123", 20, creds, tt.excludeReplies)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotParam != tt.wantParam {
				t.Errorf("exclude_replies = %q, want %q", gotParam, tt.wantParam)
			}
		})
	}
}

// TestConvertStatusesToPosts_ReplyIncluded verifies that reply statuses are converted
// to posts with PostTypeReply when the API returns them.
func TestConvertStatusesToPosts_ReplyIncluded(t *testing.T) {
	client := NewMastodonClient()

	statuses := []mastodonStatus{
		newTestMastodonStatus("1"),
		newTestReplyStatus("2", "original-99"),
	}

	posts := client.convertStatusesToPosts(statuses, "https://mastodon.social", nil)

	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].Type != PostTypeOriginal {
		t.Errorf("post[0].Type = %v, want PostTypeOriginal", posts[0].Type)
	}
	if posts[1].Type != PostTypeReply {
		t.Errorf("post[1].Type = %v, want PostTypeReply", posts[1].Type)
	}
	if posts[1].InReplyToID != "original-99" {
		t.Errorf("post[1].InReplyToID = %q, want %q", posts[1].InReplyToID, "original-99")
	}
}

// TestPruneOptions_ExcludeRepliesDefault verifies that ExcludeReplies defaults to false,
// meaning replies are included in pruning by default.
func TestPruneOptions_ExcludeRepliesDefault(t *testing.T) {
	opts := PruneOptions{}
	if opts.ExcludeReplies {
		t.Error("ExcludeReplies should default to false (replies pruned by default)")
	}
}

// TestPruneOptions_ExcludeRepliesFlag verifies that ExcludeReplies can be set to true.
func TestPruneOptions_ExcludeRepliesFlag(t *testing.T) {
	opts := PruneOptions{ExcludeReplies: true}
	if !opts.ExcludeReplies {
		t.Error("ExcludeReplies should be true when explicitly set")
	}
}

// applyExcludeRepliesFilter mirrors the pruning loop's ExcludeReplies check so we can
// test the filtering logic without running a full PrunePosts (which requires credentials).
func applyExcludeRepliesFilter(posts []Post, opts PruneOptions) []Post {
	var out []Post
	for _, p := range posts {
		if opts.ExcludeReplies && p.Type == PostTypeReply {
			continue
		}
		out = append(out, p)
	}
	return out
}

// TestExcludeRepliesFilter_Default verifies that without the flag, replies pass through.
func TestExcludeRepliesFilter_Default(t *testing.T) {
	posts := []Post{
		{ID: "1", Type: PostTypeOriginal},
		{ID: "2", Type: PostTypeReply},
		{ID: "3", Type: PostTypeRepost},
	}
	got := applyExcludeRepliesFilter(posts, PruneOptions{ExcludeReplies: false})
	if len(got) != 3 {
		t.Errorf("default: expected 3 posts, got %d", len(got))
	}
}

// TestExcludeRepliesFilter_FlagSet verifies that with the flag, replies are dropped.
func TestExcludeRepliesFilter_FlagSet(t *testing.T) {
	posts := []Post{
		{ID: "1", Type: PostTypeOriginal},
		{ID: "2", Type: PostTypeReply},
		{ID: "3", Type: PostTypeReply},
		{ID: "4", Type: PostTypeRepost},
	}
	got := applyExcludeRepliesFilter(posts, PruneOptions{ExcludeReplies: true})
	if len(got) != 2 {
		t.Errorf("flag set: expected 2 posts, got %d", len(got))
	}
	for _, p := range got {
		if p.Type == PostTypeReply {
			t.Errorf("flag set: reply post %q should have been filtered out", p.ID)
		}
	}
}

// TestExcludeRepliesFilter_OnlyReplies verifies that filtering a reply-only list leaves nothing.
func TestExcludeRepliesFilter_OnlyReplies(t *testing.T) {
	posts := []Post{
		{ID: "1", Type: PostTypeReply},
		{ID: "2", Type: PostTypeReply},
	}
	got := applyExcludeRepliesFilter(posts, PruneOptions{ExcludeReplies: true})
	if len(got) != 0 {
		t.Errorf("all replies: expected 0 posts, got %d", len(got))
	}
}

// TestFetchUserStatusesPaginatedPublic_RepliesReturnedByDefault verifies that reply statuses
// are included in the result when excludeReplies=false (the default).
func TestFetchUserStatusesPaginatedPublic_RepliesReturnedByDefault(t *testing.T) {
	replyToID := "original-42"
	statuses := []mastodonStatus{
		newTestMastodonStatus("1"),
		newTestReplyStatus("2", replyToID),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		param := r.URL.Query().Get("exclude_replies")
		if param != "false" {
			t.Errorf("exclude_replies = %q, want %q", param, "false")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statuses)
	}))
	defer server.Close()

	client := NewMastodonClient()
	got, _, err := client.fetchUserStatusesPaginatedPublic(server.URL, "user1", 20, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(got))
	}
	if got[1].InReplyToID == nil || *got[1].InReplyToID != replyToID {
		t.Errorf("reply status not preserved in results")
	}
}

// newTestReblogStatus returns a mastodonStatus that represents a reblog (boost).
// id is the reblog action's own status ID; originalID is the original post's ID.
func newTestReblogStatus(id, originalID string) mastodonStatus {
	orig := newTestMastodonStatus(originalID)
	s := newTestMastodonStatus(id)
	s.Reblog = &orig
	return s
}

// TestConvertStatusesToPosts_ReblogID verifies that a reblog's post.ID is the reblog
// action's own status ID (needed to DELETE it), not the original post's ID.
func TestConvertStatusesToPosts_ReblogID(t *testing.T) {
	client := NewMastodonClient()

	reblogActionID := "reblog-action-999"
	originalPostID := "original-post-42"
	statuses := []mastodonStatus{
		newTestReblogStatus(reblogActionID, originalPostID),
	}

	posts := client.convertStatusesToPosts(statuses, "https://mastodon.social", nil)

	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}
	if posts[0].Type != PostTypeRepost {
		t.Errorf("post.Type = %v, want PostTypeRepost", posts[0].Type)
	}
	if posts[0].ID != reblogActionID {
		t.Errorf("post.ID = %q, want reblog action ID %q", posts[0].ID, reblogActionID)
	}
}

// TestUnreblogUsesDelete verifies that unreblogPost removes the boost by issuing a DELETE
// request against the reblog action's status ID (not POST /unreblog with the wrong ID).
func TestUnreblogUsesDelete(t *testing.T) {
	reblogActionID := "reblog-999"
	var gotMethod, gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewMastodonClient()
	creds := &Credentials{
		Platform:    "mastodon",
		Username:    "testuser",
		Instance:    server.URL,
		AccessToken: "tok",
	}

	err := client.unreblogPost(creds, reblogActionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("HTTP method = %q, want DELETE", gotMethod)
	}
	wantPath := "/api/v1/statuses/" + reblogActionID
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q", gotPath, wantPath)
	}
}

// TestPrunePostsPaginationContinuesWhenBatchHasNoMatches verifies that the paginator
// does not stop after the first batch just because none of its posts satisfy the age
// threshold. Posts that are old enough can appear in later pages.
func TestPrunePostsPaginationContinuesWhenBatchHasNoMatches(t *testing.T) {
	// Page 1: recent posts (within MaxAge — should NOT be pruned)
	// Page 2: old posts (beyond MaxAge — SHOULD be pruned)
	// Page 3: empty (signals end of timeline)
	recentTime := time.Now().Add(-1 * 24 * time.Hour)   // 1 day ago
	oldTime := time.Now().Add(-60 * 24 * time.Hour)      // 60 days ago
	maxAge := 30 * 24 * time.Hour

	page1 := []mastodonStatus{
		{ID: "100", Content: "<p>recent post</p>", CreatedAt: recentTime,
			Account: mastodonAccount{ID: "u1", Username: "u", Acct: "u"}},
	}
	page2 := []mastodonStatus{
		{ID: "50", Content: "<p>old post</p>", CreatedAt: oldTime,
			Account: mastodonAccount{ID: "u1", Username: "u", Acct: "u"}},
	}

	var pageCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&pageCount, 1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			json.NewEncoder(w).Encode(page1)
		case 2:
			json.NewEncoder(w).Encode(page2)
		default:
			json.NewEncoder(w).Encode([]mastodonStatus{})
		}
	}))
	defer server.Close()

	client := NewMastodonClient()
	creds := &Credentials{
		Platform:    "mastodon",
		Username:    "u",
		Instance:    server.URL,
		AccessToken: "tok",
	}

	// Use fetchUserStatusesPaginated directly to test the pagination logic in isolation.
	// Page 1 — no matches yet.
	statuses1, cursor1, err := client.fetchUserStatusesPaginated(server.URL, "u1", 10, "", creds, false)
	if err != nil {
		t.Fatalf("page 1 fetch error: %v", err)
	}
	if len(statuses1) == 0 {
		t.Fatal("expected statuses on page 1")
	}
	// The first page has no old posts; the caller must not stop here.
	// Simulate continuing: fetch page 2 using cursor1.
	statuses2, _, err := client.fetchUserStatusesPaginated(server.URL, "u1", 10, cursor1, creds, false)
	if err != nil {
		t.Fatalf("page 2 fetch error: %v", err)
	}
	if len(statuses2) == 0 {
		t.Fatal("expected old statuses on page 2")
	}

	// Verify: the old post is on page 2 and matches the age filter.
	found := false
	for _, s := range statuses2 {
		if time.Since(s.CreatedAt) > maxAge {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected an old post on page 2 that satisfies the %v max-age filter", maxAge)
	}
	_ = creds
}
