BINARY_NAME := msgraph-skill
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X github.com/merill/msgraph-skill/internal/config.Version=$(VERSION)"
CGO_ENABLED := 0

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64

.PHONY: build build-all clean test lint index help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build for current platform
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BINARY_NAME) .

build-all: ## Build for all platforms
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} ; \
		output=$(BINARY_NAME)-$${GOOS}-$${GOARCH} ; \
		if [ "$${GOOS}" = "windows" ]; then output=$${output}.exe; fi ; \
		echo "Building $${output}..." ; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$${GOOS} GOARCH=$${GOARCH} go build $(LDFLAGS) -o dist/$${output} . ; \
	done

install: build ## Build and copy binary to skill scripts/bin/
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]') ; \
	ARCH=$$(uname -m) ; \
	case $$ARCH in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac ; \
	cp $(BINARY_NAME) skills/msgraph/scripts/bin/$(BINARY_NAME)-$${OS}-$${ARCH} ; \
	echo "Installed to skills/msgraph/scripts/bin/$(BINARY_NAME)-$${OS}-$${ARCH}"

test: ## Run tests
	go test ./... -v

lint: ## Run linter
	golangci-lint run ./...

index: ## Run the OpenAPI indexer to generate graph-api-index.json
	go run ./tools/openapi-indexer/... -output skills/msgraph/references/graph-api-index.json

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -f skills/msgraph/scripts/bin/$(BINARY_NAME)-*

docs-dev: ## Start docs dev server
	cd docs && npm run dev

docs-build: ## Build docs site
	cd docs && npm run build
