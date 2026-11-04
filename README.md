# GIT-OPS

A powerful, Go-based DevOps web application for GitLab teams. GIT-OPS provides three core tools — **Code Scanner**, **Commit Reporter**, and **Branch Compare** — accessible through a modern, theme-aware Tool Hub interface after login.

---

## Key Features

- **GitLab SSO:** Securely login using your GitLab account via OAuth2.
- **Tool Hub:** A beautiful post-login landing screen with animated cards to select the tool you need — no cluttered sidebar navigation.
- **Code Scanner:** Recursively scan all repositories within a GitLab Group (or Subgroup) for specific keywords, endpoints, or deprecated functions via BFS subgroup traversal.
- **Fast Parallel Scanning:** Two-level goroutine parallelism — configurable `PROJECT_WORKERS` repos scanned concurrently, each with `FILE_WORKERS` file-fetching goroutines. No dependency on GitLab Elasticsearch.
- **Commit Reporter:** Generate detailed reports of user commits over a specified date range and branch.
- **Branch Compare:** Compare source and target branches to identify divergence and sync status.
- **Smart Response Caching:** Redis-backed 10-minute TTL on groups + 15-minute TTL on projects — reduces UI latency from ~9s to <5ms.
- **Instant PDF / XLSX Export:** Export scan results into formatted reports instantly (results cached in the browser, no re-scan required).
- **Theme System:** Switch between **Dark**, **Light**, and **Mint (Retro)** themes at any time — even before login.
- **Navigation State Persistence:** Refreshing the page while inside a tool restores your workspace, not the Hub.
- **Structured Logs:** Emoji-prefixed, filterable log output for every scan stage.
- **Self-Hosted Friendly:** Configured to work seamlessly with self-hosted GitLab instances (SSL certificate bypass for internal servers).

---

## Prerequisites

- [Go](https://golang.org/doc/install) (1.20 or higher)
- [Docker & Docker Compose](https://docs.docker.com/get-docker/)
- [Redis](https://redis.io/) (local or containerised)
- A GitLab Account (SaaS or Self-Hosted)

---

## Setup & Installation

### 1. Create a GitLab OAuth Application
1. Go to your GitLab **Admin Area** → **Applications**.
2. Click **New application** and name it `GIT-OPS`.
3. Set the **Redirect URI** to `http://localhost:5050/auth/callback`.
4. Check the **`read_api`** and **`read_user`** scopes.
5. Save and copy your **Application ID** and **Secret**.

### 2. Configure Environment Variables

```bash
cp .env.example .env
```

Edit `.env` with your details:

```env
# GitLab OAuth
GITLAB_URL=https://your-gitlab-instance.com
GITLAB_CLIENT_ID=your_application_id
GITLAB_CLIENT_SECRET=your_secret
OAUTH_REDIRECT_URL=http://localhost:5050/auth/callback
SESSION_SECRET=a_random_secret_string

# Database (PostgreSQL)
DB_HOST=localhost
DB_PORT=5433
DB_USER=scanner
DB_PASSWORD=scanner_password
DB_NAME=scanner_db

# Redis Cache
REDIS_ADDR=localhost:6379

# Scanner concurrency (tune to your GitLab server's capacity)
PROJECT_WORKERS=4   # repos scanned in parallel
FILE_WORKERS=3      # files fetched per repo in parallel
```

### 3. Run the Application

Recommended — Docker Compose (starts PostgreSQL, Redis, and Nginx alongside 3 app replicas):

```bash
docker-compose up -d --build
```

Run directly (DB and Redis must be running separately):

```bash
docker-compose up -d db redis
go run ./cmd/server/main.go
```

---

## Usage

1. Open [http://localhost:5050](http://localhost:5050) in your browser.
2. *(Optional)* Select a **theme** (Dark / Light / Mint) from the top-right dropdown before logging in.
3. Click **Sign in with GitLab** and authorize the application.
4. You land on the **Tool Hub** — select the tool you want to use.
5. Use the **← Hub** button in the workspace header to return to the Tool Hub at any time.

### Code Scanner
1. Select one or more **Groups** from the sidebar.
2. Enter comma-separated **Keywords** and an optional **Branch** name.
3. Click **Start Scan**. Progress is polled in real time.
4. Review results and click **Export PDF** or **Export XLSX**.

### Commit Reporter
1. Select a **Group** from the sidebar.
2. Enter a **Branch**, **Start Date**, and **End Date**.
3. Click **Generate Report** to view and export commit data.

### Branch Compare
1. Select a **Group** from the sidebar.
2. Enter a **Source Branch** and **Target Branch**.
3. Click **Compare** to see divergence status.

---

## Log Output Reference

During a scan you'll see structured logs like:

```
🔎 [SCAN]    Starting — 12 project(s) | 4 project-worker(s) × 3 file-worker(s) | terms=[/api/users]
📂 [BFS]     Visiting group: jpl/jhs (id=42)
🔍 [BFS]     Queuing subgroup: jpl/jhs/microservices
✅ [BFS]     Done — 3 group(s) visited, 12 project(s) collected
🚀 [PROJECT] Starting (1/12) jpl/jhs/user-service | branch=main
📋 [PROJECT] jpl/jhs/user-service — 87 file(s) to scan with 3 worker(s)
🎯 [MATCH]   keyword="/api/users" file=src/routes.go line=42 project=jpl/jhs/user-service
✅ [PROJECT] Done (1/12) jpl/jhs/user-service | files=87 | matches=1 | total_matches_so_far=1
🏁 [SCAN]    Completed — 12 project(s) scanned | total matches=5
```

---

## Architecture Overview

| Layer | Technology |
|---|---|
| Backend | Go, Gin Framework (async job architecture, 2-level goroutine parallelism) |
| Database | PostgreSQL 18 (`gorm.io/driver/postgres`) |
| Cache | Redis (TTL-based, keyed per user/group) |
| Load Balancer | Nginx (3 app replicas) |
| API Integration | `gitlab.com/gitlab-org/api/client-go` |
| PDF Engine | `jung-kurt/gofpdf` |
| Frontend | Vanilla HTML, CSS, JS — Tool Hub SPA with Dark / Light / Mint themes |

---

## Known Limitations

- **Self-Hosted SSL:** Uses `InsecureSkipVerify: true` to support self-hosted GitLab with custom certificates.
- **Rate Limiting:** Very high `PROJECT_WORKERS` × `FILE_WORKERS` may trigger GitLab API rate limits. Defaults (4 × 3) are safe and conservative.
