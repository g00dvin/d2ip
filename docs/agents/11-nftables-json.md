# Agent 11 — nftables JSON Parsing

**Model:** Sonnet  
**Priority:** 🟠 MEDIUM  
**Effort:** 4-6 hours  
**Iteration:** 7b

## Goal

Replace brittle plain-text parsing of `nft list set` output with robust JSON parsing using `nft --json` mode, with fallback to plain text for older nftables versions.

## Background

Current implementation in `internal/routing/nftables.go` parses `nft list set` output as plain text:

```go
// internal/routing/nftables.go (CURRENT - BRITTLE)
func parseNftSet(output string, tableName, setName string) ([]string, error) {
    // Format: elements = { 192.0.2.0/24, ... }
    if strings.HasPrefix(line, "elements = {") {
        // Parse comma-separated list
    }
}
```

**Problems:**
- Fragile to nft output format changes
- Fails on edge cases (comments, formatting variations)
- Harder to maintain
- No structured error handling

**Better approach:** Use `nft --json` output mode with structured parsing.

## Files Involved

- **Parser:** `internal/routing/nftables.go` → `parseNftSet()` function
- **Backend:** `internal/routing/nftables.go` → `NftablesBackend` type
- **Tests:** `internal/routing/nftables_test.go` → parser tests
- **Integration:** `internal/routing/nftables_integration_test.go` (netns tests)

## Requirements

### 1. Add JSON Parsing

Create new `parseNftSetJSON()` function:

```go
// internal/routing/nftables.go (NEW)

type NftJSONOutput struct {
    Nftables []NftObject `json:"nftables"`
}

type NftObject struct {
    Set *NftSet `json:"set,omitempty"`
}

type NftSet struct {
    Family string   `json:"family"`
    Table  string   `json:"table"`
    Name   string   `json:"name"`
    Elem   []NftElem `json:"elem,omitempty"`
}

type NftElem struct {
    Prefix *NftPrefix `json:"prefix,omitempty"` // For CIDR
    Val    string     `json:"val,omitempty"`    // For single IP (unlikely)
}

type NftPrefix struct {
    Addr string `json:"addr"`
    Len  int    `json:"len"`
}

func parseNftSetJSON(jsonOutput string) ([]string, error) {
    var output NftJSONOutput
    if err := json.Unmarshal([]byte(jsonOutput), &output); err != nil {
        return nil, fmt.Errorf("unmarshal nft JSON: %w", err)
    }
    
    var prefixes []string
    for _, obj := range output.Nftables {
        if obj.Set != nil && obj.Set.Elem != nil {
            for _, elem := range obj.Set.Elem {
                if elem.Prefix != nil {
                    // Format: "192.0.2.0/24"
                    prefix := fmt.Sprintf("%s/%d", elem.Prefix.Addr, elem.Prefix.Len)
                    prefixes = append(prefixes, prefix)
                } else if elem.Val != "" {
                    // Single IP (convert to /32 or /128)
                    prefixes = append(prefixes, elem.Val)
                }
            }
        }
    }
    
    return prefixes, nil
}
```

### 2. Update Snapshot() to Use JSON

Modify `Snapshot()` method:

```go
// internal/routing/nftables.go (MODIFIED)

func (b *NftablesBackend) Snapshot() (*RoutingSnapshot, error) {
    // Try JSON first
    cmdJSON := exec.Command("nft", "--json", "list", "set", b.tableName, b.setNameV4)
    outputJSON, errJSON := cmdJSON.CombinedOutput()
    
    var v4Prefixes []string
    var err error
    
    if errJSON == nil && len(outputJSON) > 0 {
        // JSON mode succeeded
        v4Prefixes, err = parseNftSetJSON(string(outputJSON))
        if err != nil {
            // JSON parse failed, fallback to plain text
            output, _ := exec.Command("nft", "list", "set", b.tableName, b.setNameV4).CombinedOutput()
            v4Prefixes, err = parseNftSet(string(output), b.tableName, b.setNameV4)
        }
    } else {
        // nft --json not available, use plain text
        output, _ := exec.Command("nft", "list", "set", b.tableName, b.setNameV4).CombinedOutput()
        v4Prefixes, err = parseNftSet(string(output), b.tableName, b.setNameV4)
    }
    
    // Similar for IPv6...
    
    return &RoutingSnapshot{
        IPv4Prefixes: v4Prefixes,
        IPv6Prefixes: v6Prefixes,
    }, nil
}
```

### 3. Keep Plain-Text Fallback

