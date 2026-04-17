# Agent 14 — Property-Based Testing (CIDR Aggregator)

**Model:** Opus (complex algorithm testing)  
**Priority:** 🟡 LOW  
**Effort:** 8 hours  
**Iteration:** 9

## Goal

Add property-based testing (PBT) to the CIDR aggregator using `pgregory.net/rapid` to find edge cases in aggregation logic that unit tests miss.

## Background

**Current testing (example-based):**
```go
// pkg/cidr/aggregate_test.go (CURRENT)
func TestAggregateConservative(t *testing.T) {
    input := []netip.Prefix{
        netip.MustParsePrefix("192.0.2.0/25"),
        netip.MustParsePrefix("192.0.2.128/25"),
    }
    
    result := Aggregate(input, "conservative")
    
    want := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
    // ... assert equality ...
}
```

**Problem:** Only tests specific examples. May miss edge cases:
- Overlapping prefixes
- Degenerate inputs (single IP, /0, /128)
- Order sensitivity
- Empty inputs
- Maximum prefix length violations
- IPv4 vs IPv6 mixing

**Solution:** Property-based testing generates thousands of random inputs and checks invariants.

## Property-Based Testing Concept

Instead of "input X should produce output Y", we test **properties** that hold for **all** inputs:

**Properties of CIDR aggregation:**
1. **Lossless:** Every IP in input is covered by output
2. **No overlap:** Output prefixes don't overlap each other
3. **Minimal:** No two output prefixes can be merged further (given aggressiveness level)
4. **Order-independent:** Sorting input shouldn't change output (modulo order)
5. **Idempotent:** Aggregating twice gives same result
6. **Subset:** Output is subset of input (no new IPs introduced)

## Files Involved

### New Files
- `pkg/cidr/aggregate_rapid_test.go` — Property-based tests

### Modified Files
- `go.mod` / `go.sum` — Add `pgregory.net/rapid` dependency

## Requirements

### 1. Add Dependency

```bash
go get pgregory.net/rapid@latest
```

### 2. Write Property Tests

