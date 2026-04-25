# Web UI Documentation

## Overview

The d2ip web UI is a modern, responsive single-page application (SPA) built with Vue 3 and Naive UI.

**Access:** http://localhost:9099/

**Technology Stack:**
- Vue 3 (Composition API with `<script setup>`)
- Naive UI (component library)
- Tailwind CSS v3 (utility classes for layout)
- Vue Router (hash mode)
- Pinia (state management)
- Axios (HTTP client)
- Embedded in binary via Go `embed` package
- Total size: ~468KB (gzipped embedded assets)

## Features

### 1. Dashboard

**Location:** Default landing page

**Components:**
- **Health Status:** Real-time health check indicator
  - Green (● Healthy) when /healthz returns 200
  - Red (● Unhealthy) when /healthz fails
  - Auto-refreshes every 10 seconds
- **Quick Actions:** Run pipeline, force resolve buttons
- **Last Run Summary:** Shows most recent pipeline results
- **Routing State:** Backend type, prefix counts, last applied timestamp
- **Warning Banner:** Appears when no categories are configured

### 2. Pipeline

**Purpose:** Trigger and monitor pipeline runs

**Components:**
- **Run Button:** Triggers pipeline execution (POST /pipeline/run)
- **Force Resolve Button:** Re-runs resolution even for fresh domains
- **Cancel Button:** Cancels running pipeline (with confirmation)
- **Status Display:** Live status with adaptive polling (2s when running, 10s idle)
- **Run History:** Table of past runs with metrics

### 3. Config

**Purpose:** View and edit configuration at runtime

**Components:**
- Dynamic form for all config sections
- Shows current values, defaults, and active overrides
- Save applies overrides via KV store with hot-reload

### 4. Sources

**Purpose:** Manage the multi-source registry

**Components:**
- **Source List Table:** ID, provider, prefix, enabled status, category count, health indicators
- **Health Indicators:**
  - Green "Healthy" tag when `last_fetched` is recent and `last_error` is empty
  - Red "Error" tag with tooltip showing `last_error` when source failed to load
  - "Last Fetched" displayed as relative time (e.g. "2 hours ago")
- **Add/Edit Source Drawer:** Form with provider type selector, prefix input, enabled toggle, provider-specific config fields
  - **v2flygeosite:** URL, cache path, refresh interval, HTTP timeout
  - **v2flygeoip:** URL, cache path, refresh interval, HTTP timeout
  - **ipverse:** Base URL template, country multi-select (50-country dropdown)
  - **mmdb:** File path or URL, country multi-select
  - **plaintext:** File upload (`.txt` only), type selector (domains/ips)
- **Per-Source Actions:** Refresh (manual reload), Edit, Delete
- **File Upload:** Drag-and-drop or click to upload plaintext files; saved to `/tmp/d2ip-uploads/{uuid}.txt`

### 5. Categories

**Purpose:** Manage categories from all loaded sources

**Components:**
- **Configured Categories:** Table with domain counts, browse/remove actions
- **Available Categories:** Searchable list grouped by source prefix (expandable `n-collapse` panels with count badges)
- **Domain Browser:** Expandable panel showing up to 500 domains per category
- **Prefix Grouping:** Categories are visually grouped by their source prefix (e.g. `geosite:`, `ipverse:`, `mmdb:`)

### 6. Cache

**Purpose:** Monitor cache health

**Components:**
- Statistics: domains, records (total/v4/v6), valid/failed counts
- Oldest/newest updated timestamps
- Vacuum action (with confirmation)

### 7. Source (Legacy)

**Purpose:** View legacy dlc.dat metadata (deprecated — use Sources page instead)

**Components:**
- SHA256 hash (truncated)
- Fetched timestamp
- File size
- ETag

### 8. Policies

**Purpose:** Manage multi-policy routing configuration

**Components:**
- **Policy List Table:** Name, backend, categories count, table/set, enabled status
- **Add/Edit Policy Drawer:** Form with category multi-select (grouped by source prefix), backend picker, backend-specific fields (table_id/iface for iproute2, nft_table/set for nftables), export format, dry-run toggle
- **Per-Policy Actions:** Run, dry-run, rollback, enable/disable, delete

### 9. Routing (Legacy)

**Purpose:** Preview and manage single-policy routing table changes (legacy mode)

