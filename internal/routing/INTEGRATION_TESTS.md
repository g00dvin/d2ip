# Routing Integration Tests

Integration tests for the routing agent that interact with the real Linux kernel (nftables/iproute2).

## Requirements

- **CAP_NET_ADMIN** capability (run with `sudo`)
- **Network namespace** support (`CONFIG_NET_NS`)
- **nftables** installed (for nftables backend tests)
- **iproute2** installed (for iproute2 backend tests)

## Running Tests

### With Docker (Recommended)

```bash
# Build dev image if not already built
make docker-dev

# Run integration tests in isolated container with NET_ADMIN capability
sudo -E docker run --rm --cap-add=NET_ADMIN \
    -v $(pwd):/work -w /work \
    d2ip-dev:latest \
    go test -v -tags=routing_integration ./internal/routing
```

### With Local Go (if Go 1.22+ installed)

```bash
# Run with sudo to get CAP_NET_ADMIN
sudo -E go test -v -tags=routing_integration ./internal/routing
```

### Skip Integration Tests

Integration tests are controlled by the `routing_integration` build tag. By default, they are **not** included:

```bash
# This will NOT run integration tests
go test ./internal/routing

# Explicitly skip integration tests
go test -short ./internal/routing
```

## Test Structure

### Files

- `netns_helper_test.go` — Helper functions for network namespace management
- `nftables_integration_test.go` — nftables backend integration tests
- `iproute2_integration_test.go` — iproute2 backend integration tests

### Test Cases

**Nftables Backend:**
1. Apply IPv4 prefixes
2. Verify sets created
3. Idempotence (second apply is no-op)
4. Update prefixes (add/remove)
5. Rollback (remove all)
6. Dry-run (preview without applying)

**Iproute2 Backend:**
1. Apply IPv4 routes
2. Verify routes created in kernel
3. Idempotence
4. Update routes
5. Apply IPv6 routes
6. Rollback
7. Dry-run

## Safety

All tests run in **isolated network namespaces** (`ip netns`), ensuring:
- No impact on host routing tables
- No risk of bricking host network
- Clean teardown even on test failure

Test namespaces:
- `d2ip-test-nft` — nftables tests
- `d2ip-test-ip` — iproute2 tests (with `dummy0` interface)

## Expected Output

```
=== RUN   TestNftablesBackend_Integration_RealKernel
--- PASS: TestNftablesBackend_Integration_RealKernel (0.15s)
    --- PASS: TestNftablesBackend_Integration_RealKernel/Apply_IPv4_Prefixes (0.03s)
    --- PASS: TestNftablesBackend_Integration_RealKernel/Verify_Sets_Created (0.00s)
    --- PASS: TestNftablesBackend_Integration_RealKernel/Idempotence_Second_Apply (0.01s)
    --- PASS: TestNftablesBackend_Integration_RealKernel/Update_Prefixes (0.02s)
    --- PASS: TestNftablesBackend_Integration_RealKernel/Rollback (0.01s)
    --- PASS: TestNftablesBackend_Integration_RealKernel/DryRun (0.01s)
...
PASS
```

## Troubleshooting

### "Skipping integration test: requires CAP_NET_ADMIN"

Run with `sudo` or grant `CAP_NET_ADMIN` to the test binary.

### "Skipping integration test: nftables not available"

Install nftables: `apt install nftables` (Debian/Ubuntu) or `apk add nftables` (Alpine).

### "Failed to create netns"

Check kernel support: `cat /boot/config-$(uname -r) | grep CONFIG_NET_NS` should show `CONFIG_NET_NS=y`.

### Tests fail with "permission denied"

Ensure you're running with `sudo -E` (the `-E` preserves environment variables like `GOPATH`).

## CI Integration

Add to `.github/workflows/test.yml`:

```yaml
- name: Run integration tests
  if: matrix.os == 'ubuntu-latest'
  run: |
    sudo -E go test -v -tags=routing_integration ./internal/routing
```

**Note:** Integration tests should run on **Linux only** (not macOS/Windows) and require elevated privileges.
