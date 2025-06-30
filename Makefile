.PHONY: build clean install dev test lint lint-fix lint-verbose check help

# Default target
build:
	@mkdir -p bin
	go build -o bin/ghprs

# Development build with race detection
dev:
	@mkdir -p bin
	go build -race -o bin/ghprs

# Install to GOPATH/bin
install:
	go install

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Run tests with Ginkgo
test-ginkgo:
	~/go/bin/ginkgo -r

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run

# Run linter and auto-fix issues where possible
lint-fix:
	golangci-lint run --fix

# Run linter with verbose output
lint-verbose:
	golangci-lint run -v

# Run comprehensive checks (tests + linting)
check: test-ginkgo lint
	@echo "✅ All checks passed!"

# Show help for available make targets
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary to bin/ghprs"
	@echo "  dev           - Build with race detection"
	@echo "  install       - Install to GOPATH/bin"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run standard Go tests"
	@echo "  test-ginkgo   - Run tests with Ginkgo framework"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  lint          - Run golangci-lint"
	@echo "  lint-fix      - Run golangci-lint with auto-fix"
	@echo "  lint-verbose  - Run golangci-lint with verbose output"
	@echo "  check         - Run tests and linting together"
	@echo "  run ARGS=...  - Build and run with arguments"
	@echo "  build-all     - Build for multiple platforms"
	@echo "  help          - Show this help message"

# Build and run (use: make run ARGS="--help")
run: build
	./bin/ghprs $(ARGS)

# Build for multiple platforms
build-all:
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/ghprs-linux-amd64
	GOOS=darwin GOARCH=amd64 go build -o bin/ghprs-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build -o bin/ghprs-darwin-arm64
	GOOS=windows GOARCH=amd64 go build -o bin/ghprs-windows-amd64.exe 