```go
// pkg/cidr/aggregate_rapid_test.go (NEW)

package cidr

import (
    "net/netip"
    "slices"
    "testing"
    
    "pgregory.net/rapid"
)

// Generators

// genIPv4Prefix generates random IPv4 prefixes with valid prefix lengths
func genIPv4Prefix() *rapid.Generator[netip.Prefix] {
    return rapid.Custom(func(t *rapid.T) netip.Prefix {
        // Random IPv4 address (4 bytes)
        bytes := [4]byte{
            rapid.Byte().Draw(t, "b0"),
            rapid.Byte().Draw(t, "b1"),
            rapid.Byte().Draw(t, "b2"),
            rapid.Byte().Draw(t, "b3"),
        }
        addr := netip.AddrFrom4(bytes)
        
        // Random prefix length (0-32)
        bits := rapid.IntRange(0, 32).Draw(t, "bits")
        
        prefix := netip.PrefixFrom(addr, bits)
        return prefix.Masked() // Normalize (zero host bits)
    })
}

// genIPv6Prefix generates random IPv6 prefixes with valid prefix lengths
func genIPv6Prefix() *rapid.Generator[netip.Prefix] {
    return rapid.Custom(func(t *rapid.T) netip.Prefix {
        // Random IPv6 address (16 bytes)
        bytes := [16]byte{}
        for i := 0; i < 16; i++ {
            bytes[i] = rapid.Byte().Draw(t, "b"+string(rune(i)))
        }
        addr := netip.AddrFrom16(bytes)
        
        // Random prefix length (0-128)
        bits := rapid.IntRange(0, 128).Draw(t, "bits")
        
        prefix := netip.PrefixFrom(addr, bits)
        return prefix.Masked() // Normalize
    })
}

// genPrefixList generates a list of random prefixes (IPv4 or IPv6)
func genPrefixList(ipv4 bool, maxLen int) *rapid.Generator[[]netip.Prefix] {
    gen := genIPv4Prefix()
    if !ipv4 {
        gen = genIPv6Prefix()
    }
    
    return rapid.SliceOfN(gen, 0, maxLen)
}

// Property Tests

// TestAggregate_Lossless verifies no IPs are lost during aggregation
func TestAggregate_Lossless(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Generate random IPv4 prefix list
        input := genPrefixList(true, 100).Draw(t, "input")
        
        // Skip empty input (trivial case)
        if len(input) == 0 {
            return
        }
        
        // Aggregate
        output := Aggregate(input, "conservative")
        
        // Property: Every IP in input is covered by output
        for _, inPrefix := range input {
            covered := false
            for _, outPrefix := range output {
                if outPrefix.Contains(inPrefix.Addr()) || outPrefix.Overlaps(inPrefix) {
                    covered = true
                    break
                }
            }
            if !covered {
                t.Fatalf("Input prefix %s not covered by output %v", inPrefix, output)
            }
        }
    })
}

// TestAggregate_NoOverlap verifies output prefixes don't overlap
func TestAggregate_NoOverlap(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        input := genPrefixList(true, 100).Draw(t, "input")
        
        if len(input) == 0 {
            return
        }
        
        output := Aggregate(input, "conservative")
        
        // Property: No two output prefixes overlap
        for i := 0; i < len(output); i++ {
            for j := i + 1; j < len(output); j++ {
                if output[i].Overlaps(output[j]) {
                    t.Fatalf("Output prefixes overlap: %s and %s", output[i], output[j])
                }
            }
        }
    })
}

// TestAggregate_OrderIndependent verifies sorting input doesn't change result
func TestAggregate_OrderIndependent(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        input := genPrefixList(true, 50).Draw(t, "input")
        
        if len(input) < 2 {
            return // Need at least 2 elements to test order
        }
        
        // Aggregate original order
        output1 := Aggregate(input, "conservative")
        
        // Shuffle input
        shuffled := make([]netip.Prefix, len(input))
        copy(shuffled, input)
        rapid.SliceOf(genIPv4Prefix()).Draw(t, "shuffle") // Trigger randomness
        for i := len(shuffled) - 1; i > 0; i-- {
            j := rapid.IntRange(0, i).Draw(t, "j")
            shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
        }
        
        // Aggregate shuffled
        output2 := Aggregate(shuffled, "conservative")
        
        // Property: Results should be equivalent (same set, possibly different order)
        if !prefixSetsEqual(output1, output2) {
            t.Fatalf("Order-dependent aggregation:\nOriginal: %v\nShuffled: %v", output1, output2)
        }
    })
}

// TestAggregate_Idempotent verifies aggregating twice gives same result
func TestAggregate_Idempotent(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        input := genPrefixList(true, 100).Draw(t, "input")
        
        if len(input) == 0 {
            return
        }
        
        // Aggregate once
        output1 := Aggregate(input, "conservative")
        
        // Aggregate again
        output2 := Aggregate(output1, "conservative")
        
        // Property: Second aggregation should be no-op
        if !prefixSetsEqual(output1, output2) {
            t.Fatalf("Not idempotent:\nFirst:  %v\nSecond: %v", output1, output2)
        }
    })
}

// TestAggregate_NoNewIPs verifies no IPs are added (output is subset of input)
func TestAggregate_NoNewIPs(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        input := genPrefixList(true, 100).Draw(t, "input")
        
        if len(input) == 0 {
            return
        }
        
        output := Aggregate(input, "conservative")
        
        // Property: Every IP in output was covered by input
        for _, outPrefix := range output {
            // Sample some IPs from output prefix
            addr := outPrefix.Addr()
            
            covered := false
            for _, inPrefix := range input {
                if inPrefix.Contains(addr) {
                    covered = true
                    break
                }
            }
            if !covered {
                t.Fatalf("Output prefix %s contains IP %s not in input", outPrefix, addr)
            }
        }
    })
}

// TestAggregate_IPv6 tests properties for IPv6
func TestAggregate_IPv6_Lossless(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        input := genPrefixList(false, 100).Draw(t, "input")
        
        if len(input) == 0 {
            return
        }
        
        output := Aggregate(input, "conservative")
        
        // Property: Lossless for IPv6
        for _, inPrefix := range input {
            covered := false
            for _, outPrefix := range output {
                if outPrefix.Contains(inPrefix.Addr()) || outPrefix.Overlaps(inPrefix) {
                    covered = true
                    break
                }
            }
            if !covered {
                t.Fatalf("IPv6 input prefix %s not covered by output %v", inPrefix, output)
            }
        }
    })
}

// Helper: Check if two prefix sets are equal (ignoring order)
func prefixSetsEqual(a, b []netip.Prefix) bool {
    if len(a) != len(b) {
        return false
    }
    
    // Sort both slices for comparison
    sortedA := make([]netip.Prefix, len(a))
    sortedB := make([]netip.Prefix, len(b))
    copy(sortedA, a)
    copy(sortedB, b)
    
    slices.SortFunc(sortedA, func(x, y netip.Prefix) int {
        return x.Addr().Compare(y.Addr())
    })
    slices.SortFunc(sortedB, func(x, y netip.Prefix) int {
        return x.Addr().Compare(y.Addr())
    })
    
    for i := range sortedA {
        if sortedA[i] != sortedB[i] {
            return false
        }
    }
    
    return true
}
```

### 3. Run Property Tests

```bash
# Run property tests (rapid will generate 100 random inputs per test by default)
go test -v ./pkg/cidr -run Rapid

# Run with more iterations (1000 random inputs)
go test -v ./pkg/cidr -run Rapid -rapid.checks=1000

# Run with specific seed (for reproducibility)
go test -v ./pkg/cidr -run Rapid -rapid.seed=12345

# Run with verbose output (see generated inputs)
go test -v ./pkg/cidr -run Rapid -rapid.v
```

