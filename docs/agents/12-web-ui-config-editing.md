# Agent 12 — Web UI Config Editing

**Model:** Opus (security-sensitive: auth + kv_settings mutation)  
**Priority:** 🟠 MEDIUM  
**Effort:** 12 hours  
**Iteration:** 8

## Goal

Add config editing interface to the web UI, allowing runtime configuration changes via `kv_settings` table without server restart. Includes simple password authentication.

## Background

**Current state:**
- Web UI is read-only ([internal/api/web/index.html](internal/api/web/index.html))
- Config precedence: ENV > kv_settings > YAML > defaults
- kv_settings backend ready but unused (SQLite table for runtime overrides)
- Hot-reload via Watcher broadcasts changes to subscribers

**Goal:** Add config editing form that writes to kv_settings table, allowing runtime overrides without restart.

## Security Requirements

⚠️ **CRITICAL:** Config editing is a privileged operation. **Must have authentication.**

**Simple password auth approach:**
```bash
# Set password via ENV var
export D2IP_WEB_PASSWORD="your-secure-password"
```

**Flow:**
1. User visits `/config/edit` → 401 if not authenticated
2. User enters password in login form → session cookie set (secure, httpOnly)
3. User edits config → POST validates session → writes to kv_settings
4. Config watcher broadcasts change → all components reload

**Session management:**
- Use Go's `gorilla/sessions` or similar
- Session cookie: secure flag (HTTPS only in production), httpOnly, SameSite=Strict
- Session timeout: 1 hour
- Logout endpoint: `/config/logout`

## Files Involved

### New Files
- `internal/api/web/config_edit.html` — Config editing form
- `internal/api/web/login.html` — Password login form
- `internal/api/auth.go` — Authentication middleware
- `internal/config/kv_backend.go` — kv_settings CRUD (if not exists)

### Modified Files
- `internal/api/api.go` — Add routes: `/config/edit`, `/config/login`, `/config/save`
- `internal/api/web/styles.css` — Form styling
- `go.mod` / `go.sum` — Add session library if needed

## Requirements

### 1. Authentication Middleware

```go
// internal/api/auth.go (NEW)

package api

import (
    "crypto/subtle"
    "net/http"
    "os"
)

type AuthMiddleware struct {
    password string
    sessions *SessionStore // e.g., gorilla/sessions
}

func NewAuthMiddleware() *AuthMiddleware {
    return &AuthMiddleware{
        password: os.Getenv("D2IP_WEB_PASSWORD"),
        sessions: newSessionStore(),
    }
}

func (a *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        session, _ := a.sessions.Get(r, "d2ip-session")
        
        if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
            http.Redirect(w, r, "/config/login", http.StatusFound)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

func (a *AuthMiddleware) Login(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        // Serve login form
        serveTemplate(w, "login.html", nil)
        return
    }
    
    password := r.FormValue("password")
    
    // Constant-time comparison to prevent timing attacks
    if subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) != 1 {
        http.Error(w, "Invalid password", http.StatusUnauthorized)
        return
    }
    
    session, _ := a.sessions.Get(r, "d2ip-session")
    session.Values["authenticated"] = true
    session.Options.MaxAge = 3600 // 1 hour
    session.Options.HttpOnly = true
    session.Options.SameSite = http.SameSiteStrictMode
    // session.Options.Secure = true  // Enable in production with HTTPS
    
    if err := session.Save(r, w); err != nil {
        http.Error(w, "Failed to save session", http.StatusInternalServerError)
        return
    }
    
    http.Redirect(w, r, "/config/edit", http.StatusFound)
}

func (a *AuthMiddleware) Logout(w http.ResponseWriter, r *http.Request) {
    session, _ := a.sessions.Get(r, "d2ip-session")
    session.Options.MaxAge = -1 // Delete session
    session.Save(r, w)
    http.Redirect(w, r, "/", http.StatusFound)
}
```

### 2. Config Editing Form

