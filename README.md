# GitLab Code Scanner

A powerful, Go-based web application designed to recursively scan all projects within a GitLab Group (or Subgroup) for specific keywords, endpoints, or deprecated functions. It features a modern Single Page Application dashboard and instantaneous PDF report generation with deep links directly to the offending lines of code.

## Key Features

- **GitLab SSO:** Securely login using your GitLab account via OAuth2.
- **Recursive Group Scanning:** Enter a Group ID or path (e.g., `jpl/jhs/code/microservices`) and it will scan all repositories under that group.
- **Branch Support:** Search against the default branch, or specify any custom branch (e.g., `jhsqa3`).
- **Exact Line Deep-Linking:** Generates precise URLs that take you directly to the highlighted line of code in the GitLab UI.
- **Instant PDF Export:** Export your scan results into a neatly formatted PDF document instantly.
- **Self-Hosted Friendly:** Configured to work seamlessly with self-hosted GitLab instances (includes SSL certificate bypass for internal servers).

## Prerequisites

- [Go](https://golang.org/doc/install) (1.20 or higher recommended)
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
# Copy the example environment file
cp .env.example .env
```

Edit the `.env` file and fill in your details:
```env
GITLAB_URL=https://your-gitlab-instance.com
GITLAB_CLIENT_ID=your_application_id
GITLAB_CLIENT_SECRET=your_secret
OAUTH_REDIRECT_URL=http://localhost:5050/auth/callback
SESSION_SECRET=a_random_secret_string
```

### 3. Run the Application
Start the Go server:
```bash
go run ./cmd/server
```
Alternatively, build an executable:
```bash
go build -o gitlab-scan.exe ./cmd/server
./gitlab-scan.exe
```

### 4. Usage
1. Open your web browser and navigate to [http://localhost:5050](http://localhost:5050).
2. Click **Sign in with GitLab SSO** and authorize the application.
3. On the dashboard, enter your target **GitLab Group ID or Path** (e.g., `jpl/jhs/code/backendjobs/consumerjobs`).
4. Enter comma-separated **Keywords** to search for (e.g., `/jhswebapi/api/jmapis/userDevices_JM`).
5. (Optional) Enter a **Branch Name** if you want to scan a non-default branch.
6. Click **Start Scan**.
7. Review the results in the browser, click **View File** to jump to GitLab, or click **Export PDF** to download a report.

## Architecture Overview
- **Backend:** Go, Gin Framework
- **Database:** SQLite (Pure Go driver `glebarez/sqlite`)
- **API Integration:** `gitlab.com/gitlab-org/api/client-go`
- **PDF Engine:** `jung-kurt/gofpdf`
- **Frontend:** Vanilla HTML, CSS, JS
