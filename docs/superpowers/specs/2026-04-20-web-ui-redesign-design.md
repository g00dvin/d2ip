# Web UI Redesign — Design Spec

**Date:** 2026-04-20
**Status:** Draft

## Overview

Redesign and expand the d2ip Web UI from a basic monitoring dashboard into a full-featured management interface covering configuration, pipeline control, category browsing, cache management, source info, and routing control.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| UI Framework | HTMX + vanilla JS (no Alpine.js) | Consistent with existing codebase, minimal dependencies, sufficient for all features |
| Styling | Vanilla CSS, terminal-inspired theme | No build step, embedded naturally, fits the tool's CLI nature |
| Layout | Fixed sidebar (180px) + scrollable content | Classic admin layout, works on all screen sizes |
| Authentication | None | Accessible to anyone on the listen address |
| Server rendering | HTML fragments via HTMX | No SPA complexity, clean Go template code |
| Color palette | Cold colors (steel blues, slate grays) | Professional, low eye strain, no neon |

## Architecture

### Frontend

**Technology:** HTMX 1.9.10 (CDN), vanilla CSS, Go `html/template`

**Structure:**
```
internal/api/web/
├── index.html          # Shell: sidebar + content area placeholder
├── styles.css          # All CSS variables + component styles
└── templates/          # Go HTML templates for each section
    ├── dashboard.html
    ├── pipeline.html
    ├── config.html
    ├── categories.html
    ├── cache.html
    ├── source.html
    └── routing.html
```

**Navigation:** Sidebar links use `hx-get` to load section templates into the content area. Active section highlighted in accent color.

### Backend

**New API endpoints** (all under `/api/` prefix, returning JSON for HTMX consumption):

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/settings` | Get all config + defaults + overrides |
| PUT | `/api/settings` | Set config overrides (kv_settings) |
| DELETE | `/api/settings/{key}` | Reset field to default |
| GET | `/api/pipeline/history` | Last 10 pipeline runs |
| POST | `/pipeline/cancel` | Cancel running pipeline |
| GET | `/api/categories` | List categories with domain counts |
| GET | `/api/categories/{code}/domains` | Paginated domain list for category |
| POST | `/api/categories` | Add new category |
| DELETE | `/api/categories/{code}` | Remove category |
| GET | `/api/cache/stats` | Cache statistics |
| POST | `/api/cache/purge` | Purge entries by pattern/age/failed |
| POST | `/api/cache/vacuum` | Run SQLite VACUUM |
| GET | `/api/cache/entries?domain=` | Search cached entries |
| GET | `/api/source/info` | Source metadata (URL, ETag, size, checksum) |
| POST | `/routing/apply` | Apply current routing plan |

**Existing endpoints reused:**
- `GET /healthz` — health status
- `POST /pipeline/run` — trigger pipeline
- `GET /pipeline/status` — current run status
- `POST /routing/dry-run` — preview routing changes
- `POST /routing/rollback` — revert routing
- `GET /routing/snapshot` — current applied state

### Template Rendering

Each section is a Go `html/template` that renders an HTML fragment. HTMX requests target these fragments. The shell (`index.html`) contains the sidebar and a `<div id="content">` that gets replaced.

**Template data:** Each template receives a struct with the data it needs (config snapshot, pipeline status, etc.). Templates are parsed once at server startup and cached.

## UI Sections

### Dashboard (Landing Page)

**Purpose:** System at-a-glance status.

**Content:**
- System status: health, uptime, last pipeline run timestamp
- Quick actions: Run Pipeline, Force Resolve, Dry Run (routing)
- Last run summary: domains processed, resolved, failed, duration, IPv4/IPv6 prefix counts
- Routing state: backend type, applied timestamp, prefix counts (v4/v6)
- Config summary: listen address, active categories count, resolver upstream

**Auto-refresh:** Polls `/pipeline/status` and `/healthz` every 10s via HTMX `hx-trigger`.

### Pipeline

**Purpose:** Control and monitor pipeline execution.

**Content:**
- Trigger controls: Run Pipeline, Force Resolve, Dry Run buttons
- Current run status: if running — current step, progress bar, elapsed time, cancel button
- Run history table: last 10 runs (ID, timestamp, duration, domains, resolved, failed, status)
- Run detail: click a row to expand full PipelineReport

**Auto-refresh:** Polls every 2s while a run is active. History table is static until manually refreshed.

### Config

**Purpose:** View and edit all configuration fields.

**Content:**
- All config fields grouped by section: Source, Categories, Resolver, Cache, Aggregation, Export, Routing, Scheduler, Logging, Metrics
- Each field rendered as appropriate input type (text, number, select, duration)
- Validation: server-side on save, client-side format hints
- Save: writes to kv_settings, triggers hot-reload
- Reset: button per field to revert to default
- Diff view: highlights fields that differ from defaults

**Interaction:** Form submits via HTMX `hx-put`. Validation errors shown inline. No restart needed — config reloads via Watcher pub/sub.

### Categories

**Purpose:** Browse and manage geosite categories.

**Content:**
- Category list table: code, description, domain count, @attribute filter
- Browse category: click to see domains in paginated table
- Search/filter: text input to filter domains within a category (client-side)
- Add category: form with code input, optional @attribute filter, validation
- Remove category: delete button per row, confirmation prompt

**Interaction:** HTMX loads domain list on category click. Search filters client-side via vanilla JS. Add/remove triggers config save + hot-reload.

### Cache

**Purpose:** Monitor and manage SQLite DNS cache.

**Content:**
- Stats panel: total entries, IPv4 count, IPv6 count, DB size, oldest/newest entry
- Purge controls: by domain pattern (wildcard), by age, all failed entries
- Vacuum: button to run SQLite VACUUM, shows last vacuum timestamp
- Entry browser: search by domain, see cached IPs, TTL, last resolved timestamp

**Auto-refresh:** Stats every 30s. Purge/vacuum are POST actions with confirmation.

### Source

**Purpose:** View and manage dlc.dat source.

**Content:**
- Source info: URL, local cache path, last fetch timestamp, ETag, file size
- Checksum: SHA256 of current cached `dlc.dat`
- Refresh: button to force re-fetch (triggers full pipeline)

**Interaction:** Info loads on page visit. Refresh triggers pipeline run.

### Routing

**Purpose:** Control and inspect routing state.

**Content:**
- State display: backend type, last applied timestamp, IPv4/IPv6 prefix counts
- Dry run: input for prefixes, shows diff preview without applying
- Apply: button to apply current routing plan
- Rollback: button to revert, with confirmation
- Snapshot: raw view of applied prefixes (paginated, filterable by family)

**Auto-refresh:** Snapshot every 30s. Dry run shows terminal-style diff output.

## Color Palette

| Token | Value | Usage |
|-------|-------|-------|
| `--bg-primary` | `#0f1923` | Main content background |
| `--bg-sidebar` | `#1a2332` | Sidebar background |
| `--bg-card` | `#1a2332` | Card/panel backgrounds |
| `--border` | `#2a3a4a` | Borders, dividers |
| `--border-light` | `#3a4a5a` | Hover states, subtle borders |
| `--text-primary` | `#b0bec5` | Main text |
| `--text-secondary` | `#6a7a8a` | Labels, secondary text |
| `--text-muted` | `#4a5a6a` | Version info, hints |
| `--accent` | `#5dade2` | Active nav, links, primary buttons |
| `--accent-hover` | `#3498db` | Button hover |
| `--success` | `#4caf80` | Healthy status, success messages |
| `--warning` | `#e0a050` | Warnings, force actions |
| `--error` | `#e74c3c` | Errors, failed status |
| `--code-bg` | `#0a1018` | Code/command blocks |

