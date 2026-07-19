# =============================================================================
# ©AngelaMos | 2026
# justfile - crypha steganography multi-tool
# =============================================================================

set shell := ["bash", "-uc"]
set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]

binary := "crypha"
version := `git describe --tags --always 2>/dev/null || echo "dev"`

# =============================================================================
# Default
# =============================================================================

default:
    @just --list --unsorted

# =============================================================================
# Formatting and Linting
# =============================================================================

[group('lint')]
fmt:
    gofmt -w -s cmd/ internal/

[group('lint')]
vet:
    go vet ./...

[group('lint')]
lint:
    GOTOOLCHAIN=go1.25.7 golangci-lint run ./...

# =============================================================================
# Testing
# =============================================================================

[group('test')]
test *ARGS:
    go test ./... {{ARGS}}

[group('test')]
test-race:
    go test -race ./...

[group('test')]
cov:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

[group('test')]
cov-html: cov
    go tool cover -html=coverage.out

# =============================================================================
# Build and Run
# =============================================================================

[group('build')]
build:
    go build -o dist/{{binary}} ./cmd/{{binary}}

[group('build')]
run *ARGS:
    go run ./cmd/{{binary}} {{ARGS}}

# =============================================================================
# CI / Housekeeping
# =============================================================================

[group('ci')]
ci: fmt vet test build

[group('ci')]
tidy:
    go mod tidy

[group('ci')]
info:
    @echo "Binary:  {{binary}}"
    @echo "Version: {{version}}"
    @echo "Go:      $(go version)"

[group('ci')]
clean:
    -rm -rf dist
    -rm -f coverage.out
    @echo "Cleaned"