**Important:** Don't delete the old `parseNftSet()` function. Keep it as fallback for:
- Older nftables versions without `--json` support
- Environments where JSON output is disabled
- Emergency recovery scenarios

### 4. Add Tests

Update `internal/routing/nftables_test.go`:

```go
func TestParseNftSetJSON(t *testing.T) {
    jsonOutput := `{
        "nftables": [
            {
                "set": {
                    "family": "inet",
                    "table": "d2ip",
                    "name": "d2ip_v4",
                    "elem": [
                        {"prefix": {"addr": "192.0.2.0", "len": 24}},
                        {"prefix": {"addr": "198.51.100.0", "len": 24}}
                    ]
                }
            }
        ]
    }`
    
    prefixes, err := parseNftSetJSON(jsonOutput)
    if err != nil {
        t.Fatalf("parseNftSetJSON: %v", err)
    }
    
    want := []string{"192.0.2.0/24", "198.51.100.0/24"}
    if !reflect.DeepEqual(prefixes, want) {
        t.Errorf("got %v, want %v", prefixes, want)
    }
}

func TestParseNftSetJSON_EmptySet(t *testing.T) {
    jsonOutput := `{"nftables": [{"set": {"family": "inet", "table": "d2ip", "name": "d2ip_v4"}}]}`
    
    prefixes, err := parseNftSetJSON(jsonOutput)
    if err != nil {
        t.Fatalf("parseNftSetJSON: %v", err)
    }
    
    if len(prefixes) != 0 {
        t.Errorf("expected empty list, got %v", prefixes)
    }
}

func TestParseNftSetJSON_InvalidJSON(t *testing.T) {
    _, err := parseNftSetJSON("not json")
    if err == nil {
        t.Error("expected error for invalid JSON")
    }
}
```

### 5. Integration Test

Ensure `internal/routing/nftables_integration_test.go` still passes:

```bash
sudo -E go test -tags=routing_integration ./internal/routing -v -run TestNftablesBackend
```

The integration tests create real nftables sets in isolated netns, so they'll validate that JSON parsing works with real kernel output.

## Acceptance Criteria

- [ ] JSON parsing implemented with correct structs
- [ ] `Snapshot()` uses JSON first, falls back to plain text
- [ ] Old `parseNftSet()` kept as fallback (not deleted)
- [ ] Unit tests for JSON parser (happy path + edge cases)
- [ ] All existing tests still pass (18 unit + 13 integration)
- [ ] No behavior change (output identical to plain-text parsing)
- [ ] Error messages clear when JSON parsing fails

## Non-Goals

- Remove plain-text parsing (keep as fallback)
- Support nftables features beyond sets (chains, rules, etc.)
- Optimize performance (JSON parsing is fast enough)
- Add JSON output to other nft commands (only `list set`)

## Testing Strategy

1. **Unit tests:** JSON parsing with mock data
2. **Fallback tests:** Verify plain-text used when JSON unavailable
3. **Integration tests:** Real nftables in netns (validates actual kernel JSON format)
4. **Error handling:** Invalid JSON, missing fields, empty sets

## Expected nftables JSON Format

Reference: `nft --json list set inet d2ip d2ip_v4`

```json
{
  "nftables": [
    {
      "metainfo": {
        "version": "1.0.2",
        "release_name": "Lester Gooch",
        "json_schema_version": 1
      }
    },
    {
      "set": {
        "family": "inet",
        "name": "d2ip_v4",
        "table": "d2ip",
        "type": "ipv4_addr",
        "flags": ["interval"],
        "elem": [
          {
            "prefix": {
              "addr": "192.0.2.0",
              "len": 24
            }
          },
          {
            "prefix": {
              "addr": "198.51.100.0",
              "len": 24
            }
          }
        ]
      }
    }
  ]
}
```

**Key fields:**
- `nftables[]` — top-level array
- `.set.elem[]` — array of elements
- `.prefix.addr` + `.prefix.len` → CIDR format

## Deliverables

1. **New JSON structs** in `nftables.go`
2. **parseNftSetJSON()** function
3. **Updated Snapshot()** with JSON-first logic
4. **Unit tests** for JSON parser
5. **All tests passing** (unit + integration)
6. **Brief documentation** of JSON format in code comments

## Success Metrics

- ✅ JSON parsing more robust than plain-text
- ✅ Fallback ensures compatibility with older nftables
- ✅ No behavior change (output identical)
- ✅ All 31 tests pass (18 unit + 13 integration)
- ✅ Code maintainability improved
