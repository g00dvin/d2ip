// Package source manages the local copy of dlc.dat, including HTTP fetch,
// integrity verification, and atomic file replacement.
package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Version represents metadata about a fetched dlc.dat file.
type Version struct {
	SHA256       string    `json:"sha256"`
	Size         int64     `json:"size"`
	FetchedAt    time.Time `json:"fetched_at"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
}

// DLCStore provides access to the local dlc.dat file with automatic refresh.
type DLCStore interface {
	// Get returns the path to a valid dlc.dat file. If the file is older than
	// maxAge, attempts to refresh it from the remote URL. If maxAge is 0,
	// returns the cached file without network access.
	Get(ctx context.Context, maxAge time.Duration) (path string, v Version, err error)

	// ForceRefresh unconditionally fetches a new copy from the remote URL.
	ForceRefresh(ctx context.Context) (path string, v Version, err error)

	// Info returns the current metadata without network access.
	Info() Version
}

// HTTPStore implements DLCStore using HTTP with ETag caching.
type HTTPStore struct {
	url       string
	cachePath string
	timeout   time.Duration
	client    *http.Client

	mu      sync.Mutex
	current Version
}

// NewHTTPStore creates a new DLCStore that fetches from the given URL.
// cachePath is the local file path where dlc.dat will be stored.
func NewHTTPStore(url, cachePath string, timeout time.Duration) (*HTTPStore, error) {
	if url == "" {
		return nil, errors.New("source: url cannot be empty")
	}
	if cachePath == "" {
		return nil, errors.New("source: cachePath cannot be empty")
	}

	s := &HTTPStore{
		url:       url,
		cachePath: cachePath,
		timeout:   timeout,
		client:    &http.Client{Timeout: timeout},
	}

	// Load metadata from sidecar file if it exists.
	if err := s.loadMetadata(); err != nil {
		log.Warn().Err(err).Msg("source: failed to load metadata, will create on first fetch")
	}

	return s, nil
}

// Get returns the path to a valid dlc.dat file.
func (s *HTTPStore) Get(ctx context.Context, maxAge time.Duration) (string, Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If maxAge is 0, return cached file without network access.
	if maxAge == 0 {
		if _, err := os.Stat(s.cachePath); err != nil {
			return "", Version{}, fmt.Errorf("source: no cached file available: %w", err)
		}
		return s.cachePath, s.current, nil
	}

	// Check if current file is fresh enough.
	if !s.current.FetchedAt.IsZero() && time.Since(s.current.FetchedAt) < maxAge {
		if _, err := os.Stat(s.cachePath); err == nil {
			return s.cachePath, s.current, nil
		}
	}

	// Need refresh.
	return s.fetchLocked(ctx)
}

// ForceRefresh unconditionally fetches a new copy.
func (s *HTTPStore) ForceRefresh(ctx context.Context) (string, Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetchLocked(ctx)
}

// Info returns the current metadata without network access.
func (s *HTTPStore) Info() Version {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// fetchLocked performs the actual HTTP fetch. Caller must hold s.mu.
func (s *HTTPStore) fetchLocked(ctx context.Context) (string, Version, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
	if err != nil {
		return s.fallback(fmt.Errorf("source: create request: %w", err))
	}

	// Add conditional request headers if we have cached metadata.
	if s.current.ETag != "" {
		req.Header.Set("If-None-Match", s.current.ETag)
	}
	if s.current.LastModified != "" {
		req.Header.Set("If-Modified-Since", s.current.LastModified)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return s.fallback(fmt.Errorf("source: http request: %w", err))
	}
	defer resp.Body.Close()

	// Handle 304 Not Modified.
	if resp.StatusCode == http.StatusNotModified {
		log.Info().Str("url", s.url).Msg("source: file not modified (304)")
		if _, err := os.Stat(s.cachePath); err == nil {
			return s.cachePath, s.current, nil
		}
		return "", Version{}, errors.New("source: 304 response but no local file")
	}

	// Only accept 200 OK.
	if resp.StatusCode != http.StatusOK {
		return s.fallback(fmt.Errorf("source: http %d from %s", resp.StatusCode, s.url))
	}

	// Download to temp file and compute SHA256.
	tempPath := s.cachePath + ".tmp"
	hash := sha256.New()
	size, err := s.downloadToFile(resp.Body, tempPath, hash)
	if err != nil {
		os.Remove(tempPath)
		return s.fallback(fmt.Errorf("source: download failed: %w", err))
	}

	// Atomic rename to final path.
	if err := os.Rename(tempPath, s.cachePath); err != nil {
		os.Remove(tempPath)
		return s.fallback(fmt.Errorf("source: atomic rename failed: %w", err))
	}

	// Update metadata.
	s.current = Version{
		SHA256:       hex.EncodeToString(hash.Sum(nil)),
		Size:         size,
		FetchedAt:    time.Now(),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	// Persist metadata to sidecar file.
	if err := s.saveMetadata(); err != nil {
		log.Warn().Err(err).Msg("source: failed to save metadata")
	}

	log.Info().
		Str("url", s.url).
		Int64("size", size).
		Str("sha256", s.current.SHA256[:16]+"...").
		Msg("source: fetched new file")

	return s.cachePath, s.current, nil
}

// downloadToFile writes the response body to a file and computes hash inline.
func (s *HTTPStore) downloadToFile(body io.Reader, path string, hash io.Writer) (int64, error) {
	// Ensure directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return 0, err
	}

	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Write to file and hash simultaneously.
	w := io.MultiWriter(f, hash)
	size, err := io.Copy(w, body)
	if err != nil {
		return 0, err
	}

	return size, f.Sync()
}

// fallback returns the cached file if available, otherwise returns the error.
func (s *HTTPStore) fallback(fetchErr error) (string, Version, error) {
	if _, err := os.Stat(s.cachePath); err == nil {
		log.Warn().Err(fetchErr).Msg("source: using cached file after fetch failure")
		return s.cachePath, s.current, nil
	}
	return "", Version{}, fetchErr
}

// loadMetadata reads metadata from the sidecar file.
func (s *HTTPStore) loadMetadata() error {
	metaPath := s.cachePath + ".meta.json"
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return err
	}

	var v Version
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	s.current = v
	log.Debug().Str("sha256", v.SHA256[:16]+"...").Time("fetched_at", v.FetchedAt).Msg("source: loaded metadata")
	return nil
}

// saveMetadata writes metadata to the sidecar file.
func (s *HTTPStore) saveMetadata() error {
	metaPath := s.cachePath + ".meta.json"
	data, err := json.MarshalIndent(s.current, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}