### 4. Minimize Failing Inputs

If a property test fails, `rapid` will automatically minimize the failing input to the smallest example:

```
--- FAIL: TestAggregate_Lossless (0.05s)
    aggregate_rapid_test.go:50: Failed after 23 iterations
    
    Minimal failing input:
        input = [192.0.2.0/32, 192.0.2.0/31]
    
    Error: Input prefix 192.0.2.0/32 not covered by output [192.0.2.0/31]
```

This helps debug the root cause (overlap handling, prefix merging logic, etc.).

## Acceptance Criteria

- [ ] `pgregory.net/rapid` dependency added
- [ ] 5+ property tests implemented (lossless, no-overlap, idempotent, order-independent, subset)
- [ ] Tests pass for IPv4 and IPv6
- [ ] Tests run in CI (add to Makefile and GitHub Actions)
- [ ] Documentation explains what each property tests
- [ ] Failures provide minimal failing examples (rapid's shrinking works)
- [ ] All existing unit tests still pass

## Properties to Test

1. **Lossless:** Every IP in input is covered by output
2. **No overlap:** Output prefixes are disjoint
3. **Minimal:** No two output prefixes can be merged (given aggressiveness)
4. **Order-independent:** Sorting input doesn't change output set
5. **Idempotent:** `Aggregate(Aggregate(x))` = `Aggregate(x)`
6. **Subset:** Output is subset of input (no new IPs)
7. **Empty input:** `Aggregate([])` = `[]`
8. **Single input:** `Aggregate([x])` = `[x]`

## Edge Cases to Cover

- Empty prefix list
- Single prefix
- Overlapping prefixes (A contains B)
- Adjacent prefixes (192.0.2.0/25 + 192.0.2.128/25 → 192.0.2.0/24)
- Non-adjacent prefixes (no merging possible)
- Maximum prefix length (v4_max_prefix, v6_max_prefix enforcement)
- Degenerate inputs (/0, /32, /128)
- Mixed order inputs (unsorted, reverse sorted)

## Non-Goals

- Fuzzing (use `go test -fuzz` instead, separate effort)
- Performance benchmarking (use `go test -bench` instead)
- Correctness proofs (PBT finds bugs, doesn't prove absence)

## Testing Strategy

1. **Run rapid tests locally** with 1000 iterations
2. **Add to CI** (GitHub Actions: `go test -run Rapid -rapid.checks=100`)
3. **Document failures** if any property fails, investigate root cause
4. **Fix bugs** in aggregator if properties violated
5. **Add regression tests** for any bugs found by rapid

## Expected Outcome

**Scenario 1: All properties pass**
```bash
$ go test -v ./pkg/cidr -run Rapid -rapid.checks=1000
=== RUN   TestAggregate_Lossless
--- PASS: TestAggregate_Lossless (0.12s)
=== RUN   TestAggregate_NoOverlap
--- PASS: TestAggregate_NoOverlap (0.10s)
=== RUN   TestAggregate_OrderIndependent
--- PASS: TestAggregate_OrderIndependent (0.15s)
=== RUN   TestAggregate_Idempotent
--- PASS: TestAggregate_Idempotent (0.11s)
=== RUN   TestAggregate_NoNewIPs
--- PASS: TestAggregate_NoNewIPs (0.13s)
=== RUN   TestAggregate_IPv6_Lossless
--- PASS: TestAggregate_IPv6_Lossless (0.14s)
PASS
ok      github.com/yourusername/d2ip/pkg/cidr    0.750s
```

**Scenario 2: Property fails (bug found)**
```bash
$ go test -v ./pkg/cidr -run Rapid
=== RUN   TestAggregate_Lossless
    aggregate_rapid_test.go:50: Failed after 23 iterations
    
    Minimal failing input:
        input = [192.0.2.0/31, 192.0.2.1/32]
    
    Error: Input prefix 192.0.2.1/32 not covered by output [192.0.2.0/31]
--- FAIL: TestAggregate_Lossless (0.05s)
FAIL
```

→ Investigate why `192.0.2.1/32` (subset of `192.0.2.0/31`) is missing in output. Likely bug in overlap detection.

## Deliverables

1. **Property tests** (`pkg/cidr/aggregate_rapid_test.go`)
2. **Dependency** (`pgregory.net/rapid` in `go.mod`)
3. **CI integration** (Makefile + GitHub Actions)
4. **Documentation** (comments explaining each property)
5. **Bug fixes** (if any property fails)

## Success Metrics

- ✅ 5+ properties tested
- ✅ 1000+ random inputs generated per property
- ✅ All properties pass (or bugs found and fixed)
- ✅ CI runs property tests on every PR
- ✅ Minimal failing examples help debug issues

## Resources

- **rapid documentation:** https://pkg.go.dev/pgregory.net/rapid
- **PBT introduction:** https://increment.com/testing/in-praise-of-property-based-testing/
- **Go PBT examples:** https://github.com/flyingmutant/rapid/tree/master/examples
