.PHONY: build run clean test

# Build the server
build:
	go build -o time-sync-server ./cmd/server

# Run the server
run:
	go run ./cmd/server/main.go

# Clean build artifacts
clean:
	rm -f time-sync-server
	rm -f *.db *.db-shm *.db-wal

# Run tests
test:
	go test ./...

# Build for production
build-prod:
	CGO_ENABLED=1 go build -ldflags="-s -w" -o time-sync-server ./cmd/server

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run
