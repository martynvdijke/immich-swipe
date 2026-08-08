package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// newUpstream returns an httptest server that echoes the request body and
// returns the given status code. It records whether it was ever hit.
func newUpstream(status int) (*httptest.Server, *sync.Mutex, *[]byte) {
	var mu sync.Mutex
	gotBody := new([]byte)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		body, _ := io.ReadAll(r.Body)
		*gotBody = body
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	return upstream, &mu, gotBody
}

func upstreamBody(mu *sync.Mutex, gotBody *[]byte) []byte {
	mu.Lock()
	defer mu.Unlock()
	return *gotBody
}

// doProxied sends an authenticated request through the full ServeHTTP path.
func doProxied(t *testing.T, srv *Server, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func snapshotFor(t *testing.T, srv *Server) (kept, deleted int, users []StatsEntry) {
	t.Helper()
	kept, deleted, users, _ = srv.stats.Snapshot()
	return
}

// ─── Proxy counting (tasks 2.1-2.3) ────────────────────────────────────────

func TestCounting_DeleteIncrementsDeleted(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	rr := doProxied(t, srv, token, http.MethodDelete, "/api/assets", `{"ids":["a","b","c"],"force":false}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	_, deleted, _ := snapshotFor(t, srv)
	if deleted != 3 {
		t.Fatalf("expected deleted=3, got %d", deleted)
	}
}

func TestCounting_RestoreDecrementsDeleted(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	doProxied(t, srv, token, http.MethodDelete, "/api/assets", `{"ids":["a","b"]}`)
	doProxied(t, srv, token, http.MethodPost, "/api/trash/restore/assets", `{"ids":["b"]}`)

	_, deleted, _ := snapshotFor(t, srv)
	if deleted != 1 {
		t.Fatalf("expected deleted=1 after delete+restore, got %d", deleted)
	}
}

func TestCounting_AlbumAddIncrementsKept(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	rr := doProxied(t, srv, token, http.MethodPut, "/api/albums/alb123/assets", `{"ids":["x","y"]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	kept, _, _ := snapshotFor(t, srv)
	if kept != 2 {
		t.Fatalf("expected kept=2, got %d", kept)
	}
}

func TestCounting_FavoriteTrueIncrementsKept(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	doProxied(t, srv, token, http.MethodPut, "/api/assets/asset-1", `{"isFavorite":true}`)
	doProxied(t, srv, token, http.MethodPut, "/api/assets/asset-2", `{"isFavorite":true}`)

	kept, _, _ := snapshotFor(t, srv)
	if kept != 2 {
		t.Fatalf("expected kept=2 from favorites, got %d", kept)
	}
}

func TestCounting_FavoriteFalseIgnored(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	doProxied(t, srv, token, http.MethodPut, "/api/assets/asset-1", `{"isFavorite":false}`)

	kept, deleted, users := snapshotFor(t, srv)
	if kept != 0 || deleted != 0 || len(users) != 0 {
		t.Fatalf("expected no counters for isFavorite:false, got kept=%d deleted=%d users=%d", kept, deleted, len(users))
	}
}

func TestCounting_ForceTrueIgnored(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	// Hard delete with force:true must not count as a swipe trash.
	doProxied(t, srv, token, http.MethodDelete, "/api/assets", `{"ids":["a","b"],"force":true}`)

	_, deleted, users := snapshotFor(t, srv)
	if deleted != 0 || len(users) != 0 {
		t.Fatalf("expected force:true delete to be ignored, got deleted=%d users=%d", deleted, len(users))
	}
}

func TestCounting_MalformedBodyIgnored(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	doProxied(t, srv, token, http.MethodDelete, "/api/assets", `{not valid json`)

	_, deleted, users := snapshotFor(t, srv)
	if deleted != 0 || len(users) != 0 {
		t.Fatalf("expected malformed body to be ignored, got deleted=%d users=%d", deleted, len(users))
	}
}

func TestCounting_NonCountableRequestUntouched(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	rr := doProxied(t, srv, token, http.MethodGet, "/api/users/me", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	kept, deleted, users := snapshotFor(t, srv)
	if kept != 0 || deleted != 0 || len(users) != 0 {
		t.Fatalf("expected non-countable request to be untouched, got kept=%d deleted=%d users=%d", kept, deleted, len(users))
	}
}

func TestCounting_FailedUpstreamDoesNotCount(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusInternalServerError)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	rr := doProxied(t, srv, token, http.MethodDelete, "/api/assets", `{"ids":["a","b"]}`)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected upstream 500 to propagate, got %d", rr.Code)
	}
	_, deleted, users := snapshotFor(t, srv)
	if deleted != 0 || len(users) != 0 {
		t.Fatalf("failed upstream must not change counters, got deleted=%d users=%d", deleted, len(users))
	}
}

func TestCounting_NotDoubleCounted(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	// Multiple hits on the same request must never double-count.
	doProxied(t, srv, token, http.MethodDelete, "/api/assets", `{"ids":["a","b","c"]}`)
	doProxied(t, srv, token, http.MethodDelete, "/api/assets", `{"ids":["a","b","c"]}`)

	_, deleted, _ := snapshotFor(t, srv)
	if deleted != 6 {
		t.Fatalf("expected deleted=6 across two requests, got %d", deleted)
	}
}

func TestCounting_UpstreamBodyByteIdentical(t *testing.T) {
	upstream, mu, gotBody := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "key", upstream.URL)

	sent := `{"ids":["a","b"],"force":false}`
	rr := doProxied(t, srv, token, http.MethodDelete, "/api/assets", sent)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	if got := string(upstreamBody(mu, gotBody)); got != sent {
		t.Fatalf("upstream body mismatch:\n sent:    %q\n received: %q", sent, got)
	}
}

// ─── Public stats endpoint (task 3.x) ──────────────────────────────────────

func TestStatsEndpoint_AggregatesAcrossUsers(t *testing.T) {
	upstream, _, _ := newUpstream(http.StatusOK)
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	tokenA := srv.session.CreateAPIKey("Alice", "key", upstream.URL)
	tokenB := srv.session.CreateAPIKey("Bob", "key", upstream.URL)

	doProxied(t, srv, tokenA, http.MethodDelete, "/api/assets", `{"ids":["a","b"]}`)
	doProxied(t, srv, tokenA, http.MethodPut, "/api/albums/al/assets", `{"ids":["k1"]}`)
	doProxied(t, srv, tokenB, http.MethodDelete, "/api/assets", `{"ids":["c"]}`)

	// Public endpoint: no Authorization header required.
	req := httptest.NewRequest(http.MethodGet, "/api/trmnl/stats", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		KeptCount    int    `json:"keptCount"`
		DeletedCount int    `json:"deletedCount"`
		TotalCount   int    `json:"totalCount"`
		ServerURL    string `json:"serverUrl"`
		UpdatedAt    string `json:"updatedAt"`
		Users        []struct {
			UserName     string `json:"userName"`
			KeptCount    int    `json:"keptCount"`
			DeletedCount int    `json:"deletedCount"`
		} `json:"users"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.KeptCount != 1 || resp.DeletedCount != 3 || resp.TotalCount != 4 {
		t.Fatalf("unexpected totals: kept=%d deleted=%d total=%d", resp.KeptCount, resp.DeletedCount, resp.TotalCount)
	}
	if resp.ServerURL != upstream.URL {
		t.Fatalf("expected serverUrl %q, got %q", upstream.URL, resp.ServerURL)
	}
	if resp.UpdatedAt == "" {
		t.Fatal("expected non-empty updatedAt")
	}
	if len(resp.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp.Users))
	}
}

func TestStatsEndpoint_EmptyStore(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich.example"})

	req := httptest.NewRequest(http.MethodGet, "/api/trmnl/stats", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		KeptCount    int    `json:"keptCount"`
		DeletedCount int    `json:"deletedCount"`
		TotalCount   int    `json:"totalCount"`
		UpdatedAt    string `json:"updatedAt"`
		Users        []any  `json:"users"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.KeptCount != 0 || resp.DeletedCount != 0 || resp.TotalCount != 0 {
		t.Fatalf("expected zero counters for empty store, got %+v", resp)
	}
	if len(resp.Users) != 0 {
		t.Fatalf("expected empty users array, got %d", len(resp.Users))
	}
	if resp.UpdatedAt == "" {
		t.Fatal("expected non-empty updatedAt for empty store")
	}
}

func TestStatsEndpoint_MethodNotAllowed(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich.example"})

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/trmnl/stats", nil)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestStatsEndpoint_NotProxied(t *testing.T) {
	// The upstream must never be hit for /api/trmnl/stats: the route is
	// served locally before the /api/ proxy catch-all.
	hit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})

	req := httptest.NewRequest(http.MethodGet, "/api/trmnl/stats", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if hit {
		t.Fatal("/api/trmnl/stats must be served locally, not proxied to upstream")
	}
}
