# donn local task runner. Install just via https://just.systems.
# Run `just` for a list of recipes.

# Show available recipes when invoked without arguments.
default:
    @just --list

# Run the full gate locally, the same checks CI would run.
check: build test lint

# Compile every package.
build:
    go build ./...

# Run the test suite with the race detector.
test:
    go test -race -timeout 120s ./...

# Run golangci-lint v2.
lint:
    golangci-lint run --timeout 5m

# Format the code and tidy the module.
fmt:
    go fmt ./...
    go mod tidy

# Run the service locally on PORT (default 8080).
run:
    go run ./cmd/donn
