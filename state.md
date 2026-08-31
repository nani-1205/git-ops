# GitLab Code Scanner - Project State

## Overview
This document summarizes the development progress, architectural decisions, and features implemented in the GitLab Code Scanner project up to the current date.

## Architecture & Tech Stack
- **Backend:** Go (Golang)
- **Web Framework:** Gin (`github.com/gin-gonic/gin`)
- **Database:** SQLite (Pure Go driver `github.com/glebarez/sqlite` to avoid CGO requirements on Windows)
- **ORM:** GORM (`gorm.io/gorm`)
- **GitLab Integration:** Official Go GitLab Client (`gitlab.com/gitlab-org/api/client-go`)
- **PDF Generation:** gofpdf (`github.com/jung-kurt/gofpdf`)
- **Frontend:** Vanilla HTML, CSS, JavaScript (Single Page Application with Glassmorphism UI)

## Features Implemented

### 1. Authentication & Session Management
- Integrated **GitLab OAuth2 SSO** for secure user login.
- Implemented session cookies (`gin-contrib/sessions`) to persist user authentication.
- Added API endpoints for `/auth/login`, `/auth/callback`, `/auth/logout`, and `/auth/me`.

### 2. GitLab Code Scanning (`scanner.go`)
- Developed a recursive scanner that finds all projects within a specified GitLab Group or Subgroup.
- Utilized GitLab's Search API (`client.Search.BlobsByProject`) to find specific keywords in the codebase.
- **Branch Support:** Added the ability to specify a non-default branch (e.g., `jhsqa3`) to search against using the `Ref` parameter.
- **Deep Linking:** Engineered the scanner to extract exact line numbers (`blob.Startline`) and generate deep links (e.g., `/-/blob/main/path#L123`) that jump directly to the highlighted code in GitLab.
- Uses `gitlab.NewOAuthClient` to seamlessly authenticate API requests using the user's active SSO token.

### 3. PDF Reporting (`pdf_generator.go`)
- Implemented a PDF generator that builds a well-structured, grouped report of all scan findings.
- Prints the exact File Name, Line Number, Keyword, and the *Full Code Snippet*.
- Includes clickable deep links directly in the PDF document.
- **Optimization:** The PDF export endpoint accepts cached scan results directly from the frontend, making PDF generation instantaneous without needing to re-query the GitLab API.

### 4. Database & History (`models.go`, `db.go`)
- Implemented automatic database migrations for the `User` and `ScanHistory` models.
- Tracks every scan performed by a user, including the Group ID, Keywords, Branch, and Match Count.

### 5. Frontend Dashboard
- Built a modern, dark-themed, responsive dashboard.
- Uses dynamic fetch requests to prevent page reloads.
- Includes comprehensive error handling to display exact server errors (e.g., 401 Unauthorized or 404 Not Found) to the user.

## Known Limitations / Notes
- **GitLab Search Indexing:** GitLab's Basic Search API can sometimes be slow on very large monorepos. If the instance has Advanced Search (Elasticsearch) enabled, the API automatically leverages it for faster results.
- **Self-Hosted SSL:** The scanner is configured to skip SSL verification (`InsecureSkipVerify: true`) to support self-hosted GitLab instances using custom or self-signed certificates.

## Completed Tasks
- [x] Initial project scaffolding and routing.
- [x] OAuth SSO integration.
- [x] Database configuration (migrated from CGO SQLite to Pure Go SQLite).
- [x] Scanner core logic.
- [x] UI/UX design and implementation.
- [x] PDF Generator integration.
- [x] Branch-specific searching.
- [x] Line-number deep linking.
- [x] Instant PDF Export optimization.
