package api

import (
	"io"
	"io/fs"
	"testing"
)

// TestWebFilesEmbedded verifies that web UI files are embedded in the binary.
func TestWebFilesEmbedded(t *testing.T) {
	tests := []struct {
		path string
		want string // substring to check
	}{
		{"web/index.html", "<!DOCTYPE html>"},
		{"web/index.html", "d2ip"},
		{"web/index.html", "htmx.org"},
		{"web/styles.css", ":root"},
		{"web/styles.css", "--color-primary"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			f, err := webFS.Open(tt.path)
			if err != nil {
				t.Fatalf("failed to open %s: %v", tt.path, err)
			}
			defer f.Close()

			content, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("failed to read %s: %v", tt.path, err)
			}

			if len(content) == 0 {
				t.Errorf("%s is empty", tt.path)
			}

			// Check for expected substring
			if tt.want != "" {
				s := string(content)
				if len(s) < len(tt.want) || !contains(s, tt.want) {
					t.Errorf("%s does not contain %q", tt.path, tt.want)
				}
			}
		})
	}
}

// TestWebFilesSize verifies total web size is under 50KB.
func TestWebFilesSize(t *testing.T) {
	var total int64
	err := fs.WalkDir(webFS, "web", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk web dir: %v", err)
	}

	const maxSize = 50 * 1024 // 50KB
	if total > maxSize {
		t.Errorf("web files size %d bytes exceeds %d bytes", total, maxSize)
	}
	t.Logf("Total web size: %d bytes (%.1fKB)", total, float64(total)/1024)
}

// contains is a simple substring check helper.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
