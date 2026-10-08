# GIT-OPS - Project State

## Overview
This document summarizes the development progress, architectural decisions, and features implemented in the GIT-OPS project (formerly GitLab Code Scanner).

## Architecture & Tech Stack
- **Backend:** Go (Golang)
- **Web Framework:** Gin (`github.com/gin-gonic/gin`)
- **Database:** PostgreSQL 18 (using `gorm.io/driver/postgres`)
- **Cache:** Redis (TTL-based caching via `go-redis`)
- **Load Balancer:** Nginx
- **Containerization:** Docker Compose
- **ORM:** GORM (`gorm.io/gorm`)
- **GitLab Integration:** Official Go GitLab Client (`gitlab.com/gitlab-org/api/client-go`)
- **PDF Generation:** gofpdf (`github.com/jung-kurt/gofpdf`)
- **Frontend:** Vanilla HTML, CSS, JavaScript (Single Page Application — Tool Hub pattern)

---

## Features Implemented

### 1. Authentication & Session Management
- Integrated **GitLab OAuth2 SSO** for secure user login.
- Implemented session cookies (`gin-contrib/sessions`) to persist user authentication.
- Added API endpoints for `/auth/login`, `/auth/callback`, `/auth/logout`, and `/auth/me`.

### 2. GitLab Code Scanning (`scanner.go`) — **Rewritten for Speed**
- **Strategy change:** Replaced GitLab's Elasticsearch blob search API (slow, requires Advanced Search) with a direct **file-tree walk + local grep** approach.
- **Two-level parallelism:**
  - `PROJECT_WORKERS` goroutines scan repositories concurrently (default: 4, env-configurable).
  - `FILE_WORKERS` goroutines fetch and scan files within each repo concurrently (default: 3, env-configurable).
- **BFS group traversal:** `collectGroupProjects()` performs a breadth-first search over all subgroups, deduplicating by project ID.
- **File fetch with fallback:** Primary path via GitLab Files API; falls back to raw blob SHA fetch for files >1 MB.
- **Binary file detection:** First 8 KB checked for null bytes; binary files silently skipped.
- **Branch resolution:** `resolveRef()` tries the target branch, falls back to the project's default branch.
- **Deep Linking:** URL-encoded deep links pointing directly to the exact line in GitLab.
- **Progress tracking:** `ScanProgress` struct with atomic counters (`ProjectsTotal`, `ProjectsScanned`, `TotalMatches`) for live status polling.

### 3. Structured Logging
- All scanner stages emit prefixed, emoji-tagged log lines for easy filtering:
  - `📂 [BFS]` — group/subgroup traversal progress
  - `🚀 [PROJECT]` — per-repo scan start with progress counter (e.g. `3/10`)
  - `📋 [PROJECT]` — file count and worker count before scanning
  - `✅ [PROJECT]` — per-repo completion with file/match counts and running total
  - `🎯 [MATCH]` — every keyword hit with file, line number, and project
  - `⏭️  [FILE]` — binary file skip notification
  - `⚠️  [FILE/PROJECT/BFS]` — fetch errors and warnings
  - `🔎 [SCAN]` — scan start summary (projects, workers, terms)
  - `🏁 [SCAN]` — scan completion summary

### 4. Centralised Configuration (`config.go`)
- All runtime configuration (GitLab OAuth, session, **database DSN**, and **worker counts**) is loaded once in `config.Load()`.
- `getEnvOrDefault()` helper keeps fallback defaults in one place.

### 5. Environment-Driven Tunability (`.env`)
- All config is declared in `.env` / `.env.example`, including:
  - `PROJECT_WORKERS` / `FILE_WORKERS` for scanner concurrency.
  - `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` for the database.

### 6. Asynchronous Jobs & Scaling (`scan_handler.go`, `docker-compose.yml`)
- Introduced an asynchronous background worker architecture to prevent 504 Gateway Timeouts on massive scans.
- `ScanHandler` now creates a `ScanJob` in PostgreSQL and returns a `job_id` immediately.
- The UI polls `/api/scan/:id` (`JobStatusHandler`) to check job progress.
- Migrated from SQLite to PostgreSQL 18 to support concurrent database writes.
- Scaled the backend to **3 replicas** behind an Nginx reverse proxy load balancer.

### 7. PDF & Excel Reporting
- PDF generator builds a well-structured, grouped report of all scan findings.
- Prints the exact File Name, Line Number, Keyword, and Full Code Snippet.
- Includes clickable deep links directly in the PDF document.
- PDF/Excel export endpoints accept cached scan results from the frontend — instantaneous without re-querying GitLab API.

### 8. Database & History (`models.go`, `db.go`)
- Implemented automatic database migrations for the `User`, `ScanHistory`, and `ScanJob` models.
- Tracks every scan performed by a user, including the Group ID, Keywords, Branch, and Match Count.

