# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies (CGO required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./

RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -o time-sync-server \
    ./cmd/server

# Stage 2: Production
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite-libs wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/time-sync-server .

# Create data directory for database
RUN mkdir -p /app/data

# Expose port (matches ServerPort in config)
EXPOSE 8080

# Run the server
CMD ["./time-sync-server"]
