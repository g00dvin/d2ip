package exporter

import (
	"strings"
	"testing"
)

func TestFormatPrefixes(t *testing.T) {
	prefixes := []string{"1.2.3.0/24", "2001:db8::/32"}

	tests := []struct {
		name   string
		format string
		want   []string // substrings expected in output
	}{
		{"plain", "plain", []string{"1.2.3.0/24", "2001:db8::/32"}},
		{"json", "json", []string{"[", "1.2.3.0/24", "2001:db8::/32", "]"}},
		{"ipset", "ipset", []string{"add", "1.2.3.0/24"}},
		{"nft", "nft", []string{"elements", "1.2.3.0/24"}},
		{"iptables", "iptables", []string{"-A", "1.2.3.0/24"}},
		{"bgp", "bgp", []string{"network", "1.2.3.0/24"}},
		{"yaml", "yaml", []string{"prefixes:", "- 1.2.3.0/24"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatPrefixes(prefixes, tt.format)
			if err != nil {
				t.Fatalf("formatPrefixes error: %v", err)
			}
			for _, sub := range tt.want {
				if !strings.Contains(got, sub) {
					t.Errorf("formatPrefixes(%q) missing %q in output:\n%s", tt.format, sub, got)
				}
			}
		})
	}
}
