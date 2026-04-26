package exporter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "nested", "export")

	exp, err := New(subDir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if exp.baseDir != subDir {
		t.Errorf("baseDir = %q, want %q", exp.baseDir, subDir)
	}

	// Verify directory was created
	info, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("stat baseDir: %v", err)
	}
	if !info.IsDir() {
		t.Error("baseDir is not a directory")
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("baseDir mode = %o, want 0755", info.Mode().Perm())
	}
}

func TestWriteBasic(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	v4 := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("198.51.100.0/24"),
	}
	v6 := []netip.Prefix{
		netip.MustParsePrefix("2001:db8::/32"),
	}

	report, err := exp.Write(context.Background(), v4, v6)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Verify report
	if report.IPv4Count != 2 {
		t.Errorf("IPv4Count = %d, want 2", report.IPv4Count)
	}
	if report.IPv6Count != 1 {
		t.Errorf("IPv6Count = %d, want 1", report.IPv6Count)
	}
	if report.IPv4Digest == "" {
		t.Error("IPv4Digest is empty")
	}
	if report.IPv6Digest == "" {
		t.Error("IPv6Digest is empty")
	}
	if report.Unchanged {
		t.Error("Unchanged = true on first write, want false")
	}

	// Verify IPv4 file content
	v4Content, err := os.ReadFile(report.IPv4Path)
	if err != nil {
		t.Fatalf("read ipv4.txt: %v", err)
	}
	wantV4 := "192.0.2.0/24\n198.51.100.0/24\n"
	if string(v4Content) != wantV4 {
		t.Errorf("ipv4.txt = %q, want %q", v4Content, wantV4)
	}

	// Verify IPv6 file content
	v6Content, err := os.ReadFile(report.IPv6Path)
	if err != nil {
		t.Fatalf("read ipv6.txt: %v", err)
	}
	wantV6 := "2001:db8::/32\n"
	if string(v6Content) != wantV6 {
		t.Errorf("ipv6.txt = %q, want %q", v6Content, wantV6)
	}

	// Verify file permissions
	v4Info, _ := os.Stat(report.IPv4Path)
	if v4Info.Mode().Perm() != 0644 {
		t.Errorf("ipv4.txt mode = %o, want 0644", v4Info.Mode().Perm())
	}
}

func TestWriteUnchanged(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	v4 := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	// First write
	report1, err := exp.Write(context.Background(), v4, v6)
	if err != nil {
		t.Fatalf("Write() #1 failed: %v", err)
	}
	if report1.Unchanged {
		t.Error("First write should not be unchanged")
	}

	// Get mtime before second write
	v4Info1, _ := os.Stat(report1.IPv4Path)
	v4Mtime1 := v4Info1.ModTime()

	// Second write with same data
	time.Sleep(10 * time.Millisecond) // Ensure mtime would differ if rewritten
	report2, err := exp.Write(context.Background(), v4, v6)
	if err != nil {
		t.Fatalf("Write() #2 failed: %v", err)
	}

	// Verify unchanged detection
	if !report2.Unchanged {
		t.Error("Second write with same data should be unchanged")
	}
	if report2.IPv4Digest != report1.IPv4Digest {
		t.Error("Digest should match between unchanged writes")
	}

	// Verify file was NOT rewritten (mtime should be same)
	v4Info2, _ := os.Stat(report2.IPv4Path)
	v4Mtime2 := v4Info2.ModTime()
	if !v4Mtime2.Equal(v4Mtime1) {
		t.Errorf("ipv4.txt mtime changed: %v -> %v (should be unchanged)", v4Mtime1, v4Mtime2)
	}

	// Third write with different data
	v4Different := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}
	report3, err := exp.Write(context.Background(), v4Different, v6)
	if err != nil {
		t.Fatalf("Write() #3 failed: %v", err)
	}
	if report3.Unchanged {
		t.Error("Write with different data should not be unchanged")
	}
	if report3.IPv4Digest == report2.IPv4Digest {
		t.Error("Digest should differ after data change")
	}
}

func TestSHASidecar(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	v4 := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	report, err := exp.Write(context.Background(), v4, v6)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Verify SHA sidecar files exist
	v4SHAPath := report.IPv4Path + ".sha256"
	v4SHA, err := os.ReadFile(v4SHAPath)
	if err != nil {
		t.Fatalf("read ipv4.txt.sha256: %v", err)
	}
	if string(v4SHA) != report.IPv4Digest {
		t.Errorf("sidecar SHA = %q, report digest = %q", v4SHA, report.IPv4Digest)
	}

	// Verify digest correctness
	v4Content, _ := os.ReadFile(report.IPv4Path)
	expectedDigest := sha256.Sum256(v4Content)
	expectedHex := hex.EncodeToString(expectedDigest[:])
	if report.IPv4Digest != expectedHex {
		t.Errorf("IPv4Digest = %q, want %q", report.IPv4Digest, expectedHex)
	}
}

