# Makefile for gzfs testing

.PHONY: test test-unit test-integration test-coverage test-race benchmark clean help

# Default target
help:
	@echo "Available targets:"
	@echo "  test           - Run all unit tests"
	@echo "  test-unit      - Run unit tests only"
	@echo "  test-integration - Run integration tests (requires ZFS)"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-race      - Run tests with race detection"
	@echo "  benchmark      - Run benchmarks"
	@echo "  lint           - Run go vet and staticcheck"
	@echo "  clean          - Clean test artifacts"

# Run all unit tests
test: test-unit

# Run unit tests only (no ZFS required)
test-unit:
	@echo "Running unit tests..."
	go test -v ./...

# Run integration tests (requires ZFS installation)
test-integration:
	@echo "Running integration tests..."
	@echo "Note: This requires ZFS to be installed and accessible"
	GZFS_INTEGRATION_TESTS=1 go test -tags=integration -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -v -race ./...

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Run integration benchmarks
benchmark-integration:
	@echo "Running integration benchmarks..."
	GZFS_INTEGRATION_TESTS=1 go test -tags=integration -bench=. -benchmem ./...

# Run linting
lint:
	@echo "Running go vet..."
	go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then \
		echo "Running staticcheck..."; \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed, skipping (install with: go install honnef.co/go/tools/cmd/staticcheck@latest)"; \
	fi

# Clean test artifacts
clean:
	@echo "Cleaning test artifacts..."
	rm -f coverage.out coverage.html
	go clean -testcache

# Quick test during development
quick:
	go test -short ./...

# Test specific package
test-helpers:
	go test -v -run "TestParse.*"

test-client:
	go test -v -run "TestCmd.*|TestNewClient.*|TestLocalRunner.*"

test-zfs:
	go test -v -run "TestZFS.*"

test-zpool:
	go test -v -run "TestZpool.*|TestZPool.*"

test-zdb:
	go test -v -run "TestZDB.*"

# Development workflow targets
dev-test: lint test-race

# CI/CD workflow targets
ci-test: test-unit test-coverage

# Check if tests pass before committing
pre-commit: lint test-unit

# Generate test data (for updating sample responses)
generate-testdata:
	@echo "To generate test data, run the following on a system with ZFS:"
	@echo "  zfs list -o all -p -j > testutil/sample_zfs_list.json"
	@echo "  zpool list -o all -p -P -j > testutil/sample_zpool_list.json"
	@echo "  zdb -C poolname > testutil/sample_zdb_output.txt"