# Testing Guide for gzfs

This guide covers how to write and run tests for the gzfs Go library that provides ZFS, ZPool, and ZDB functionality.

## Test Structure

The testing setup includes:

- **Unit Tests**: Test individual functions and methods with mocked dependencies
- **Integration Tests**: Test against real ZFS commands (requires ZFS installation)
- **Mock Utilities**: Helper functions for testing without real ZFS commands
- **Sample Data**: Pre-defined JSON responses and command outputs for testing

## Files Overview

### Test Files

- `helpers_test.go` - Tests for utility functions (ParseSize, ParseRatio, etc.)
- `client_test.go` - Tests for client creation and command execution
- `zfs_test.go` - Tests for ZFS operations (List, Get, Create, Snapshot, etc.)
- `zpool_test.go` - Tests for ZPool operations (List, Get, Create, Status, etc.)
- `zdb_test.go` - Tests for ZDB parsing and caching functionality
- `integration_test.go` - Integration tests that require real ZFS commands

### Utility Files

- `testutil/mock_runner.go` - Mock implementation of the Runner interface
- `testutil/sample_data.go` - Sample JSON responses and command outputs

## Running Tests

### Unit Tests (Recommended for Development)

Run all unit tests (no ZFS required):
```bash
go test -v
```

Run specific test file:
```bash
go test -v -run TestParseSize
```

Run tests with coverage:
```bash
go test -cover
```

Generate detailed coverage report:
```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Integration Tests (Requires ZFS)

Integration tests require actual ZFS commands to be available on the system.

Enable integration tests:
```bash
export GZFS_INTEGRATION_TESTS=1
go test -tags=integration -v
```

Run integration benchmarks:
```bash
export GZFS_INTEGRATION_TESTS=1
go test -tags=integration -bench=. -v
```

## Writing Tests

### Using Mock Runner

The `MockRunner` allows you to simulate command execution without running actual commands:

```go
func TestZFS_List(t *testing.T) {
    ctx := context.Background()
    mockRunner := testutil.NewMockRunner()
    
    client := &zfs{
        cmd: Cmd{
            Bin:    "zfs",
            Runner: mockRunner,
        },
    }
    
    // Set up expected command and response
    expectedCmd := "zfs list -o all -p -j"
    mockRunner.AddCommand(expectedCmd, testutil.ZFSListJSON, "", nil)
    
    // Execute test
    datasets, err := client.List(ctx, false)
    
    // Verify results
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    if len(datasets) != 2 {
        t.Errorf("Expected 2 datasets, got %d", len(datasets))
    }
}
```

### Testing Error Conditions

```go
func TestZFS_ListError(t *testing.T) {
    ctx := context.Background()
    mockRunner := testutil.NewMockRunner()
    
    client := &zfs{
        cmd: Cmd{
            Bin:    "zfs",
            Runner: mockRunner,
        },
    }
    
    // Don't add mock command to simulate error
    _, err := client.List(ctx, false)
    
    if err == nil {
        t.Error("Expected error but got nil")
    }
}
```

### Testing with Custom Data

Create custom JSON responses for specific test scenarios:

```go
const customZFSJSON = `{
  "output_version": {"command": "zfs list", "vers_major": 0, "vers_minor": 1},
  "datasets": {
    "testpool": {
      "name": "testpool",
      "type": "FILESYSTEM",
      "pool": "testpool",
      "properties": {
        "used": {"value": "1024", "source": {"type": "default", "data": ""}},
        "available": {"value": "2048", "source": {"type": "default", "data": ""}}
      }
    }
  }
}`