## Typography

- Font family: `monospace` (system monospace stack)
- Base size: 13px
- Heading sizes: h2=16px, h3=14px
- Line height: 1.5

## Component Styles

### Buttons
- Outlined, no fill
- Border color = text color
- Hover: background fills with border color, text inverts to bg
- Sizes: small (padding 4px 8px), normal (6px 12px)
- Variants: default (accent), success (green), warning (amber), danger (red)

### Cards/Panels
- Border: 1px solid `--border`
- Padding: 12px
- Margin-bottom: 12px
- Optional header: uppercase label in `--text-secondary`

### Tables
- Border-collapse, full width
- Header: `--text-secondary`, uppercase, smaller font
- Rows: alternating subtle background
- Hover: `--border-light` bottom border
- Clickable rows: cursor pointer, accent hover

### Forms
- Inputs: dark background, light border, monospace text
- Focus: accent border
- Labels: `--text-secondary`, uppercase, small
- Errors: `--error` text below input
- Selects: styled to match inputs

### Status Indicators
- Dot + text: `● healthy` (green), `● unhealthy` (red), `● running` (amber)
- Pulsing animation for active states

## Error Handling

- **API errors:** Return JSON with `{"error": "message"}`, HTMX displays in error region
- **Validation errors:** Inline below form fields, red text
- **Network errors:** HTMX `htmx:responseError` handler shows toast notification
- **Pipeline errors:** Shown in run history with red status, expandable for details

## Data Flow

```
Browser → HTMX request → Go handler → Orchestrator/Config/Cache → JSON/HTML → Browser
```

- Config reads use `Watcher.Snapshot()` for thread-safe access
- Config writes use `KVStore.Set()` → triggers hot-reload
- Pipeline triggers use `Orchestrator.Run()` with context
- Cache operations use the cache agent directly
- Category data comes from the domainlist agent's parsed protobuf

## Testing

- **Web embed test:** Verify embedded files are under size limit
- **API endpoint tests:** Unit tests for each new handler
- **Template tests:** Verify templates render without errors
- **Integration tests:** HTMX flows tested via existing pipeline tests

## Constraints

- Web UI embedded in binary — changes require rebuild
- HTMX from CDN — offline use requires bundling (not planned)
- No server-side templating currently — adding Go `html/template`
- Config `listen` address changes require restart (documented limitation)
- No authentication — accessible to anyone on the listen address
