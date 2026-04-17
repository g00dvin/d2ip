# Web UI Documentation

## Overview

The d2ip web UI is a minimal, mobile-friendly interface built with HTMX for controlling and monitoring the pipeline.

**Access:** http://localhost:9099/

**Technology Stack:**
- HTMX 1.9.10 (from CDN)
- Vanilla CSS (no frameworks)
- Embedded in binary via Go `embed` package
- Total size: ~17KB

## Features

### 1. Header with Health Status

**Location:** Top of page

**Components:**
- **Logo:** "d2ip" branding with subtitle "Domain to IP Resolver"
- **Status Indicator:** Real-time health check
  - Green (● Healthy) when /healthz returns 200
  - Red (● Unhealthy) when /healthz fails
  - Auto-refreshes every 10 seconds

### 2. Pipeline Control Section

**Purpose:** Trigger and monitor pipeline runs

**Components:**
- **Trigger Button:** "▶ Trigger Pipeline" button (blue, primary)
  - POST to /pipeline/run
  - Immediately refreshes status display
- **Status Display:** Shows current or last run status
  - **While Running:** Blue box with Run ID and start time
  - **After Completion:** Green box (success) or Yellow box (warnings)
    - Run ID
    - Start time
    - Duration (seconds)
    - Domains processed
    - Resolved count
    - Failed count
    - IPv4/IPv6 output counts
  - Auto-refreshes every 5 seconds

### 3. Routing Control Section

**Purpose:** Preview and manage routing table changes

**Components:**
- **Dry Run Button:** "🔍 Dry Run" (gray, secondary)
  - POST to /routing/dry-run with empty prefixes
  - Shows planned changes (add/remove counts)
  - Does not apply changes
- **Rollback Button:** "↩ Rollback" (red, danger)
  - POST to /routing/rollback
  - Confirmation dialog before execution
  - Restores previous routing state
- **Result Display:** Shows dry-run or rollback outcomes
  - IPv4/IPv6 prefix add/remove counts
  - Success/error messages
- **Current Snapshot:** Auto-refreshing display (every 30s)
  - Backend type (nftables/iproute2/none)
  - Last applied timestamp
  - IPv4 prefix count
  - IPv6 prefix count

### 4. Configuration Section (Placeholder)

**Status:** Coming soon

**Purpose:** Will integrate with kv_settings for runtime config management

### 5. Footer Links

**Links:**
- **Prometheus Metrics:** Opens /metrics in new tab
- **Documentation:** Links to GitHub repository

## Auto-Refresh Behavior

The UI uses HTMX polling for real-time updates:

| Element | Endpoint | Interval | Trigger |
|---------|----------|----------|---------|
| Health status | /healthz | 10s | load, timer |
| Pipeline status | /pipeline/status | 5s | load, timer, manual |
| Routing snapshot | /routing/snapshot | 30s | load, timer |

**Manual triggers:**
- Clicking "Trigger Pipeline" refreshes pipeline status immediately
- Successful rollback refreshes routing snapshot immediately

## Responsive Design

**Desktop (>768px):**
- Two-column grid layouts for status displays
- Side-by-side button groups
- Max width: 1200px, centered

**Mobile (≤768px):**
- Single-column layouts
- Stacked buttons
- Full-width cards
- Header logo and status indicator stack vertically

## Color Scheme

**Status Colors:**
- Green (#10b981): Success, healthy
- Blue (#3b82f6): Running, info
- Yellow (#f59e0b): Warning
- Red (#ef4444): Error, danger
- Gray (#6b7280): Secondary, muted

**UI Theme:**
- Background: Light gray (#f9fafb)
- Cards: White (#ffffff)
- Text: Dark gray (#111827)
- Borders: Light gray (#e5e7eb)
- Shadows: Subtle drop shadows for depth

## API Response Handling

The UI uses HTMX's `htmx:beforeSwap` event to transform JSON responses into formatted HTML:

### Pipeline Status Response
```json
{
  "running": false,
  "run_id": 123,
  "started": "2026-04-16T23:30:00Z",
  "report": {
    "run_id": 123,
    "domains": 1000,
    "stale": 50,
    "resolved": 950,
    "failed": 50,
    "ipv4_out": 500,
    "ipv6_out": 300,
    "duration": 45000000000
  }
}
```

Transformed into:
- Status box with color coding
- Grid layout of key metrics
- Duration converted to seconds

### Routing Snapshot Response
```json
{
  "backend": "nftables",
  "applied_at": "2026-04-16T23:35:00Z",
  "v4": ["10.0.0.0/8", "192.168.0.0/16"],
  "v6": ["2001:db8::/32"]
}
```

Transformed into:
- Backend and timestamp display
- Prefix counts (not full lists for performance)

## Error Handling

**API Errors:**
- Network failures: HTMX retries (built-in)
- HTTP 4xx/5xx: Red error alerts with message
- Routing disabled (503): "routing disabled" message

**Confirmation Dialogs:**
- Rollback button: `hx-confirm` attribute shows browser confirmation

## Performance

**Size:** 17KB total (HTML + CSS)
- index.html: ~10KB
- styles.css: ~7KB

**External Dependencies:** 1
- HTMX 1.9.10 from unpkg.com CDN

**Network Usage:**
- Initial page load: 17KB + HTMX (~14KB gzipped)
- Polling: Small JSON responses (< 1KB each)
- No images, no fonts, no JavaScript frameworks

## Browser Compatibility

**Tested:**
- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)
- Mobile browsers (iOS Safari, Chrome Mobile)

**Requirements:**
- JavaScript enabled (for HTMX)
- CSS Grid support (all modern browsers)
- Fetch API support (for HTMX)

## Development

**Files:**
- `/internal/api/web/index.html` - Main UI structure
- `/internal/api/web/styles.css` - Styling
- `/internal/api/api.go` - Static file serving (embed directive)
- `/internal/api/web_test.go` - Tests for embedded files

**Embedding:**
```go
//go:embed web
var webFS embed.FS
```

**Serving:**
- Root path `/` redirects to `/index.html`
- All web files served from `/web/*`
- API routes take precedence over static files
