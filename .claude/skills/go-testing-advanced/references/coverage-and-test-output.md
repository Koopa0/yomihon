# Coverage Workflow and Test Output

Coverage commands and CI integration, plus structured test output APIs.
The "What to Cover" priority table lives in SKILL.md.

## Coverage Workflow

### Basic Coverage

```bash
# Run tests with coverage
go test -cover ./...

# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out
```

### Package-Specific Coverage

```bash
# Coverage for specific packages (useful in monorepos)
go test -coverprofile=coverage.out -coverpkg=./internal/order/...,./internal/user/... ./...

# Coverage for a single package including its dependencies
go test -coverprofile=coverage.out -coverpkg=./internal/order/... ./internal/order/...
```

### Coverage in CI

```bash
# Check coverage meets minimum threshold
go test -coverprofile=coverage.out ./...
coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
if (( $(echo "$coverage < 60" | bc -l) )); then
    echo "Coverage $coverage% is below 60% threshold"
    exit 1
fi
```

## t.Attr — Structured Test Attributes (Go 1.25+)

`t.Attr` emits structured key-value attributes to the test log. Useful for
associating metadata with test runs (e.g., for CI dashboards or post-processing).

```go
func TestOrder(t *testing.T) {
    t.Attr("order_id", id)
    t.Attr("status", "created")
}
```

## t.ArtifactDir — Persistent Test Output (Go 1.26+)

`t.ArtifactDir()` returns a directory that survives test cleanup. Use it for
test-generated files you want to inspect after the test run.

```go
func TestGenerate(t *testing.T) {
    dir := t.ArtifactDir() // survives test cleanup
    os.WriteFile(filepath.Join(dir, "output.json"), data, 0o644)
}
// Run with: go test -artifacts ./testoutput
```
