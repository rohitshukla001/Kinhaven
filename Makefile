GO       ?= go
BIN_DIR  ?= bin
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
MODULE   := github.com/rohitshukla001/AmazonDeveloperHackathon
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION)

.DEFAULT_GOAL := check

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check:
	@out="$$(gofmt -l $$(git ls-files '*.go' 2>/dev/null || find . -name '*.go' -not -path './vendor/*'))"; \
	if [ -n "$$out" ]; then echo "These files need gofmt:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: lint
lint: fmt-check vet

.PHONY: test
test:
	$(GO) test -race -count=1 ./...

.PHONY: cover
cover:
	$(GO) test -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -n 1

.PHONY: build
build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/ ./cmd/...

.PHONY: run
run:
	$(GO) run ./cmd/kinhaven-server

.PHONY: check
check: lint test build

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) coverage.out
