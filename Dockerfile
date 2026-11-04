# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Install git (required by some Go modules during download)
RUN apk add --no-cache git

# Copy go module files first for better layer caching:
# dependencies are only re-downloaded when go.mod / go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree
COPY . .

# Build a statically-linked binary for Linux.
# CGO_ENABLED=0 produces a self-contained binary with no libc dependency,
# which is safe to run on Alpine (PostgreSQL driver is pure-Go via pgx).
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gitlab-scan ./cmd/server

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.21

# ca-certificates  – required for TLS/HTTPS calls to GitLab (assetgit.com etc.)
# tzdata           – required so time.LoadLocation("Asia/Kolkata") works correctly
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/gitlab-scan .

# Copy static assets and HTML templates
COPY --from=builder /app/web ./web

# Expose the application port
EXPOSE 5050

# Run the application
CMD ["./gitlab-scan"]