### 9. Response Caching & Performance
- **Groups list cache (10-min TTL):** First fetch from GitLab takes ~9s for large instances; subsequent requests <5ms. Keyed per user OAuth token.
- **Group projects cache (15-min TTL):** Per group-ID cache for instant re-selection.
- **Frontend debounce (300ms):** Rapid checkbox clicks batched into one request after 300ms idle.
- **Static file browser cache (7-day `Cache-Control`):** CSS/JS/images served with `public, max-age=604800, immutable`.
- **Favicon 204:** `/favicon.ico` returns `204 No Content` to suppress 404 noise.
- **Gin release mode:** `gin.SetMode(gin.ReleaseMode)` removes debug warnings.

---

## UI/UX — Tool Hub Design System

### Branding
- Application renamed from **GitLab Scanner** to **GIT-OPS** across all templates, PDF reports, and export filenames.

### Tool Hub (Post-Login Landing)
- After login, users land on the **Tool Hub** — a dedicated, card-based landing screen instead of jumping directly into a tool.
- The Hub displays three tool cards: **Code Scanner**, **Commit Reporter**, and **Branch Compare**.
- Each card has a tool icon, title, description, hover animations (lift + gradient overlay), and navigates directly into the selected tool's workspace.

### Workspace (Per-Tool View)
- The **Dashboard/Workspace** view is reached by selecting a tool from the Hub.
- **Back to Hub** button in the top navigation bar (pill-shaped, with animated back-arrow icon) returns the user to the Hub.
- The "Tools" sidebar list (redundant after introducing the Hub) has been removed to declutter the sidebar. The sidebar now shows only **Your Groups**.
- Active tool title and icon are shown in the workspace header.

### Navigation State Persistence
- `localStorage` persists `activeView` (`hub` or `dashboard`) and `activeTool` (`scanner`, `commits`, `compare`).
- On page refresh while inside a tool, the app restores the user directly to their active workspace — not back to the Hub.

### Theme System
- Three themes supported: **Dark** (default), **Light**, and **Mint (Retro)**.
- All themes are defined via CSS custom property overrides (`:root`, `.light-mode`, `.mint-mode`).
- A `<select>` theme dropdown is placed in the **login page brand header** (top-right) — users can choose a theme before signing in.
- The same dropdown appears in the **top navigation bar** of both the Hub and Workspace views, keeping selection always accessible.
- All theme selects are kept in sync; selecting on any view updates all others.
- Theme preference is saved to `localStorage` and restored on every page load.

### Mint (Retro) Theme Details
- Background: `#CFFFD9` (mint green)
- Text: `#000000` (stark black)
- Borders & shadows: solid black, offset box-shadow for retro feel
- Login page headings and brand text use `var(--color-foreground)` — readable in all themes.

---

## Known Limitations / Notes
- **Self-Hosted SSL:** The scanner is configured to skip SSL verification (`InsecureSkipVerify: true`) for self-hosted GitLab instances with custom certificates.
- **Rate Limiting:** Very high `PROJECT_WORKERS` × `FILE_WORKERS` values may trigger GitLab API rate limits. Defaults (4 × 3) are conservative and safe.

---

## Completed Tasks
- [x] Initial project scaffolding and routing.
- [x] OAuth SSO integration.
- [x] Scanner core logic — initial implementation.
- [x] UI/UX design and implementation.
- [x] PDF Generator integration with Unicode safety.
- [x] Branch-specific searching.
- [x] Line-number deep linking.
- [x] Instant PDF Export optimization.
- [x] Migrate to PostgreSQL for concurrent access.
- [x] Asynchronous background job processing and UI polling.
- [x] Nginx load balancing and 3-replica Docker Compose scaling.
- [x] **Scanner rewrite:** Replaced Elasticsearch blob search with file-tree walk + local grep (2-level goroutine parallelism).
- [x] **Config centralisation:** All DB and worker config moved to `config.go`, driven entirely by `.env`.
- [x] **Structured logging:** Emoji-prefixed, filterable log lines for every stage of the scan pipeline.
- [x] **Response caching:** Redis-backed 10-min groups list + 15-min group projects TTL cache.
- [x] **Frontend debounce:** 300ms debounce on checkbox clicks prevents N×N API calls.
- [x] **Static file caching:** 7-day `Cache-Control` headers.
- [x] **Gin release mode + favicon 204:** Clean startup logs.
- [x] **Branding rename:** Application renamed to GIT-OPS across all files.
- [x] **Tool Hub UI:** Post-login landing page with 3 animated tool selection cards.
- [x] **Workspace view:** Tool-specific workspace with Back to Hub button.
- [x] **Sidebar cleanup:** Removed redundant Tools navigation list from workspace sidebar.
- [x] **Navigation state persistence:** `localStorage` preserves active view and tool on page refresh.
- [x] **Theme system:** Dark / Light / Mint (Retro) themes via CSS variables.
- [x] **Login page theme select:** Theme can be selected before login.
- [x] **Back to Hub button:** Pill-shaped button with animated back-arrow micro-interaction.
