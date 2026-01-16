.PHONY: build test install clean validate analyze

# Build the binary
build:
	go build -o claude-permissions-hook .

# Build optimized release binary
release:
	go build -ldflags="-s -w" -o claude-permissions-hook .

# Run tests
test:
	go test -v ./...

# Install to GOPATH/bin
install:
	go install .

# Clean build artifacts
clean:
	rm -f claude-permissions-hook
	go clean

# Validate example config
validate: build
	./claude-permissions-hook validate --config example.toml

# Analyze sample session allowlist
analyze: build
	./claude-permissions-hook analyze --allowlist tests/sample-session-allowlist.json --format toml

# Parse example commands
parse-examples: build
	@echo "=== Simple command ==="
	./claude-permissions-hook parse "git add -A"
	@echo ""
	@echo "=== Compound command ==="
	./claude-permissions-hook parse "git add -A && git commit -m 'test'"
	@echo ""
	@echo "=== Timeout wrapper ==="
	./claude-permissions-hook parse "timeout 30 dotnet run"
	@echo ""
	@echo "=== Path-based command ==="
	./claude-permissions-hook parse "/home/linuxbrew/.linuxbrew/bin/glab ci list"

# Run with sample input
demo: build
	@echo '{"session_id":"test","tool_name":"Bash","tool_input":{"command":"git add -A && git commit -m test"}}' | \
		./claude-permissions-hook run --config example.toml

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	go vet ./...
