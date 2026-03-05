BINARY_NAME := msgraph
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X github.com/merill/msgraph/internal/config.Version=$(VERSION)"
CGO_ENABLED := 0

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64
SKILL_BIN_DIR := skills/msgraph/scripts/bin
DEV_SKILL_DIR := dev-skill-test/.opencode/skills/msgraph

.PHONY: build build-all clean test lint index samples api-docs concept-docs dev dev-clean help

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

index: ## Run the OpenAPI indexer to generate graph-api-index.json and .db
	go run ./tools/openapi-indexer/... -output skills/msgraph/references/graph-api-index.json

samples: ## Build the samples index (JSON + FTS database) from YAML source files
	go run . build-samples-index --samples-dir samples --output skills/msgraph/references/samples-index.json

api-docs: ## Generate api-docs-index.json and .db from Graph API documentation
	go run ./tools/api-docs-indexer/... -version beta -output skills/msgraph/references/api-docs-index.json
	cp skills/msgraph/references/api-docs-index.json docs/public/api-docs-index.json
	cp skills/msgraph/references/api-docs-index.db docs/public/api-docs-index.db

concept-docs: ## Rebuild curated concept docs from Microsoft Graph docs repo
	go run ./tools/concept-docs-builder/... -output skills/msgraph/references/docs

dev: build samples ## Build skill and install to dev-skill-test/, then open OpenCode
	@rm -rf dev-skill-test
	@mkdir -p $(DEV_SKILL_DIR)/scripts/bin
	@cp skills/msgraph/SKILL.md $(DEV_SKILL_DIR)/
	@cp -R skills/msgraph/references $(DEV_SKILL_DIR)/
	@cp -R samples $(DEV_SKILL_DIR)/
	@cp skills/msgraph/scripts/run.sh $(DEV_SKILL_DIR)/scripts/
	@cp skills/msgraph/scripts/run.ps1 $(DEV_SKILL_DIR)/scripts/
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]') ; \
	ARCH=$$(uname -m) ; \
	case $$ARCH in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac ; \
	cp $(BINARY_NAME) $(DEV_SKILL_DIR)/scripts/bin/$(BINARY_NAME)_$${OS}_$${ARCH}
	@echo "Skill installed to dev-skill-test/"
	@echo "Launching OpenCode..."
	opencode dev-skill-test

dev-clean: ## Remove dev-skill-test/ directory
	rm -rf dev-skill-test

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -f $(SKILL_BIN_DIR)/$(BINARY_NAME)_*

docs-dev: ## Start docs dev server
	cd docs && npm run dev

docs-build: ## Build docs site
	cd docs && npm run build
