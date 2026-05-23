package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewBlueskyClient(t *testing.T) {
	client := NewBlueskyClient()
	if client == nil {
		t.Fatal("NewBlueskyClient should return a non-nil client")
	}
	if client.sessionManager == nil {
		t.Error("sessionManager should be initialised")
	}
}

func TestBlueskyClient_GetPlatformName(t *testing.T) {
	if name := NewBlueskyClient().GetPlatformName(); name != "Bluesky" {
		t.Errorf("expected %q, got %q", "Bluesky", name)
	}
}

func TestBlueskyClient_RequiresAuth(t *testing.T) {
	if !NewBlueskyClient().RequiresAuth() {
		t.Error("Bluesky client should require authentication")
	}
}

// ---------------------------------------------------------------------------
// extractPostID
// ---------------------------------------------------------------------------

func TestExtractPostID(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"at://did:plc:abc123/app.bsky.feed.post/xyz789", "xyz789"},
		{"at://did:plc:x/app.bsky.feed.post/verylongid", "verylongid"},
		{"", ""},
		{"invalid-uri-format", ""},
		{"at://did:plc:abc123/app.bsky.feed.post/", ""},
	}
	for _, tt := range tests {
		if got := extractPostID(tt.uri); got != tt.expected {
			t.Errorf("extractPostID(%q) = %q, want %q", tt.uri, got, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// determinePostType
// ---------------------------------------------------------------------------

func TestBlueskyClient_DeterminePostType(t *testing.T) {
	c := NewBlueskyClient()
	tests := []struct {
		name     string
		post     blueskyPost
		expected PostType
	}{
		{
			"original post",
			blueskyPost{Record: blueskyRecord{Type: "app.bsky.feed.post"}},
			PostTypeOriginal,
		},
		{
			"repost",
			blueskyPost{Record: blueskyRecord{Type: "app.bsky.feed.repost"}},
			PostTypeRepost,
		},
		{
			"reply",
			blueskyPost{Record: blueskyRecord{
				Type:  "app.bsky.feed.post",
				Reply: &blueskyReply{Parent: blueskyPostRef{URI: "at://parent"}},
			}},
			PostTypeReply,
		},
		{
			"unknown type defaults to original",
			blueskyPost{Record: blueskyRecord{Type: "app.bsky.unknown"}},
			PostTypeOriginal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.determinePostType(tt.post); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// convertBskyPost — the main new abstraction
// ---------------------------------------------------------------------------

func TestBlueskyClient_ConvertBskyPost(t *testing.T) {
	c := NewBlueskyClient()
	now := time.Now().Truncate(time.Second)

	t.Run("original post fields", func(t *testing.T) {
		p := blueskyPost{
			URI: "at://did:plc:abc/app.bsky.feed.post/rkey1",
			Author: blueskyAuthor{
				DID:         "did:plc:abc",
				Handle:      "user.bsky.social",
				DisplayName: "Test User",
			},
			Record: blueskyRecord{
				Type:      "app.bsky.feed.post",
				Text:      "hello",
				CreatedAt: now,
			},
			LikeCount:   3,
			RepostCount: 1,
			ReplyCount:  2,
		}
		got := c.convertBskyPost(p)
		if got.ID != p.URI {
			t.Errorf("ID: want %q, got %q", p.URI, got.ID)
		}
		if got.Author != "Test User" {
			t.Errorf("Author: want %q, got %q", "Test User", got.Author)
		}
		if got.Handle != "user.bsky.social" {
			t.Errorf("Handle: want %q, got %q", "user.bsky.social", got.Handle)
		}
		if got.Content != "hello" {
			t.Errorf("Content: want %q, got %q", "hello", got.Content)
		}
		if got.Platform != "bluesky" {
			t.Errorf("Platform: want %q, got %q", "bluesky", got.Platform)
		}
		if got.LikeCount != 3 || got.RepostCount != 1 || got.ReplyCount != 2 {
			t.Errorf("metrics wrong: likes=%d reposts=%d replies=%d", got.LikeCount, got.RepostCount, got.ReplyCount)
		}
		if got.Type != PostTypeOriginal {
			t.Errorf("Type: want %v, got %v", PostTypeOriginal, got.Type)
		}
		wantURL := "https://bsky.app/profile/user.bsky.social/post/rkey1"
		if got.URL != wantURL {
			t.Errorf("URL: want %q, got %q", wantURL, got.URL)
		}
	})

	t.Run("empty DisplayName falls back to Handle", func(t *testing.T) {
		p := blueskyPost{
			URI:    "at://did:plc:x/app.bsky.feed.post/r",
			Author: blueskyAuthor{Handle: "handle.bsky.social"},
			Record: blueskyRecord{Type: "app.bsky.feed.post", CreatedAt: now},
		}
		got := c.convertBskyPost(p)
		if got.Author != "handle.bsky.social" {
			t.Errorf("want handle fallback, got %q", got.Author)
		}
	})

	t.Run("viewer data liked", func(t *testing.T) {
		likeURI := "at://like"
		p := blueskyPost{
			URI:        "at://did:plc:x/app.bsky.feed.post/r",
			Author:     blueskyAuthor{Handle: "h"},
			Record:     blueskyRecord{Type: "app.bsky.feed.post", CreatedAt: now},
			ViewerData: &blueskyViewerData{Like: &likeURI},
		}
		if !c.convertBskyPost(p).IsLikedByUser {
			t.Error("expected IsLikedByUser=true")
		}
	})

	t.Run("viewer data not liked", func(t *testing.T) {
		p := blueskyPost{
			URI:        "at://did:plc:x/app.bsky.feed.post/r",
			Author:     blueskyAuthor{Handle: "h"},
			Record:     blueskyRecord{Type: "app.bsky.feed.post", CreatedAt: now},
			ViewerData: &blueskyViewerData{Like: nil},
		}
		if c.convertBskyPost(p).IsLikedByUser {
			t.Error("expected IsLikedByUser=false")
		}
	})

	t.Run("nil viewer data", func(t *testing.T) {
		p := blueskyPost{
			URI:    "at://did:plc:x/app.bsky.feed.post/r",
			Author: blueskyAuthor{Handle: "h"},
			Record: blueskyRecord{Type: "app.bsky.feed.post", CreatedAt: now},
		}
		if c.convertBskyPost(p).IsLikedByUser {
			t.Error("expected IsLikedByUser=false when ViewerData is nil")
		}
	})

	t.Run("pinned post", func(t *testing.T) {
		p := blueskyPost{
			URI:      "at://did:plc:x/app.bsky.feed.post/r",
			Author:   blueskyAuthor{Handle: "h"},
			Record:   blueskyRecord{Type: "app.bsky.feed.post", CreatedAt: now},
			IsPinned: true,
		}
		if !c.convertBskyPost(p).IsPinned {
			t.Error("expected IsPinned=true")
		}
	})

	t.Run("reply sets InReplyToID", func(t *testing.T) {
		p := blueskyPost{
			URI:    "at://did:plc:x/app.bsky.feed.post/r",
			Author: blueskyAuthor{Handle: "h"},
			Record: blueskyRecord{
				Type:      "app.bsky.feed.post",
				CreatedAt: now,
				Reply:     &blueskyReply{Parent: blueskyPostRef{URI: "at://parent/uri"}},
			},
		}
		got := c.convertBskyPost(p)
		if got.Type != PostTypeReply {
			t.Errorf("Type: want reply, got %v", got.Type)
		}
		if got.InReplyToID != "at://parent/uri" {
			t.Errorf("InReplyToID: want %q, got %q", "at://parent/uri", got.InReplyToID)
		}
	})

	t.Run("repost type", func(t *testing.T) {
		p := blueskyPost{
			URI:    "at://did:plc:x/app.bsky.feed.repost/r",
			Author: blueskyAuthor{Handle: "h"},
			Record: blueskyRecord{Type: "app.bsky.feed.repost", CreatedAt: now},
		}
		if c.convertBskyPost(p).Type != PostTypeRepost {
			t.Error("expected PostTypeRepost")
		}
	})
}

// ---------------------------------------------------------------------------
// validatePostURI
// ---------------------------------------------------------------------------

func TestBlueskyClient_ValidatePostURI(t *testing.T) {
	c := NewBlueskyClient()
	tests := []struct {
		name    string
		uri     string
		did     string
		wantErr bool
	}{
		{"valid", "at://did:plc:abc/app.bsky.feed.post/rkey", "did:plc:abc", false},
		{"DID mismatch", "at://did:plc:abc/app.bsky.feed.post/rkey", "did:plc:other", true},
		{"too few parts", "at://short", "did:plc:abc", true},
		{"empty URI", "", "did:plc:abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.validatePostURI(tt.uri, tt.did)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseJWTExpiration
// ---------------------------------------------------------------------------

func TestBlueskyClient_ParseJWTExpiration(t *testing.T) {
	c := NewBlueskyClient()

	t.Run("valid JWT with future expiry", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour).Unix()
		payload := fmt.Sprintf(`{"exp":%d}`, future)
		encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
		token := "header." + encoded + ".sig"
		exp, err := c.parseJWTExpiration(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exp.Unix() != future {
			t.Errorf("exp mismatch: want %d, got %d", future, exp.Unix())
		}
	})

	t.Run("malformed JWT returns error", func(t *testing.T) {
		if _, err := c.parseJWTExpiration("not-a-jwt"); err == nil {
			t.Error("expected error for malformed JWT")
		}
	})

	t.Run("JWT with non-JSON payload returns error", func(t *testing.T) {
		encoded := base64.RawURLEncoding.EncodeToString([]byte("notjson"))
		if _, err := c.parseJWTExpiration("h." + encoded + ".s"); err == nil {
			t.Error("expected error for non-JSON payload")
		}
	})
}

// ---------------------------------------------------------------------------
// FetchUserPosts / FetchUserPostsPaginated (via httptest)
// ---------------------------------------------------------------------------

func makeFeedServer(t *testing.T, posts []blueskyPost) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "getAuthorFeed") {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		feed := blueskyEnhancedFeedResponse{}
		for _, p := range posts {
			entry := struct {
				Post       blueskyPost        `json:"post"`
				ViewerData *blueskyViewerData `json:"viewer,omitempty"`
				PinnedPost bool               `json:"pinnedPost,omitempty"`
			}{Post: p}
			feed.Feed = append(feed.Feed, entry)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(feed)
	}))
}

func TestBlueskyClient_FetchUserPosts(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	srv := makeFeedServer(t, []blueskyPost{
		{
			URI:    "at://did:plc:u/app.bsky.feed.post/r1",
			Author: blueskyAuthor{Handle: "u.bsky.social", DisplayName: "U"},
			Record: blueskyRecord{Type: "app.bsky.feed.post", Text: "hi", CreatedAt: now},
		},
	})
	defer srv.Close()

	c := NewBlueskyClient()
	c.publicBaseURL = srv.URL

	posts, err := c.FetchUserPosts("u.bsky.social", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) == 0 {
		t.Fatal("expected at least one post")
	}
	if posts[0].Content != "hi" {
		t.Errorf("Content: want %q, got %q", "hi", posts[0].Content)
	}
	if posts[0].Platform != "bluesky" {
		t.Errorf("Platform: want bluesky, got %q", posts[0].Platform)
	}
}

func TestBlueskyClient_FetchUserPostsPaginated_Cursor(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	cursor := "next-page-cursor"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		feed := blueskyEnhancedFeedResponse{
			Cursor: &cursor,
		}
		entry := struct {
			Post       blueskyPost        `json:"post"`
			ViewerData *blueskyViewerData `json:"viewer,omitempty"`
			PinnedPost bool               `json:"pinnedPost,omitempty"`
		}{Post: blueskyPost{
			URI:    "at://did:plc:u/app.bsky.feed.post/r1",
			Author: blueskyAuthor{Handle: "u"},
			Record: blueskyRecord{Type: "app.bsky.feed.post", Text: "paginated", CreatedAt: now},
		}}
		feed.Feed = append(feed.Feed, entry)
		json.NewEncoder(w).Encode(feed)
	}))
	defer srv.Close()

	c := NewBlueskyClient()
	c.publicBaseURL = srv.URL

	posts, nextCursor, err := c.FetchUserPostsPaginated("u", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) == 0 {
		t.Fatal("expected posts")
	}
	if nextCursor != cursor {
		t.Errorf("nextCursor: want %q, got %q", cursor, nextCursor)
	}
}

func TestBlueskyClient_FetchUserPosts_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewBlueskyClient()
	c.publicBaseURL = srv.URL

	_, err := c.FetchUserPosts("u", 10)
	if err == nil {
		t.Error("expected error on 500 response")
	}
}

// ---------------------------------------------------------------------------
// deleteAtpRecord (via httptest)
// ---------------------------------------------------------------------------

func makeDeleteServer(t *testing.T, wantDID string) (*httptest.Server, *bool) {
	t.Helper()
	called := new(bool)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "deleteRecord") {
			*called = true
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["repo"] != wantDID {
				http.Error(w, "wrong repo", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.Contains(r.URL.Path, "createSession") {
			json.NewEncoder(w).Encode(atpSessionResponse{
				AccessJwt: "tok", RefreshJwt: "ref", Handle: "h", DID: wantDID,
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	return srv, called
}

func TestBlueskyClient_DeleteAtpRecord(t *testing.T) {
	const did = "did:plc:testuser"
	srv, called := makeDeleteServer(t, did)
	defer srv.Close()

	c := NewBlueskyClient()
	c.atpBaseURL = srv.URL
	// Pre-populate a session so ensureValidSession reuses it
	c.session = &atpSessionResponse{AccessJwt: "tok", RefreshJwt: "ref", Handle: "h", DID: did}
	c.sessionManager.UpdateSession("tok", "ref", time.Now().Add(1*time.Hour), &Credentials{Username: "u", AppPassword: "p"})

	uri := fmt.Sprintf("at://%s/app.bsky.feed.post/rkey1", did)
	creds := &Credentials{Username: "u", AppPassword: "p"}

	if err := c.deleteAtpRecord(creds, uri); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*called {
		t.Error("deleteRecord endpoint was not called")
	}
}

func TestBlueskyClient_DeleteAtpRecord_DIDMismatch(t *testing.T) {
	const did = "did:plc:testuser"
	srv, _ := makeDeleteServer(t, did)
	defer srv.Close()

	c := NewBlueskyClient()
	c.atpBaseURL = srv.URL
	c.session = &atpSessionResponse{AccessJwt: "tok", RefreshJwt: "ref", Handle: "h", DID: did}
	c.sessionManager.UpdateSession("tok", "ref", time.Now().Add(1*time.Hour), &Credentials{Username: "u", AppPassword: "p"})

	uri := "at://did:plc:DIFFERENT/app.bsky.feed.post/rkey1"
	if err := c.deleteAtpRecord(&Credentials{Username: "u", AppPassword: "p"}, uri); err == nil {
		t.Error("expected DID mismatch error")
	}
}

func TestBlueskyClient_DeleteAtpRecord_InvalidURI(t *testing.T) {
	c := NewBlueskyClient()
	c.session = &atpSessionResponse{DID: "did:plc:x"}
	c.sessionManager.UpdateSession("tok", "ref", time.Now().Add(1*time.Hour), &Credentials{Username: "u", AppPassword: "p"})

	if err := c.deleteAtpRecord(&Credentials{Username: "u", AppPassword: "p"}, "at://short"); err == nil {
		t.Error("expected error for short URI")
	}
}

// ---------------------------------------------------------------------------
// fetchAllATPRecords (via httptest) — tests pagination, age cutoff, cursor dedup
// ---------------------------------------------------------------------------

type atpRecord struct {
	URI   string `json:"uri"`
	Value struct {
		Subject   struct{ URI string `json:"uri"` } `json:"subject"`
		CreatedAt time.Time                         `json:"createdAt"`
	} `json:"value"`
}

func makeListRecordsServer(t *testing.T, pages [][]atpRecord) *httptest.Server {
	t.Helper()
	call := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "listRecords") {
			http.Error(w, "not found", 404)
			return
		}
		type response struct {
			Records []atpRecord `json:"records"`
			Cursor  string      `json:"cursor,omitempty"`
		}
		var resp response
		if call < len(pages) {
			resp.Records = pages[call]
			if call+1 < len(pages) {
				resp.Cursor = fmt.Sprintf("cursor-%d", call+1)
			}
		}
		call++
		json.NewEncoder(w).Encode(resp)
	}))
}

func makeSession(did string) *atpSessionResponse {
	return &atpSessionResponse{AccessJwt: "tok", RefreshJwt: "ref", Handle: "h", DID: did}
}

func TestFetchAllATPRecords_SinglePage(t *testing.T) {
	now := time.Now()
	old := now.Add(-48 * time.Hour)
	maxAge := 24 * time.Hour

	rec := atpRecord{URI: "at://did:plc:x/app.bsky.feed.like/r1"}
	rec.Value.Subject.URI = "at://original"
	rec.Value.CreatedAt = old

	srv := makeListRecordsServer(t, [][]atpRecord{{rec}})
	defer srv.Close()

	c := NewBlueskyClient()
	c.atpBaseURL = srv.URL
	session := makeSession("did:plc:x")

	opts := PruneOptions{MaxAge: &maxAge}
	posts, err := c.fetchAllATPRecords(session, opts, "app.bsky.feed.like", PostTypeLike, "Liked: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("want 1 post, got %d", len(posts))
	}
	if posts[0].Type != PostTypeLike {
		t.Errorf("Type: want %v, got %v", PostTypeLike, posts[0].Type)
	}
	if posts[0].Content != "Liked: at://original" {
		t.Errorf("Content: %q", posts[0].Content)
	}
}

func TestFetchAllATPRecords_MultiPage(t *testing.T) {
	now := time.Now()
	old := now.Add(-48 * time.Hour)
	maxAge := 24 * time.Hour

	makeRec := func(rkey string) atpRecord {
		r := atpRecord{URI: "at://did:plc:x/app.bsky.feed.repost/" + rkey}
		r.Value.Subject.URI = "at://orig"
		r.Value.CreatedAt = old
		return r
	}
	page1 := []atpRecord{makeRec("r1"), makeRec("r2")}
	page2 := []atpRecord{makeRec("r3")}

	srv := makeListRecordsServer(t, [][]atpRecord{page1, page2})
	defer srv.Close()

	c := NewBlueskyClient()
	c.atpBaseURL = srv.URL

	opts := PruneOptions{MaxAge: &maxAge}
	posts, err := c.fetchAllATPRecords(makeSession("did:plc:x"), opts, "app.bsky.feed.repost", PostTypeRepost, "Reposted: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 3 {
		t.Errorf("want 3 posts across 2 pages, got %d", len(posts))
	}
}

func TestFetchAllATPRecords_StopsWhenNoAgeCriteriaMatch(t *testing.T) {
	// Records that are newer than MaxAge should not trigger continuation.
	maxAge := 24 * time.Hour
	recent := time.Now().Add(-1 * time.Hour) // within maxAge — should NOT trigger continue

	rec := atpRecord{URI: "at://did:plc:x/app.bsky.feed.like/r1"}
	rec.Value.Subject.URI = "at://orig"
	rec.Value.CreatedAt = recent

	srv := makeListRecordsServer(t, [][]atpRecord{{rec}})
	defer srv.Close()

	c := NewBlueskyClient()
	c.atpBaseURL = srv.URL

	opts := PruneOptions{MaxAge: &maxAge}
	posts, err := c.fetchAllATPRecords(makeSession("did:plc:x"), opts, "app.bsky.feed.like", PostTypeLike, "Liked: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Record is collected even though it doesn't match age criteria; pagination stops
	if len(posts) != 1 {
		t.Errorf("want 1 post, got %d", len(posts))
	}
}

func TestFetchAllATPRecords_EmptyFirstPage(t *testing.T) {
	srv := makeListRecordsServer(t, [][]atpRecord{{}})
	defer srv.Close()

	c := NewBlueskyClient()
	c.atpBaseURL = srv.URL

	maxAge := 24 * time.Hour
	posts, err := c.fetchAllATPRecords(makeSession("did:plc:x"), PruneOptions{MaxAge: &maxAge}, "app.bsky.feed.like", PostTypeLike, "Liked: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("want 0 posts, got %d", len(posts))
	}
}

func TestFetchAllATPRecords_CursorDedupBreaksLoop(t *testing.T) {
	// Server always returns the same cursor — must not loop forever.
	const staleCursor = "stuck"
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call > 5 {
			t.Error("pagination did not stop on duplicate cursor")
		}
		old := time.Now().Add(-48 * time.Hour)
		maxAge := 24 * time.Hour
		_ = maxAge
		rec := atpRecord{URI: fmt.Sprintf("at://did:plc:x/app.bsky.feed.like/r%d", call)}
		rec.Value.Subject.URI = "at://orig"
		rec.Value.CreatedAt = old
		type resp struct {
			Records []atpRecord `json:"records"`
			Cursor  string      `json:"cursor"`
		}
		json.NewEncoder(w).Encode(resp{Records: []atpRecord{rec}, Cursor: staleCursor})
	}))
	defer srv.Close()

	c := NewBlueskyClient()
	c.atpBaseURL = srv.URL

	maxAge := 24 * time.Hour
	_, err := c.fetchAllATPRecords(makeSession("did:plc:x"), PruneOptions{MaxAge: &maxAge}, "app.bsky.feed.like", PostTypeLike, "Liked: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PrunePosts — credential path
// ---------------------------------------------------------------------------

func TestBlueskyClient_PrunePosts_NoCredentials(t *testing.T) {
	result, err := NewBlueskyClient().PrunePosts("test.bsky.social", PruneOptions{
		MaxAge: func() *time.Duration { d := 30 * 24 * time.Hour; return &d }(),
		DryRun: true,
	})
	if err == nil {
		t.Error("expected error when no credentials available")
	}
	if result != nil {
		t.Error("expected nil result when credentials missing")
	}
}
