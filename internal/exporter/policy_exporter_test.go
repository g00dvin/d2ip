package exporter

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodvin/d2ip/internal/config"
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

func TestFormatPrefixesError(t *testing.T) {
	_, err := formatPrefixes([]string{"invalid"}, "plain")
	if err == nil {
		t.Error("formatPrefixes with invalid prefix should fail")
	}
}

func TestNewPolicyExporter(t *testing.T) {
	e := NewPolicyExporter("/tmp/policies")
	if e == nil {
		t.Fatal("NewPolicyExporter() returned nil")
	}
	if e.baseDir != "/tmp/policies" {
		t.Errorf("baseDir = %q, want %q", e.baseDir, "/tmp/policies")
	}
}

func TestWritePolicy(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   []string
	}{
		{"plain", "plain", []string{"1.2.3.0/24"}},
		{"json", "json", []string{"1.2.3.0/24", `"family": "v4"`}},
		{"ipset", "ipset", []string{"add testpolicy_v4 1.2.3.0/24"}},
		{"nft", "nft", []string{"add elements inet d2ip testpolicy_v4 { 1.2.3.0/24 }"}},
		{"iptables", "iptables", []string{"iptables -A OUTPUT -d 1.2.3.0/24 -j DROP"}},
		{"bgp", "bgp", []string{"network 1.2.3.0/24"}},
		{"yaml", "yaml", []string{"policy: testpolicy", "family: v4", "prefixes:", "- 1.2.3.0/24"}},
		{"default", "", []string{"1.2.3.0/24"}}, // empty format defaults to plain
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			e := NewPolicyExporter(dir)
			policy := config.PolicyConfig{
				Name:         "testpolicy",
				ExportFormat: tt.format,
			}
			v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
			v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

			report, err := e.WritePolicy(context.Background(), policy, v4, v6)
			if err != nil {
				t.Fatalf("WritePolicy() failed: %v", err)
			}
			if report.PolicyName != "testpolicy" {
				t.Errorf("PolicyName = %q, want %q", report.PolicyName, "testpolicy")
			}
			if report.Format != tt.format && tt.format != "" {
				t.Errorf("Format = %q, want %q", report.Format, tt.format)
			}
			if tt.format == "" && report.Format != "plain" {
				t.Errorf("Format = %q, want %q", report.Format, "plain")
			}
			if report.IPv4Count != 1 {
				t.Errorf("IPv4Count = %d, want 1", report.IPv4Count)
			}
			if report.IPv6Count != 1 {
				t.Errorf("IPv6Count = %d, want 1", report.IPv6Count)
			}

			v4Content, err := os.ReadFile(report.IPv4Path)
			if err != nil {
				t.Fatalf("read v4 file: %v", err)
			}
			for _, sub := range tt.want {
				if !strings.Contains(string(v4Content), sub) {
					t.Errorf("v4 content missing %q in:\n%s", sub, v4Content)
				}
			}

			v6Content, err := os.ReadFile(report.IPv6Path)
			if err != nil {
				t.Fatalf("read v6 file: %v", err)
			}
			if !strings.Contains(string(v6Content), "2001:db8::/32") {
				t.Errorf("v6 content missing prefix in:\n%s", v6Content)
			}
		})
	}
}

func TestWritePolicyMkdirError(t *testing.T) {
	// Use a file as baseDir so os.MkdirAll fails
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "block")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	e := NewPolicyExporter(blockingFile)
	policy := config.PolicyConfig{Name: "testpolicy"}
	_, err := e.WritePolicy(context.Background(), policy, nil, nil)
	if err == nil {
		t.Error("WritePolicy with invalid baseDir should fail")
	}
}

func TestExtForFormat(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{"plain", "txt"},
		{"ipset", "ipset"},
		{"json", "json"},
		{"nft", "nft"},
		{"iptables", "iptables"},
		{"bgp", "bgp"},
		{"yaml", "yaml"},
		{"unknown", "txt"},
		{"", "txt"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := extForFormat(tt.format)
			if got != tt.want {
				t.Errorf("extForFormat(%q) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

func TestFormatPrefixesForPolicy(t *testing.T) {
	v4 := []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	tests := []struct {
		name    string
		format  string
		family  string
		wantSub string
	}{
		{"plain-v4", "plain", "v4", "1.2.3.0/24"},
		{"plain-v6", "plain", "v6", "2001:db8::/32"},
		{"json-v4", "json", "v4", `"family": "v4"`},
		{"json-v6", "json", "v6", `"family": "v6"`},
		{"ipset-v4", "ipset", "v4", "add policy_v4 1.2.3.0/24"},
		{"nft-v4", "nft", "v4", "add elements inet d2ip policy_v4 { 1.2.3.0/24 }"},
		{"iptables-v4", "iptables", "v4", "iptables -A OUTPUT -d 1.2.3.0/24 -j DROP"},
		{"iptables-v6", "iptables", "v6", "ip6tables -A OUTPUT -d 2001:db8::/32 -j DROP"},
		{"bgp", "bgp", "v4", "network 1.2.3.0/24"},
		{"yaml", "yaml", "v4", "policy: policy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prefixes []netip.Prefix
			if tt.family == "v4" {
				prefixes = v4
			} else {
				prefixes = v6
			}
			got := formatPrefixesForPolicy(tt.format, "policy", prefixes, tt.family)
			if !strings.Contains(got, tt.wantSub) {
				t.Errorf("formatPrefixesForPolicy missing %q in output:\n%s", tt.wantSub, got)
			}
		})
	}
}

func TestFormatIPTablesIPv6(t *testing.T) {
	prefixes := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}
	got := formatIPTables("v6", prefixes)
	want := "ip6tables -A OUTPUT -d 2001:db8::/32 -j DROP"
	if !strings.Contains(got, want) {
		t.Errorf("formatIPTables(v6) missing %q in output:\n%s", want, got)
	}
}
