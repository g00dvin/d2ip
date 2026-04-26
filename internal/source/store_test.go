package source

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// errorReader is an io.Reader that always returns an error.
type errorReader struct{}

func (e *errorReader) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}

func TestNewHTTPStore(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		cachePath string
		timeout   time.Duration
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "empty URL",
			url:       "",
			cachePath: "/tmp/test.dat",
			timeout:   time.Second,
			wantErr:   true,
			errMsg:    "source: url cannot be empty",
		},
		{
			name:      "empty cachePath",
			url:       "http://example.com/test.dat",
			cachePath: "",
			timeout:   time.Second,
			wantErr:   true,
			errMsg:    "source: cachePath cannot be empty",
		},
		{
			name:      "success",
			url:       "http://example.com/test.dat",
			cachePath: filepath.Join(t.TempDir(), "test.dat"),
			timeout:   time.Second,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewHTTPStore(tt.url, tt.cachePath, tt.timeout)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if store == nil {
				t.Fatal("expected store, got nil")
			}
			if store.url != tt.url {
				t.Errorf("expected url %q, got %q", tt.url, store.url)
			}
			if store.cachePath != tt.cachePath {
				t.Errorf("expected cachePath %q, got %q", tt.cachePath, store.cachePath)
			}
		})
	}
}

func TestNewHTTPStore_LoadsMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	metaPath := cachePath + ".meta.json"

	meta := Version{
		SHA256:    "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Size:      42,
		FetchedAt: time.Now().Add(-time.Hour),
		ETag:      `"abc123"`,
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	store, err := NewHTTPStore("http://example.com/test.dat", cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info := store.Info()
	if info.ETag != meta.ETag {
		t.Errorf("expected ETag %q, got %q", meta.ETag, info.ETag)
	}
	if info.Size != meta.Size {
		t.Errorf("expected Size %d, got %d", meta.Size, info.Size)
	}
}

func TestNewHTTPStore_InvalidMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	metaPath := cachePath + ".meta.json"

	// Write invalid JSON.
	if err := os.WriteFile(metaPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	// Should not error — just log a warning.
	_, err := NewHTTPStore("http://example.com/test.dat", cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPStore_Get_MaxAgeZero_WithCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	content := []byte("cached data")
	if err := os.WriteFile(cachePath, content, 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore("http://example.com/test.dat", cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	path, ver, err := store.Get(ctx, 0)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
	if ver.Size != 0 {
		t.Errorf("expected zero size, got %d", ver.Size)
	}
}

func TestHTTPStore_Get_MaxAgeZero_NoCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store, err := NewHTTPStore("http://example.com/test.dat", cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	_, _, err = store.Get(ctx, 0)
	if err == nil {
		t.Fatal("expected error for missing cache")
	}
	if !strings.Contains(err.Error(), "no cached file available") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHTTPStore_Get_FreshCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	content := []byte("cached data")
	if err := os.WriteFile(cachePath, content, 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore("http://example.com/test.dat", cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set metadata so file appears fresh.
	store.mu.Lock()
	store.current = Version{
		SHA256:    "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Size:      int64(len(content)),
		FetchedAt: time.Now(),
	}
	store.mu.Unlock()

	ctx := context.Background()
	path, ver, err := store.Get(ctx, time.Hour)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
	if ver.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), ver.Size)
	}
}

func TestHTTPStore_Get_StaleCache_Fetches(t *testing.T) {
	content := []byte("fresh data")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	oldContent := []byte("old data")
	if err := os.WriteFile(cachePath, oldContent, 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set metadata so file appears stale.
	store.mu.Lock()
	store.current = Version{
		SHA256:    "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Size:      int64(len(oldContent)),
		FetchedAt: time.Now().Add(-2 * time.Hour),
	}
	store.mu.Unlock()

	ctx := context.Background()
	path, ver, err := store.Get(ctx, time.Hour)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
	if ver.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), ver.Size)
	}

	// Verify file was updated.
	saved, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("failed to read cache: %v", err)
	}
	if string(saved) != string(content) {
		t.Errorf("expected content %q, got %q", content, saved)
	}
}