mockRunner.AddCommand("zfs list -o all -p -j", customZFSJSON, "", nil)
```

## Testing Best Practices

### 1. Test Structure

Use table-driven tests for multiple scenarios:

```go
func TestParseSize(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected uint64
    }{
        {"empty string", "", 0},
        {"bytes", "1024", 1024},
        {"kilobytes", "1K", 1024},
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ParseSize(tt.input)
            if result != tt.expected {
                t.Errorf("ParseSize(%q) = %d, want %d", tt.input, result, tt.expected)
            }
        })
    }
}
```

### 2. Error Testing

Always test both success and error paths:

```go
tests := []struct {
    name        string
    input       string
    expectError bool
}{
    {"valid input", "valid", false},
    {"invalid input", "invalid", true},
}
```

### 3. Mock Verification

Verify that commands were called correctly:

```go
// Execute function under test
client.SomeMethod(ctx, args...)

// Verify the right command was called
lastCall := mockRunner.GetLastCall()
if lastCall == nil {
    t.Fatal("No command was executed")
}

expectedCmd := "expected command string"
if lastCall.Cmd != expectedCmd {
    t.Errorf("Expected command %q, got %q", expectedCmd, lastCall.Cmd)
}
```

### 4. State Verification

Test that objects are properly initialized and linked:

```go
if dataset.z == nil {
    t.Error("Dataset should have zfs client reference")
}
if dataset.Used == 0 {
    t.Error("Dataset Used field should be parsed from JSON")
}
```

## Common Testing Patterns

### Testing Client Creation

```go
func TestNewClient(t *testing.T) {
    client := NewClient(Options{Sudo: true})
    
    if client == nil {
        t.Fatal("NewClient returned nil")
    }
    if client.ZFS == nil {
        t.Error("ZFS client not initialized")
    }
    if !client.ZFS.cmd.Sudo {
        t.Error("Sudo option not set")
    }
}
```

### Testing Command Building

```go
func TestCommandArgs(t *testing.T) {
    mockRunner := testutil.NewMockRunner()
    // ... setup ...
    
    client.SomeMethod(ctx, "arg1", "arg2")
    
    lastCall := mockRunner.GetLastCall()
    expectedArgs := []string{"arg1", "arg2"}
    if !reflect.DeepEqual(lastCall.Args[1:], expectedArgs) { // Skip binary name
        t.Errorf("Expected args %v, got %v", expectedArgs, lastCall.Args[1:])
    }
}
```

### Testing JSON Parsing

```go
func TestJSONParsing(t *testing.T) {
    mockRunner := testutil.NewMockRunner()
    mockRunner.AddCommand("command -j", validJSON, "", nil)
    
    result, err := client.SomeJSONMethod(ctx)
    
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    
    // Verify parsed fields
    if result.SomeField != expectedValue {
        t.Errorf("Expected SomeField %v, got %v", expectedValue, result.SomeField)
    }
}
```

## Continuous Integration

For CI/CD pipelines, run unit tests without integration tests:

```yaml
# GitHub Actions example
- name: Run Tests
  run: go test -v -race -coverprofile=coverage.out

- name: Upload Coverage
  uses: codecov/codecov-action@v3
  with:
    file: ./coverage.out
```

For systems with ZFS available:

```yaml
- name: Run Integration Tests
  run: |
    export GZFS_INTEGRATION_TESTS=1
    go test -tags=integration -v
  # Only run on systems with ZFS installed
```

## Performance Testing

Use benchmarks to ensure performance doesn't regress:

```go
func BenchmarkParseSize(b *testing.B) {
    for i := 0; i < b.N; i++ {
        ParseSize("10G")
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem
```

## Debugging Tests

Enable verbose output and run specific tests:
```bash
go test -v -run TestSpecificFunction
```

Use build tags for conditional compilation:
```bash
go test -tags=debug -v
```

## Test Coverage Goals

Aim for high test coverage on critical paths:
- Parsing functions: 100%
- Command building: 90%+
- Error handling: 90%+
- Core business logic: 85%+

Check coverage:
```bash
go test -cover
```

This testing setup provides comprehensive coverage of the gzfs library while allowing for both development (unit tests) and validation (integration tests) workflows.