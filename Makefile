.PHONY: test test-unit test-integration test-coverage test-race benchmark clean help

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

test: test-unit

test-unit:
	@echo "Running unit tests..."
	go test -v ./...

test-integration:
	@echo "Running integration tests..."
	@echo "Note: This requires ZFS to be installed and accessible"
	GZFS_INTEGRATION_TESTS=1 go test -tags=integration -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race:
	@echo "Running tests with race detection..."
	go test -v -race ./...

benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

benchmark-integration:
	@echo "Running integration benchmarks..."
	GZFS_INTEGRATION_TESTS=1 go test -tags=integration -bench=. -benchmem ./...

lint:
	@echo "Running go vet..."
	go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then \
		echo "Running staticcheck..."; \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed, skipping (install with: go install honnef.co/go/tools/cmd/staticcheck@latest)"; \
	fi

clean:
	@echo "Cleaning test artifacts..."
	rm -f coverage.out coverage.html
	go clean -testcache

quick:
	go test -short ./...

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

dev-test: lint test-race

ci-test: test-unit test-coverage

pre-commit: lint test-unit

generate-testdata:
	@echo "To generate test data, run the following on a system with ZFS:"
	@echo "  zfs list -o all -p -j > testutil/sample_zfs_list.json"
	@echo "  zpool list -o all -p -P -j > testutil/sample_zpool_list.json"
	@echo "  zdb -C poolname > testutil/sample_zdb_output.txt"
