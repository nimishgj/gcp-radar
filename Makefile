.PHONY: build run test test-race clean deps fmt fmt-check mod-check lint ci docker help

BINARY_NAME=gcp-radar
BUILD_DIR=bin
MAIN_PATH=./cmd/gcp-radar

GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# golangci-lint is installed into $(BUILD_DIR) on demand so `make ci` is
# self-contained and runs the same locally as in CI.
GOLANGCI_LINT_VERSION=v2.12.2
GOLANGCI_LINT=$(BUILD_DIR)/golangci-lint

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

run:
	@echo "Running $(BINARY_NAME)..."
	$(GORUN) $(MAIN_PATH)

test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./...

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

fmt-check:
	@echo "Checking code formatting..."
	@test -z "$$(gofmt -l .)" || (echo "Code not formatted. Run 'make fmt'" && gofmt -l . && exit 1)

mod-check:
	@echo "Checking go.mod..."
	@go mod tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod/go.sum not tidy. Run 'go mod tidy'" && exit 1)

$(GOLANGCI_LINT):
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@mkdir -p $(BUILD_DIR)
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(BUILD_DIR) $(GOLANGCI_LINT_VERSION)

lint: $(GOLANGCI_LINT)
	@echo "Linting code..."
	@$(GOLANGCI_LINT) run ./...

# CI recipe - the single source of truth, runnable locally and in CI.
ci: fmt-check mod-check lint build test
	@echo "CI checks passed!"

docker:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

help:
	@echo "GCP Radar - Available targets:"
	@echo "  build      - Build the binary"
	@echo "  run        - Run the application locally"
	@echo "  test       - Run tests"
	@echo "  test-race  - Run tests with race detection"
	@echo "  clean      - Clean build artifacts"
	@echo "  deps       - Download and tidy dependencies"
	@echo "  fmt        - Format code"
	@echo "  fmt-check  - Check code formatting (CI)"
	@echo "  mod-check  - Check go.mod is tidy (CI)"
	@echo "  lint       - Lint code"
	@echo "  ci         - Run all CI checks (fmt, mod, lint, build, test)"
	@echo "  docker     - Build Docker image"
	@echo "  help       - Show this help message"

.DEFAULT_GOAL := build