```html
<!-- internal/api/web/config_edit.html (NEW) -->

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>d2ip - Config Editor</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <link rel="stylesheet" href="/web/styles.css">
</head>
<body>
    <div class="container">
        <header>
            <h1>🛠️ Config Editor</h1>
            <a href="/config/logout" class="btn btn-secondary">Logout</a>
        </header>
        
        <form hx-post="/config/save" hx-swap="outerHTML">
            <div class="card">
                <h2>Resolver</h2>
                
                <label for="resolver_upstream">Upstream DNS</label>
                <input type="text" id="resolver_upstream" name="resolver.upstream" 
                       value="{{.Config.Resolver.Upstream}}" 
                       placeholder="1.1.1.1:53"
                       pattern="^[^:]+:\d+$"
                       required>
                <small>Format: IP:PORT (e.g., 8.8.8.8:53)</small>
                
                <label for="resolver_qps">QPS Limit</label>
                <input type="number" id="resolver_qps" name="resolver.qps" 
                       value="{{.Config.Resolver.QPS}}" 
                       min="1" max="10000"
                       required>
                
                <label for="resolver_concurrency">Concurrency</label>
                <input type="number" id="resolver_concurrency" name="resolver.concurrency" 
                       value="{{.Config.Resolver.Concurrency}}" 
                       min="1" max="1000"
                       required>
            </div>
            
            <div class="card">
                <h2>Scheduler</h2>
                
                <label for="scheduler_dlc_refresh">DLC Refresh Interval</label>
                <input type="text" id="scheduler_dlc_refresh" name="scheduler.dlc_refresh" 
                       value="{{.Config.Scheduler.DLCRefresh}}" 
                       placeholder="24h"
                       pattern="^\d+[smhd]$"
                       required>
                <small>Format: 30s, 5m, 2h, 1d</small>
                
                <label for="scheduler_resolve_cycle">Resolve Cycle</label>
                <input type="text" id="scheduler_resolve_cycle" name="scheduler.resolve_cycle" 
                       value="{{.Config.Scheduler.ResolveCycle}}" 
                       placeholder="0 (disabled)">
                <small>0 = disabled, or duration like 6h</small>
            </div>
            
            <div class="card">
                <h2>Routing</h2>
                
                <label for="routing_enabled">Enable Routing</label>
                <select id="routing_enabled" name="routing.enabled">
                    <option value="false" {{if not .Config.Routing.Enabled}}selected{{end}}>Disabled</option>
                    <option value="true" {{if .Config.Routing.Enabled}}selected{{end}}>Enabled</option>
                </select>
                
                <label for="routing_backend">Backend</label>
                <select id="routing_backend" name="routing.backend">
                    <option value="none" {{if eq .Config.Routing.Backend "none"}}selected{{end}}>None</option>
                    <option value="nftables" {{if eq .Config.Routing.Backend "nftables"}}selected{{end}}>nftables</option>
                    <option value="iproute2" {{if eq .Config.Routing.Backend "iproute2"}}selected{{end}}>iproute2</option>
                </select>
                
                <label for="routing_dry_run">Dry Run Mode</label>
                <select id="routing_dry_run" name="routing.dry_run">
                    <option value="false" {{if not .Config.Routing.DryRun}}selected{{end}}>Disabled</option>
                    <option value="true" {{if .Config.Routing.DryRun}}selected{{end}}>Enabled</option>
                </select>
            </div>
            
            <div class="card">
                <button type="submit" class="btn btn-primary">Save Changes</button>
                <button type="reset" class="btn btn-secondary">Reset</button>
            </div>
        </form>
        
        <div id="save-result"></div>
    </div>
</body>
</html>
```

### 3. kv_settings Backend

Ensure `kv_settings` table exists and has CRUD operations:

```go
// internal/config/kv_backend.go (NEW or ENHANCED)

package config

import (
    "database/sql"
    "fmt"
)

type KVBackend struct {
    db *sql.DB
}

func NewKVBackend(db *sql.DB) *KVBackend {
    return &KVBackend{db: db}
}

// Set writes a key-value pair to kv_settings table
func (kv *KVBackend) Set(key, value string) error {
    _, err := kv.db.Exec(`
        INSERT INTO kv_settings (key, value, updated_at)
        VALUES (?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(key) DO UPDATE SET
            value = excluded.value,
            updated_at = CURRENT_TIMESTAMP
    `, key, value)
    return err
}

// Get retrieves a value by key
func (kv *KVBackend) Get(key string) (string, error) {
    var value string
    err := kv.db.QueryRow(`SELECT value FROM kv_settings WHERE key = ?`, key).Scan(&value)
    if err == sql.ErrNoRows {
        return "", nil // Not found, use defaults
    }
    return value, err
}

// Delete removes a key (reverts to YAML/defaults)
func (kv *KVBackend) Delete(key string) error {
    _, err := kv.db.Exec(`DELETE FROM kv_settings WHERE key = ?`, key)
    return err
}

// All returns all key-value pairs
func (kv *KVBackend) All() (map[string]string, error) {
    rows, err := kv.db.Query(`SELECT key, value FROM kv_settings`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    result := make(map[string]string)
    for rows.Next() {
        var key, value string
        if err := rows.Scan(&key, &value); err != nil {
            return nil, err
        }
        result[key] = value
    }
    return result, rows.Err()
}
```

### 4. API Routes

```go
// internal/api/api.go (MODIFIED)

func (s *Server) Handler() http.Handler {
    r := chi.NewRouter()
    
    // ... existing middleware ...
    
    // Public routes
    r.Get("/", s.serveWebUI)
    r.Post("/pipeline/run", s.runPipeline)
    r.Get("/healthz", s.healthz)
    r.Get("/metrics", s.metrics)
    
    // Auth routes
    r.Get("/config/login", s.auth.Login)
    r.Post("/config/login", s.auth.Login)
    r.Get("/config/logout", s.auth.Logout)
    
    // Protected routes (require auth)
    r.Group(func(r chi.Router) {
        r.Use(s.auth.RequireAuth)
        
        r.Get("/config/edit", s.serveConfigEdit)
        r.Post("/config/save", s.saveConfig)
    })
    
    return r
}

func (s *Server) serveConfigEdit(w http.ResponseWriter, r *http.Request) {
    cfg := s.cfgGetter()
    
    data := struct {
        Config config.Config
    }{
        Config: cfg,
    }
    
    s.renderTemplate(w, "config_edit.html", data)
}

func (s *Server) saveConfig(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseForm(); err != nil {
        http.Error(w, "Invalid form data", http.StatusBadRequest)
        return
    }
    
    kv := s.kvBackend // Assume injected in Server struct
    
    // Save each form field to kv_settings
    for key, values := range r.Form {
        if len(values) == 0 {
            continue
        }
        value := values[0]
        
        // Convert form key (resolver.upstream) to ENV key (RESOLVER_UPSTREAM)
        kvKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
        
        if err := kv.Set(kvKey, value); err != nil {
            http.Error(w, fmt.Sprintf("Failed to save %s: %v", key, err), http.StatusInternalServerError)
            return
        }
    }
    
    // Trigger config reload
    s.watcher.Reload()
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(`<div class="alert alert-success">✅ Config saved successfully. Changes applied without restart.</div>`))
}
```

## Acceptance Criteria

- [ ] Password auth works (`D2IP_WEB_PASSWORD` ENV var)
- [ ] Login form renders at `/config/login`
- [ ] Unauthenticated users redirected to login
- [ ] Session cookie secure (httpOnly, SameSite, 1h timeout)
- [ ] Config editing form at `/config/edit` (authenticated only)
- [ ] Form validates inputs (regex for durations, IP:PORT, enums)
- [ ] Save writes to kv_settings table (not YAML file)
- [ ] Config watcher reloads after save (no restart needed)
- [ ] Logout endpoint clears session
- [ ] All tests still pass
- [ ] No XSS vulnerabilities (sanitize inputs, use HTMX safely)

## Security Checklist

- [ ] Password comparison uses `subtle.ConstantTimeCompare` (prevent timing attacks)
- [ ] Session cookie has `httpOnly` flag (prevent XSS session theft)
- [ ] Session cookie has `SameSite=Strict` (prevent CSRF)
- [ ] Session cookie has `Secure` flag (HTTPS only, documented for production)
- [ ] Input validation (duration regex, IP:PORT format, enum values)
- [ ] No SQL injection (use parameterized queries)
- [ ] No path traversal (static form fields only)
- [ ] Rate limiting considered (document reverse proxy requirement)

## Non-Goals

- OAuth/OIDC integration (too complex, simple password is enough)
- Multi-user support (single admin password is sufficient)
- Role-based access control (all-or-nothing access)
- HTTPS in application (document reverse proxy requirement)
- Password reset flow (admin resets via ENV var change)

## Testing Strategy

1. **Manual testing:** Set `D2IP_WEB_PASSWORD`, try login, edit config, verify kv_settings
2. **Auth tests:** Wrong password → 401, no session → redirect, valid session → access
3. **Save tests:** Mock kv_settings backend, verify writes
4. **Validation tests:** Invalid duration, invalid IP:PORT → error
5. **Integration:** Full flow (login → edit → save → reload → verify change)

## Deliverables

1. **Auth middleware** (`internal/api/auth.go`)
2. **Login form** (`internal/api/web/login.html`)
3. **Config edit form** (`internal/api/web/config_edit.html`)
4. **kv_settings backend** (`internal/config/kv_backend.go`)
5. **API routes** (modified `internal/api/api.go`)
6. **Tests** (auth, save, validation)
7. **Documentation** (`docs/WEB_UI.md` updated)

## Success Metrics

- ✅ Config editing works without restart
- ✅ Simple password auth protects sensitive endpoints
- ✅ No security vulnerabilities (XSS, CSRF, SQL injection)
- ✅ UX is smooth (HTMX live updates, clear error messages)
- ✅ Documented for production use (reverse proxy for HTTPS)
