# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire backend source code
COPY . .

# Build the application
# CGO_ENABLED=0 is important for static linking, especially with SQLite on Alpine
# The mattn/go-sqlite3 driver requires CGO, so CGO_ENABLED must be 1.
# We need to ensure gcc and other build tools are available in the builder image.
# golang:alpine includes gcc.
RUN go build -o /funnel-controller ./cmd/server/main.go

# Stage 2: Create the final lightweight image
FROM alpine:latest

# Install any runtime dependencies.
# For go-sqlite3, we might need some C libraries if not statically linked,
# but typically it links them in. If issues arise, add:
# RUN apk add --no-cache libc6-compat
# However, a well-built Go binary with CGO should be self-contained enough or link against Alpine's libc.

WORKDIR /app

# Create a non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Copy the built application from the builder stage
COPY --from=builder /funnel-controller .

# Copy the data directory structure (optional, if you want it pre-created)
# The application creates it if it doesn't exist.
# RUN mkdir -p ./data

# Expose the port the app runs on
EXPOSE 8080

# Environment variables (can be overridden at runtime)
ENV APP_SERVER_ADDR=":8080"
# Default DB path inside container
ENV APP_DB_PATH="/app/data/funnel_controller.db"
ENV APP_SCHEDULER_INTERVAL="1m"
ENV APP_TAILSCALE_TARGET="on"
# APP_TOKEN_SECRET must be provided at runtime

# Command to run the application
ENTRYPOINT ["./funnel-controller"]
