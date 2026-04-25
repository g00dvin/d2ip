package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodvin/d2ip/internal/config"
)

// PolicyExporter writes per-policy exports in multiple formats.
type PolicyExporter struct {
	baseDir string
}

func NewPolicyExporter(baseDir string) *PolicyExporter {
	return &PolicyExporter{baseDir: baseDir}
}

// PolicyExportReport is the result of writing a policy export.
type PolicyExportReport struct {
	PolicyName string
	Format     string
	IPv4Path   string
	IPv6Path   string
	IPv4Count  int
	IPv6Count  int
	Unchanged  bool
}

func (e *PolicyExporter) WritePolicy(ctx context.Context, policy config.PolicyConfig, v4, v6 []netip.Prefix) (PolicyExportReport, error) {
	dir := filepath.Join(e.baseDir, policy.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return PolicyExportReport{}, err
	}

	format := policy.ExportFormat
	if format == "" {
		format = "plain"
	}

	v4Path := filepath.Join(dir, "ipv4."+extForFormat(format))
	v6Path := filepath.Join(dir, "ipv6."+extForFormat(format))

	v4Data := formatPrefixesForPolicy(format, policy.Name, v4, "v4")
	v6Data := formatPrefixesForPolicy(format, policy.Name, v6, "v6")

	if err := os.WriteFile(v4Path, []byte(v4Data), 0644); err != nil {
		return PolicyExportReport{}, fmt.Errorf("write v4: %w", err)
	}
	if err := os.WriteFile(v6Path, []byte(v6Data), 0644); err != nil {
		return PolicyExportReport{}, fmt.Errorf("write v6: %w", err)
	}

	return PolicyExportReport{
		PolicyName: policy.Name,
		Format:     format,
		IPv4Path:   v4Path,
		IPv6Path:   v6Path,
		IPv4Count:  len(v4),
		IPv6Count:  len(v6),
	}, nil
}

func extForFormat(format string) string {
	switch format {
	case "ipset":
		return "ipset"
	case "json":
		return "json"
	case "nft":
		return "nft"
	case "iptables":
		return "iptables"
	case "bgp":
		return "bgp"
	case "yaml":
		return "yaml"
	default:
		return "txt"
	}
}

func formatPrefixesForPolicy(format, policyName string, prefixes []netip.Prefix, family string) string {
	switch format {
	case "ipset":
		return formatIPSet(policyName, family, prefixes)
	case "json":
		return formatJSON(policyName, family, prefixes)
	case "nft":
		return formatNFT(policyName, family, prefixes)
	case "iptables":
		return formatIPTables(family, prefixes)
	case "bgp":
		return formatBGP(policyName, prefixes)
	case "yaml":
		return formatYAML(policyName, family, prefixes)
	default:
		return formatPlain(prefixes)
	}
}

func formatPrefixes(prefixes []string, format string) (string, error) {
	var parsed []netip.Prefix
	for _, s := range prefixes {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return "", err
		}
		parsed = append(parsed, p)
	}

	switch format {
	case "ipset":
		return formatIPSet("policy", "v4", parsed), nil
	case "json":
		return formatJSON("policy", "v4", parsed), nil
	case "nft":
		return formatNFT("policy", "v4", parsed), nil
	case "iptables":
		return formatIPTables("v4", parsed), nil
	case "bgp":
		return formatBGP("policy", parsed), nil
	case "yaml":
		return formatYAML("policy", "v4", parsed), nil
	default:
		return formatPlain(parsed), nil
	}
}

func formatPlain(prefixes []netip.Prefix) string {
	var lines []string
	for _, p := range prefixes {
		lines = append(lines, p.String())
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatIPSet(policyName, family string, prefixes []netip.Prefix) string {
	setName := policyName + "_" + family
	var lines []string
	for _, p := range prefixes {
		lines = append(lines, fmt.Sprintf("add %s %s", setName, p.String()))
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatJSON(policyName, family string, prefixes []netip.Prefix) string {
	data := map[string]interface{}{
		"policy":   policyName,
		"family":   family,
		"prefixes": prefixStrings(prefixes),
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b) + "\n"
}

func formatNFT(policyName, family string, prefixes []netip.Prefix) string {
	var lines []string
	for _, p := range prefixes {
		lines = append(lines, fmt.Sprintf("add elements inet d2ip %s_%s { %s }", policyName, family, p.String()))
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatIPTables(family string, prefixes []netip.Prefix) string {
	cmd := "iptables"
	if family == "v6" {
		cmd = "ip6tables"
	}
	var lines []string
	for _, p := range prefixes {
		lines = append(lines, fmt.Sprintf("%s -A OUTPUT -d %s -j DROP", cmd, p.String()))
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatBGP(policyName string, prefixes []netip.Prefix) string {
	var lines []string
	for _, p := range prefixes {
		lines = append(lines, fmt.Sprintf("network %s", p.String()))
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatYAML(policyName, family string, prefixes []netip.Prefix) string {
	// Use simple manual YAML to avoid dependency
	var b strings.Builder
	b.WriteString(fmt.Sprintf("policy: %s\n", policyName))
	b.WriteString(fmt.Sprintf("family: %s\n", family))
	b.WriteString(fmt.Sprintf("count: %d\n", len(prefixes)))
	b.WriteString("prefixes:\n")
	for _, p := range prefixes {
		b.WriteString(fmt.Sprintf("  - %s\n", p.String()))
	}
	return b.String()
}

func prefixStrings(prefixes []netip.Prefix) []string {
	var out []string
	for _, p := range prefixes {
		out = append(out, p.String())
	}
	return out
}
