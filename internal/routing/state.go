package routing

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// loadState reads the persisted RouterState from path. Returns a zero-valued
// state (no error) if the file does not exist — first-apply case.
func loadState(path string) (RouterState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return RouterState{}, nil
		}
		return RouterState{}, fmt.Errorf("routing: read state: %w", err)
	}
	var s RouterState
	if err := json.Unmarshal(b, &s); err != nil {
		return RouterState{}, fmt.Errorf("routing: parse state: %w", err)
	}
	return s, nil
}

// saveState atomically writes the state file (write to tmp + rename).
// Creates the parent directory if missing.
func saveState(path string, s RouterState) error {
	if path == "" {
		return errors.New("routing: empty state path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("routing: mkdir state dir: %w", err)
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("routing: marshal state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("routing: write state tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("routing: rename state: %w", err)
	}
	return nil
}
