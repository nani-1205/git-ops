# GitLab Code Scanner

A powerful, Go-based web application designed to recursively scan all projects within a GitLab Group (or Subgroup) for specific keywords, endpoints, or deprecated functions. It features a modern Single Page Application dashboard and instantaneous PDF report generation with deep links directly to the offending lines of code.

## Key Features

- **GitLab SSO:** Securely login using your GitLab account via OAuth2.
- **Recursive Group Scanning:** Enter a Group ID or path (e.g., `jpl/jhs/code/microservices`) and it will scan all repositories under that group via BFS subgroup traversal.
- **Fast Parallel Scanning:** Two-level goroutine parallelism — configurable `PROJECT_WORKERS` repos scanned concurrently, each with `FILE_WORKERS` file-fetching goroutines. No dependency on GitLab Elasticsearch.
- **Smart Response Caching:** 10-minute TTL cache on GitLab API fetches for groups and 5-minute TTL for projects reduces UI latency from ~9s down to <5ms.
- **Branch Support:** Search against the default branch, or specify any custom branch (e.g., `jhsqa3`).
- **Exact Line Deep-Linking:** Generates precise URLs that take you directly to the highlighted line of code in the GitLab UI.
- **Instant PDF Export:** Export your scan results into a neatly formatted PDF document instantly.
- **Structured Logs:** Emoji-prefixed, filterable log output for every stage of the scan (group traversal, per-project progress, keyword matches, errors).
- **Self-Hosted Friendly:** Configured to work seamlessly with self-hosted GitLab instances (includes SSL certificate bypass for internal servers).

## Prerequisites

- [Go](https://golang.org/doc/install) (1.20 or higher recommended)
- [Docker & Docker Compose](https://docs.docker.com/get-docker/)
- A GitLab Account (SaaS or Self-Hosted)

## Setup & Installation

### 1. Create a GitLab OAuth Application
Before running the scanner, you must register it as an OAuth application in your GitLab instance:
1. Go to your GitLab **Admin Area** -> **Applications** (or User Settings -> Applications for a personal app).
2. Click **New application**.
3. Name it `GitLab Code Scanner`.
4. Set the **Redirect URI** to `http://localhost:5050/auth/callback`.
5. Check the boxes for the **`read_api`** and **`read_user`** scopes.
6. Save the application and copy your **Application ID** and **Secret**.

### 2. Configure Environment Variables
Clone the repository and set up your environment variables:

```bash
cp .env.example .env
```

Edit the `.env` file and fill in your details:

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

# Scanner concurrency (tune to your GitLab server's capacity)
PROJECT_WORKERS=4   # repos scanned in parallel
FILE_WORKERS=3      # files fetched per repo in parallel
```

### 3. Run the Application
The recommended way to run is Docker Compose, which starts PostgreSQL and Nginx alongside 3 app replicas:

```bash
docker-compose up -d --build
```

To run the server directly (DB must be running separately):

```bash
docker-compose up -d db          # start only the database
go run ./cmd/server/main.go      # start the Go server
```

### 4. Usage
1. Open your browser at [http://localhost:5050](http://localhost:5050).
2. Click **Sign in with GitLab SSO** and authorize the application.
3. Enter your target **GitLab Group ID or Path** (e.g., `jpl/jhs/code/backendjobs/consumerjobs`).
4. Enter comma-separated **Keywords** to search for (e.g., `/jhswebapi/api/jmapis/userDevices_JM`).
5. (Optional) Enter a **Branch Name** to scan a non-default branch.
6. Click **Start Scan**. The scan runs asynchronously in the background.
7. Review results in the browser, click **View File** to jump to GitLab, or click **Export PDF**.

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

## Architecture Overview
- **Backend:** Go, Gin Framework (async job architecture, 2-level goroutine parallelism)
- **Database:** PostgreSQL 18 (`gorm.io/driver/postgres`)
- **Load Balancer:** Nginx (3 app replicas)
- **API Integration:** `gitlab.com/gitlab-org/api/client-go`
- **PDF Engine:** `jung-kurt/gofpdf`
- **Frontend:** Vanilla HTML, CSS, JS (async polling UI)
