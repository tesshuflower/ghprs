.PHONY: build clean install dev test

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