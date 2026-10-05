# CareCircle build targets. Run "make help" for the list.

GO       ?= go
BIN_DIR  ?= bin
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
MODULE   := github.com/rohitshukla001/AmazonDeveloperHackathon
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show the available targets.
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

.PHONY: fmt
fmt: ## Format all Go files.
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## Fail if a Go file is not formatted.
	@out="$$(gofmt -l $$(git ls-files '*.go' 2>/dev/null || find . -name '*.go' -not -path './vendor/*'))"; \
	if [ -n "$$out" ]; then echo "These files need gofmt:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet: ## Run go vet.
	$(GO) vet ./...

.PHONY: test
test: ## Run all tests with the race detector.
	$(GO) test -race -count=1 ./...

.PHONY: cover
cover: ## Run tests and print coverage per function.
	$(GO) test -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -n 1

.PHONY: build
build: ## Build all binaries into $(BIN_DIR)/.
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/ ./cmd/...

.PHONY: run
run: ## Run the CareCircle server.
	$(GO) run ./cmd/carecircle-server

.PHONY: check
check: fmt-check vet test build ## Run all checks (CI target).

.PHONY: clean
clean: ## Remove build output.
	rm -rf $(BIN_DIR) coverage.out
