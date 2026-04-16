//go:build routing_integration

package routing

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// createNetns creates an isolated network namespace for testing.
// Returns a cleanup function that must be called to remove the namespace.
// Requires CAP_NET_ADMIN capability (run tests with sudo).
func createNetns(t *testing.T, name string) func() {
	t.Helper()

	// Check if we have CAP_NET_ADMIN
	if os.Geteuid() != 0 {
		t.Skip("Skipping integration test: requires CAP_NET_ADMIN (run with sudo)")
	}

	// Check if ip command exists
	if _, err := exec.LookPath("ip"); err != nil {
		t.Skip("Skipping integration test: 'ip' command not found")
	}

	// Create network namespace
	cmd := exec.Command("ip", "netns", "add", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if namespace already exists
		if !strings.Contains(string(output), "File exists") {
			t.Fatalf("Failed to create netns %s: %v\nOutput: %s", name, err, output)
		}
		// If it exists, delete and recreate
		exec.Command("ip", "netns", "del", name).Run()
		if output, err := exec.Command("ip", "netns", "add", name).CombinedOutput(); err != nil {
			t.Fatalf("Failed to recreate netns %s: %v\nOutput: %s", name, err, output)
		}
	}

	t.Logf("Created network namespace: %s", name)

	// Return cleanup function
	return func() {
		cmd := exec.Command("ip", "netns", "del", name)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Logf("Warning: failed to delete netns %s: %v\nOutput: %s", name, err, output)
		} else {
			t.Logf("Deleted network namespace: %s", name)
		}
	}
}

// runInNetns executes a command inside the network namespace.
func runInNetns(t *testing.T, ns string, name string, args ...string) ([]byte, error) {
	t.Helper()

	cmdArgs := append([]string{"netns", "exec", ns, name}, args...)
	cmd := exec.Command("ip", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command failed in netns %s: ip %s\nOutput: %s", ns, strings.Join(cmdArgs, " "), output)
	}
	return output, err
}

// createDummyInterface creates a dummy network interface in the namespace.
func createDummyInterface(t *testing.T, ns, ifname string) {
	t.Helper()

	// Create dummy interface
	if _, err := runInNetns(t, ns, "ip", "link", "add", ifname, "type", "dummy"); err != nil {
		t.Fatalf("Failed to create dummy interface %s: %v", ifname, err)
	}

	// Bring interface up
	if _, err := runInNetns(t, ns, "ip", "link", "set", ifname, "up"); err != nil {
		t.Fatalf("Failed to bring up interface %s: %v", ifname, err)
	}

	t.Logf("Created dummy interface %s in netns %s", ifname, ns)
}

// checkNftablesAvailable checks if nftables is available in the namespace.
func checkNftablesAvailable(t *testing.T, ns string) bool {
	t.Helper()

	if _, err := exec.LookPath("nft"); err != nil {
		return false
	}

	// Try to list tables in namespace
	_, err := runInNetns(t, ns, "nft", "list", "tables")
	return err == nil
}

// getNftSet retrieves the contents of an nftables set.
func getNftSet(t *testing.T, ns, table, set string) ([]string, error) {
	t.Helper()

	output, err := runInNetns(t, ns, "nft", "list", "set", table, set)
	if err != nil {
		return nil, fmt.Errorf("nft list set failed: %w", err)
	}

	// Parse output to extract elements
	// Format: elements = { 192.0.2.0/24, ... }
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "elements = {") {
			// Extract content between { and }
			start := strings.Index(line, "{")
			end := strings.LastIndex(line, "}")
			if start >= 0 && end > start {
				content := line[start+1 : end]
				content = strings.TrimSpace(content)
				if content == "" {
					return []string{}, nil
				}
				// Split by comma and trim spaces
				elements := strings.Split(content, ",")
				for i := range elements {
					elements[i] = strings.TrimSpace(elements[i])
				}
				return elements, nil
			}
		}
	}

	return []string{}, nil
}

// getIpRoutes retrieves routes from the specified table.
func getIpRoutes(t *testing.T, ns string, table int) ([]string, error) {
	t.Helper()

	output, err := runInNetns(t, ns, "ip", "route", "show", "table", fmt.Sprintf("%d", table))
	if err != nil {
		return nil, fmt.Errorf("ip route show failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var routes []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			routes = append(routes, line)
		}
	}
	return routes, nil
}