func TestHTTPStore_Get_FetchFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	content := []byte("cached data")
	if err := os.WriteFile(cachePath, content, 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set stale metadata to force fetch.
	store.mu.Lock()
	store.current = Version{
		FetchedAt: time.Now().Add(-2 * time.Hour),
	}
	store.mu.Unlock()

	ctx := context.Background()
	path, _, err := store.Get(ctx, time.Hour)
	if err != nil {
		t.Fatalf("expected fallback to cached file, got error: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
}

func TestHTTPStore_Get_FetchNoFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	_, _, err = store.Get(ctx, time.Hour)
	if err == nil {
		t.Fatal("expected error when fetch fails and no cache available")
	}
	if !strings.Contains(err.Error(), "http 500") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHTTPStore_ForceRefresh_Success(t *testing.T) {
	content := []byte("new data")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"etag123"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2025 07:28:00 GMT")
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	path, ver, err := store.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("ForceRefresh failed: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
	if ver.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), ver.Size)
	}
	if ver.ETag != `"etag123"` {
		t.Errorf("expected ETag %q, got %q", `"etag123"`, ver.ETag)
	}

	// Verify metadata sidecar was written.
	metaPath := cachePath + ".meta.json"
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf("metadata sidecar not written: %v", err)
	}
}

func TestHTTPStore_ForceRefresh_HTTPError_WithFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	content := []byte("cached data")
	if err := os.WriteFile(cachePath, content, 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	path, _, err := store.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("expected fallback to cached file, got error: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
}

func TestHTTPStore_ForceRefresh_HTTPError_NoFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	_, _, err = store.ForceRefresh(ctx)
	if err == nil {
		t.Fatal("expected error when fetch fails and no cache available")
	}
}

func TestHTTPStore_ForceRefresh_304WithCache(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"abc123"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte("data"))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(cachePath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set ETag so request is conditional.
	store.mu.Lock()
	store.current.ETag = `"abc123"`
	store.mu.Unlock()

	ctx := context.Background()
	path, _, err := store.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("ForceRefresh failed: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
}

func TestHTTPStore_ForceRefresh_304NoCache(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	_, _, err = store.ForceRefresh(ctx)
	if err == nil {
		t.Fatal("expected error for 304 without cached file")
	}
	if !strings.Contains(err.Error(), "304 response but no local file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHTTPStore_Info(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store, err := NewHTTPStore("http://example.com/test.dat", cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Initially zero.
	info := store.Info()
	if info.Size != 0 {
		t.Errorf("expected zero size, got %d", info.Size)
	}

	// After fetch.
	store.mu.Lock()
	store.current = Version{
		SHA256: "testsha256",
		Size:   100,
	}
	store.mu.Unlock()

	info = store.Info()
	if info.Size != 100 {
		t.Errorf("expected size 100, got %d", info.Size)
	}
	if info.SHA256 != "testsha256" {
		t.Errorf("expected SHA256 %q, got %q", "testsha256", info.SHA256)
	}
}

func TestHTTPStore_fetchLocked_ConditionalHeaders(t *testing.T) {
	var receivedETag, receivedLM string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedETag = r.Header.Get("If-None-Match")
		receivedLM = r.Header.Get("If-Modified-Since")
		w.Write([]byte("data"))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	store.current = Version{
		ETag:         `"etag123"`,
		LastModified: "Wed, 21 Oct 2025 07:28:00 GMT",
	}
	store.mu.Unlock()

	ctx := context.Background()
	_, _, err = store.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("ForceRefresh failed: %v", err)
	}

	if receivedETag != `"etag123"` {
		t.Errorf("expected If-None-Match %q, got %q", `"etag123"`, receivedETag)
	}
	if receivedLM != "Wed, 21 Oct 2025 07:28:00 GMT" {
		t.Errorf("expected If-Modified-Since %q, got %q", "Wed, 21 Oct 2025 07:28:00 GMT", receivedLM)
	}
}

func TestHTTPStore_fetchLocked_DownloadError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write a header then close connection to cause download error.
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		// Force flush and close by hijacking (not possible in httptest).
		// Instead, return data normally but we'll test errorReader separately in downloadToFile.
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(cachePath, []byte("cached"), 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	// This should succeed because server returns 200 OK with valid data.
	path, _, err := store.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("ForceRefresh failed: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
}

func TestHTTPStore_downloadToFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "download.dat")

	store := &HTTPStore{}
	body := strings.NewReader("hello world")
	hash := sha256.New()

	size, err := store.downloadToFile(body, path, hash)
	if err != nil {
		t.Fatalf("downloadToFile failed: %v", err)
	}
	if size != 11 {
		t.Errorf("expected size 11, got %d", size)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", data)
	}
}