func TestAtomicWrite(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	v4Initial := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	// Write initial data
	report1, err := exp.Write(context.Background(), v4Initial, v6)
	if err != nil {
		t.Fatalf("Write() initial failed: %v", err)
	}

	// Start concurrent readers
	var wg sync.WaitGroup
	stopReaders := make(chan struct{})
	partialReads := make(chan string, 10)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopReaders:
					return
				default:
					data, err := os.ReadFile(report1.IPv4Path)
					if err == nil {
						// Check for partial writes (content should always be valid)
						content := string(data)
						if content != "192.0.2.0/24\n" && content != "203.0.113.0/24\n" {
							partialReads <- content
						}
					}
				}
			}
		}()
	}

	// Perform write with different data
	time.Sleep(10 * time.Millisecond)
	v4Different := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}
	_, err = exp.Write(context.Background(), v4Different, v6)
	if err != nil {
		t.Fatalf("Write() concurrent failed: %v", err)
	}

	// Stop readers and check for partial reads
	close(stopReaders)
	wg.Wait()
	close(partialReads)

	if len(partialReads) > 0 {
		partial := <-partialReads
		t.Errorf("Concurrent reader observed partial write: %q", partial)
	}
}

func TestEmptyPrefixes(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	report, err := exp.Write(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Write() with empty prefixes failed: %v", err)
	}

	if report.IPv4Count != 0 {
		t.Errorf("IPv4Count = %d, want 0", report.IPv4Count)
	}

	// Empty files should still be created
	v4Content, err := os.ReadFile(report.IPv4Path)
	if err != nil {
		t.Fatalf("read ipv4.txt: %v", err)
	}
	if string(v4Content) != "" {
		t.Errorf("empty ipv4.txt = %q, want empty string", v4Content)
	}

	// Digest of empty content
	emptyDigest := sha256.Sum256([]byte{})
	expectedHex := hex.EncodeToString(emptyDigest[:])
	if report.IPv4Digest != expectedHex {
		t.Errorf("empty IPv4Digest = %q, want %q", report.IPv4Digest, expectedHex)
	}
}

func TestContextCancellation(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	v4 := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	_, err = exp.Write(ctx, v4, nil)
	if err == nil {
		t.Error("Write() with cancelled context should fail")
	}
}

func TestLargePrefixList(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Generate 10,000 prefixes (memory profile test would use 1M)
	// We use 10k here for faster test execution
	var v4 []netip.Prefix
	for i := 0; i < 10000; i++ {
		// Generate sequential /24 prefixes
		a := byte((i >> 16) % 256)
		b := byte((i >> 8) % 256)
		c := byte(i % 256)
		addr := netip.AddrFrom4([4]byte{a, b, c, 0})
		v4 = append(v4, netip.PrefixFrom(addr, 24))
	}

	report, err := exp.Write(context.Background(), v4, nil)
	if err != nil {
		t.Fatalf("Write() with large prefix list failed: %v", err)
	}

	if report.IPv4Count != 10000 {
		t.Errorf("IPv4Count = %d, want 10000", report.IPv4Count)
	}

	// Verify file was written correctly
	v4Content, err := os.ReadFile(report.IPv4Path)
	if err != nil {
		t.Fatalf("read ipv4.txt: %v", err)
	}

	// Verify digest matches content
	actualDigest := sha256.Sum256(v4Content)
	actualHex := hex.EncodeToString(actualDigest[:])
	if report.IPv4Digest != actualHex {
		t.Error("Large file digest mismatch")
	}
}

func TestIPv6Format(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	v6 := []netip.Prefix{
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2001:db8:1::/48"),
		netip.MustParsePrefix("fd00::/8"),
	}

	report, err := exp.Write(context.Background(), nil, v6)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	v6Content, err := os.ReadFile(report.IPv6Path)
	if err != nil {
		t.Fatalf("read ipv6.txt: %v", err)
	}

	want := "2001:db8::/32\n2001:db8:1::/48\nfd00::/8\n"
	if string(v6Content) != want {
		t.Errorf("ipv6.txt = %q, want %q", v6Content, want)
	}
}

func TestPartialUnchanged(t *testing.T) {
	exp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	v4 := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}

	// First write
	report1, err := exp.Write(context.Background(), v4, v6)
	if err != nil {
		t.Fatalf("Write() #1 failed: %v", err)
	}

	// Second write: change only IPv4
	v4Different := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}
	report2, err := exp.Write(context.Background(), v4Different, v6)
	if err != nil {
		t.Fatalf("Write() #2 failed: %v", err)
	}

	// Unchanged should be false because IPv4 changed
	if report2.Unchanged {
		t.Error("Unchanged should be false when one family changes")
	}

	// But IPv6 digest should match
	if report2.IPv6Digest != report1.IPv6Digest {
		t.Error("IPv6 digest should match when only IPv4 changed")
	}
}

func TestNewError(t *testing.T) {
	// Create a file and try to use it as a directory parent — MkdirAll should fail
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "block")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := New(filepath.Join(blockingFile, "subdir"))
	if err == nil {
		t.Error("New() with invalid path should fail")
	}
}

func TestWriteFamilyCreateTempError(t *testing.T) {
	// Use a file as baseDir so os.CreateTemp fails
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "block")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	e := &FileExporter{baseDir: blockingFile}
	_, _, err := e.writeFamily(context.Background(), "ipv4", filepath.Join(tmpDir, "out.txt"), nil)
	if err == nil {
		t.Error("writeFamily with invalid baseDir should fail")
	}
}

func TestFsyncDirError(t *testing.T) {
	err := fsyncDir("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("fsyncDir with non-existent path should fail")
	}
}
