.PHONY: build test lint clean cli service

# Build the CLI binary
cli:
	go build -o bin/toposcope ./cmd/toposcope

# Build the service binary
service:
	go build -o bin/toposcoped ./cmd/toposcoped

# Build everything
build: cli service

# Run all tests
test:
	go test ./... -v -count=1

# Run tests with coverage
test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Build the web UI
web:
	cd web && pnpm install && pnpm build

# Run the CLI locally against a repo
run-score:
	go run ./cmd/toposcope score --base main --head HEAD

# Run the local UI server
run-ui:
	go run ./cmd/toposcope ui