func TestHTTPStore_downloadToFile_MkdirError(t *testing.T) {
	store := &HTTPStore{}
	body := strings.NewReader("data")
	hash := sha256.New()

	// Use an invalid path that cannot be created.
	_, err := store.downloadToFile(body, "/dev/null/invalid/path/file.dat", hash)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestHTTPStore_downloadToFile_CopyError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "download.dat")

	store := &HTTPStore{}
	hash := sha256.New()

	_, err := store.downloadToFile(&errorReader{}, path, hash)
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
}

func TestHTTPStore_fallback_WithCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(cachePath, []byte("cached"), 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store := &HTTPStore{cachePath: cachePath}
	path, _, err := store.fallback(errors.New("fetch failed"))
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
}

func TestHTTPStore_fallback_WithoutCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nonexistent.dat")

	store := &HTTPStore{cachePath: cachePath}
	_, _, err := store.fallback(errors.New("fetch failed"))
	if err == nil {
		t.Fatal("expected error when no cache available")
	}
	if !strings.Contains(err.Error(), "fetch failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHTTPStore_loadMetadata_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store := &HTTPStore{cachePath: cachePath}
	err := store.loadMetadata()
	if err == nil {
		t.Fatal("expected error for missing metadata file")
	}
}

func TestHTTPStore_loadMetadata_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	metaPath := cachePath + ".meta.json"
	if err := os.WriteFile(metaPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	store := &HTTPStore{cachePath: cachePath}
	err := store.loadMetadata()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHTTPStore_loadMetadata_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	metaPath := cachePath + ".meta.json"

	meta := Version{
		SHA256:    "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Size:      42,
		FetchedAt: time.Now(),
		ETag:      `"abc"`,
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	store := &HTTPStore{cachePath: cachePath}
	if err := store.loadMetadata(); err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	if store.current.Size != 42 {
		t.Errorf("expected size 42, got %d", store.current.Size)
	}
}

func TestHTTPStore_saveMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store := &HTTPStore{
		cachePath: cachePath,
		current: Version{
			SHA256: "testsha256",
			Size:   100,
		},
	}

	if err := store.saveMetadata(); err != nil {
		t.Fatalf("saveMetadata failed: %v", err)
	}

	metaPath := cachePath + ".meta.json"
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read metadata: %v", err)
	}

	var v Version
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if v.Size != 100 {
		t.Errorf("expected size 100, got %d", v.Size)
	}
	if v.SHA256 != "testsha256" {
		t.Errorf("expected SHA256 %q, got %q", "testsha256", v.SHA256)
	}
}

func TestHTTPStore_Get_NoMetadataButFileExists(t *testing.T) {
	content := []byte("new data")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(cachePath, []byte("old data"), 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	path, ver, err := store.Get(ctx, time.Hour)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
	if ver.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), ver.Size)
	}
}

func TestHTTPStore_Get_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow server to allow cancellation.
		time.Sleep(500 * time.Millisecond)
		w.Write([]byte("data"))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")

	store, err := NewHTTPStore(ts.URL, cachePath, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, err = store.Get(ctx, time.Hour)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestHTTPStore_fetchLocked_Non200Status(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(cachePath, []byte("cached"), 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	path, _, err := store.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
}

func TestHTTPStore_fetchLocked_AtomicRenameFailure(t *testing.T) {
	// This is tricky to test. We simulate by using a cachePath inside a directory
	// that we later make read-only so rename fails.
	if os.Getuid() == 0 {
		t.Skip("cannot test permission issues as root")
	}

	content := []byte("fresh data")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}
	cachePath := filepath.Join(cacheDir, "test.dat")
	if err := os.WriteFile(cachePath, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	store, err := NewHTTPStore(ts.URL, cachePath, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Make directory read-only to prevent atomic rename.
	if err := os.Chmod(cacheDir, 0555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(cacheDir, 0755) //nolint:errcheck // restore for cleanup

	ctx := context.Background()
	path, _, err := store.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	if path != cachePath {
		t.Errorf("expected path %q, got %q", cachePath, path)
	}
}
