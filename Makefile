BINARY_NAME := msgraph
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X github.com/merill/msgraph/internal/config.Version=$(VERSION)"
CGO_ENABLED := 0

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64
SKILL_BIN_DIR := skills/msgraph/scripts/bin

.PHONY: build build-all clean test lint index help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build for current platform
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BINARY_NAME) .

build-all: ## Build all platform binaries into skill scripts/bin/
	@mkdir -p $(SKILL_BIN_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} ; \
		output=$(BINARY_NAME)_$${GOOS}_$${GOARCH} ; \
		if [ "$${GOOS}" = "windows" ]; then output=$${output}.exe; fi ; \
		echo "Building $${output}..." ; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$${GOOS} GOARCH=$${GOARCH} go build $(LDFLAGS) -o $(SKILL_BIN_DIR)/$${output} . ; \
	done

install: build ## Build and copy binary to skill scripts/bin/
	@mkdir -p $(SKILL_BIN_DIR)
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]') ; \
	ARCH=$$(uname -m) ; \
	case $$ARCH in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac ; \
	cp $(BINARY_NAME) $(SKILL_BIN_DIR)/$(BINARY_NAME)_$${OS}_$${ARCH} ; \
	echo "Installed to $(SKILL_BIN_DIR)/$(BINARY_NAME)_$${OS}_$${ARCH}"

test: ## Run tests
	go test ./... -v

lint: ## Run linter
	golangci-lint run ./...

index: ## Run the OpenAPI indexer to generate graph-api-index.json
	go run ./tools/openapi-indexer/... -output skills/msgraph/references/graph-api-index.json

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -f $(SKILL_BIN_DIR)/$(BINARY_NAME)_*

docs-dev: ## Start docs dev server
	cd docs && npm run dev

docs-build: ## Build docs site
	cd docs && npm run build
