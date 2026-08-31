# Build stage
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Install git if needed for fetching Go modules
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app
# Using CGO_ENABLED=0 since glebarez/sqlite is a pure Go SQLite driver
RUN CGO_ENABLED=0 GOOS=linux go build -o gitlab-scan ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/gitlab-scan .

# Copy the web directory (static files and templates)
COPY --from=builder /app/web ./web

# Expose port 5050 to the outside world
EXPOSE 5050

# Command to run the executable
CMD ["./gitlab-scan"]
