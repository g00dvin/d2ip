package source

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CustomSource loads domain lists from local files.
type CustomSource struct {
	baseDir string
}

// NewCustomSource creates a new custom list loader.
func NewCustomSource(baseDir string) *CustomSource {
	return &CustomSource{baseDir: baseDir}
}

// LoadDomains reads a custom domain list by name.
func (s *CustomSource) LoadDomains(name string) ([]string, error) {
	path := filepath.Join(s.baseDir, name+".txt")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open custom list %q: %w", name, err)
	}
	defer f.Close()

	var domains []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			domains = append(domains, line)
		}
	}
	return domains, scanner.Err()
}

// SaveDomains writes a custom domain list.
func (s *CustomSource) SaveDomains(name string, domains []string) error {
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(s.baseDir, name+".txt")
	data := strings.Join(domains, "\n") + "\n"
	return os.WriteFile(path, []byte(data), 0644)
}