**Components:**
- **Dry Run Button:** Shows planned changes without applying
- **Rollback Button:** Restores previous routing state (with confirmation)
- **Current Snapshot:** Backend type, IPv4/IPv6 prefix counts, applied timestamp

## Real-Time Updates

The UI uses **Server-Sent Events (SSE)** via `/events` endpoint for real-time updates:
- `pipeline.start` — Pipeline started
- `pipeline.progress` — Resolution progress (resolved/failed/total)
- `pipeline.complete` — Pipeline finished successfully
- `pipeline.failed` — Pipeline failed
- `config.reload` — Configuration changed
- `routing.apply` — Routing state applied

SSE connection shows a status indicator in the header. Falls back to polling if SSE disconnects.

## Auto-Refresh Behavior

Adaptive polling via `usePolling` composable (SSE primary, polling fallback):

| Element | Endpoint | Interval |
|---------|----------|----------|
| Health status | /healthz | 10s |
| Pipeline status (dashboard) | /pipeline/status | 10s |
| Pipeline status (pipeline page) | /pipeline/status | 2s (running) / 10s (idle) |
| Sources list | /api/sources | 30s |
| Policies list | /api/policies | 30s |
| Cache stats | /api/cache/stats | 30s |
| Routing snapshot | /routing/snapshot | 30s |

**Manual triggers:**
- Clicking "Run Pipeline" refreshes status immediately
- After save/rollback/vacuum, relevant sections refresh

## Responsive Design

**Desktop (>768px):**
- Fixed sidebar (180px) with navigation
- Main content area scrolls independently
- Card-based layout

**Mobile (≤768px):**
- Collapsible hamburger menu
- Overlay backdrop when sidebar open
- Full-width cards
- Single-column layout

## Themes

- **Dark theme:** Default (terminal-inspired dark blue-grey)
- **Light theme:** Toggle available
- Tailwind CSS `dark:` variants handle all theme switching

## Error Handling

**API Errors:**
- Network failures: Toast notification with error message
- HTTP 4xx/5xx: Toast with server error message
- Routing disabled (503): "routing disabled" message

**Confirmation Dialogs:**
- Cancel pipeline
- Vacuum cache
- Rollback routing
- Remove category

## Performance

**Size:** ~480KB total gzipped embedded assets (JS + CSS)
- Naive UI component library: ~425KB gzipped
- Tailwind CSS: ~3KB gzipped
- Application code: ~25KB gzipped (increased with multi-source UI features)
- Charts: ~25KB gzipped

**Network Usage:**
- Initial page load: 174KB (all assets embedded)
- Polling: Small JSON responses (< 1KB each)
- No external CDN dependencies

## Browser Compatibility

**Tested:**
- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)
- Mobile browsers (iOS Safari, Chrome Mobile)

**Requirements:**
- JavaScript enabled
- CSS Grid support
- Fetch API support

## Development

**Project Location:** `/web/`

**Key Files:**
- `web/src/main.ts` — App entry point
- `web/src/App.vue` — Root component
- `web/src/router/index.ts` — Route definitions
- `web/src/stores/` — Pinia stores
- `web/src/views/` — Page components
- `web/src/components/` — Reusable components
- `web/src/composables/` — Shared logic (polling, confirm)
- `web/src/api/rest.ts` — Typed REST API functions
- `web/src/api/types.ts` — API response interfaces
- `web/src/api/client.ts` — Axios instance with interceptors

**Build:**
```bash
cd web
npm ci
npm run build
```

**Output:** `web/dist/` → copied to `internal/api/web/`

**Embedding:**
```go
//go:embed web
var webFS embed.FS
```

**Serving:**
- Root path `/` serves `index.html`
- `/web/*` serves static assets
- API routes take precedence over static files
- SPA fallback handled by client-side router

## API Response Types

All API responses are typed in `web/src/api/index.ts`:

### Pipeline Status
```typescript
interface PipelineStatus {
  Running: boolean
  RunID: number
  Started: string
  Report: PipelineReport | null
}
```

### Routing Snapshot
```typescript
interface RoutingSnapshot {
  backend: string
  applied_at: string
  v4: string[]
  v6: string[]
}
```

### Cache Stats
```typescript
interface CacheStats {
  domains: number
  records_total: number
  records_v4: number
  records_v6: number
  records_valid: number
  records_failed: number
  records_nxdomain: number
  oldest_updated: number
  newest_updated: number
}
```