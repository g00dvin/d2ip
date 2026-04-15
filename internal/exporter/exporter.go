// Package exporter is the file export agent for d2ip. It owns the atomic
// write flow for ipv4.txt and ipv6.txt prefix files, including SHA256 digest
// tracking to detect and skip unnecessary rewrites.
//
// # Atomic write flow
//
// Each family (IPv4, IPv6) follows this sequence:
//  1. Create temp file in baseDir: os.CreateTemp(dir, "ipv4-*.tmp")
//  2. Write prefixes line-by-line while computing SHA256 digest
//  3. fsync() + Close() temp file to ensure data reaches disk
//  4. Load previous SHA256 from sidecar file (e.g., ipv4.txt.sha256)
//  5. If SHA differs: os.Rename(temp, final) for atomic replacement
//  6. If SHA matches: delete temp, set Unchanged=true
//  7. Write new SHA to sidecar file
//  8. fsync() parent directory for crash safety (metadata durability)
//
// # Crash safety
//
// The rename operation is atomic on POSIX filesystems when both paths are
// on the same device. Concurrent readers never observe a partial write:
// they either see the old complete file or the new complete file. The
// parent directory fsync ensures that the rename operation metadata is
// durable across power loss.
//
// # Unchanged detection
//
// If the computed SHA256 matches the previous digest stored in the sidecar
// file, the final rename is skipped and Unchanged=true is returned in the
// ExportReport. This prevents unnecessary disk I/O for no-op runs while
// still recomputing the digest to verify data integrity.
package exporter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
)

// ExportReport summarizes the results of a Write operation, including
// file paths, prefix counts, SHA256 digests, and whether the operation
// was a no-op (unchanged data).
type ExportReport struct {
	IPv4Path   string
	IPv6Path   string
	IPv4Count  int
	IPv6Count  int
	IPv4Digest string // sha256 hex
	IPv6Digest string
	Unchanged  bool // true if both SHA matched previous
}

// FileExporter writes aggregated CIDR prefixes to ipv4.txt and ipv6.txt
// with atomic file replacement and SHA256 digest tracking.
type FileExporter struct {
	baseDir string
}

// New creates a FileExporter that will write files to baseDir. The
// directory is created with 0755 permissions if it does not exist.
func New(baseDir string) (*FileExporter, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}
	return &FileExporter{baseDir: baseDir}, nil
}

// Write atomically exports IPv4 and IPv6 prefix lists to ipv4.txt and
// ipv6.txt respectively. If the computed SHA256 digest matches the
// previous digest (stored in .sha256 sidecar files), the final file is
// not rewritten and Unchanged=true is returned.
//
// Prefixes are written one per line in CIDR notation with LF newlines
// and a trailing newline. The caller must ensure prefixes are already
// sorted and deduplicated (aggregator contract).
func (e *FileExporter) Write(ctx context.Context, v4 []netip.Prefix, v6 []netip.Prefix) (ExportReport, error) {
	var report ExportReport

	// Write IPv4
	v4Path := filepath.Join(e.baseDir, "ipv4.txt")
	v4Digest, v4Changed, err := e.writeFamily(ctx, "ipv4", v4Path, v4)
	if err != nil {
		return report, fmt.Errorf("write ipv4: %w", err)
	}
	report.IPv4Path = v4Path
	report.IPv4Count = len(v4)
	report.IPv4Digest = v4Digest

	// Write IPv6
	v6Path := filepath.Join(e.baseDir, "ipv6.txt")
	v6Digest, v6Changed, err := e.writeFamily(ctx, "ipv6", v6Path, v6)
	if err != nil {
		return report, fmt.Errorf("write ipv6: %w", err)
	}
	report.IPv6Path = v6Path
	report.IPv6Count = len(v6)
	report.IPv6Digest = v6Digest

	// Unchanged is true only if BOTH families were unchanged
	report.Unchanged = !v4Changed && !v6Changed

	return report, nil
}

// writeFamily implements the atomic write flow for a single address family.
// Returns (digest, changed, error) where changed=true means the file was
// actually rewritten (digest differed from previous).
func (e *FileExporter) writeFamily(ctx context.Context, family, finalPath string, prefixes []netip.Prefix) (string, bool, error) {
	// Check context before heavy I/O
	select {
	case <-ctx.Done():
		return "", false, ctx.Err()
	default:
	}

	// Step 1: Create temp file
	tmpFile, err := os.CreateTemp(e.baseDir, family+"-*.tmp")
	if err != nil {
		return "", false, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure temp file is cleaned up on error
	defer func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Step 2: Write prefixes while computing SHA256
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	for _, prefix := range prefixes {
		if _, err := fmt.Fprintf(writer, "%s\n", prefix.String()); err != nil {
			return "", false, fmt.Errorf("write prefix: %w", err)
		}
	}

	// Step 3: fsync + close temp file
	if err := tmpFile.Sync(); err != nil {
		return "", false, fmt.Errorf("sync temp: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", false, fmt.Errorf("close temp: %w", err)
	}
	tmpFile = nil // Mark as closed for defer cleanup

	// Compute final digest
	digest := hex.EncodeToString(hasher.Sum(nil))

	// Step 4: Load previous SHA256 from sidecar
	shaPath := finalPath + ".sha256"
	previousSHA, _ := os.ReadFile(shaPath) // Ignore error; missing file is ok
	previousDigest := string(previousSHA)

	// Step 5: Compare digests and conditionally rename
	changed := digest != previousDigest
	if changed {
		// Atomic rename (POSIX guarantee on same filesystem)
		if err := os.Rename(tmpPath, finalPath); err != nil {
			return "", false, fmt.Errorf("rename temp to final: %w", err)
		}

		// Set file permissions explicitly (umask may vary)
		if err := os.Chmod(finalPath, 0644); err != nil {
			return "", false, fmt.Errorf("chmod final: %w", err)
		}

		// Step 8: fsync parent directory for metadata durability
		if err := fsyncDir(e.baseDir); err != nil {
			return "", false, fmt.Errorf("fsync parent: %w", err)
		}
	} else {
		// Step 6: Unchanged, remove temp file
		os.Remove(tmpPath)
	}

	// Step 7: Write SHA to sidecar (always, even if unchanged, to update mtime)
	if err := os.WriteFile(shaPath, []byte(digest), 0644); err != nil {
		return "", false, fmt.Errorf("write sha sidecar: %w", err)
	}

	return digest, changed, nil
}

// fsyncDir opens the directory and calls Sync() to ensure metadata (rename
// operations) are durable on disk. This is critical for crash safety.
func fsyncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
