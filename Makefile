BINARY_NAME :=tiny-server
APP_VERSION := $(shell git describe --tags 2>/dev/null || echo v0.1.0)

LDFLAGS += -X "main.tinyServerVersion=$(APP_VERSION)"
LDFLAGS += -X "main.goVersion=$(shell go version | sed 's/.*go\([^ ]*\).*/\1/')"
GOBUILD := CGO_ENABLED=0 go
GOLANGCI_LINT_VERSION = v2.1.2

.PHONY: all
all: deps test dev
	@echo "All tasks completed."
	@echo "You can run the program with 'make dev' or test it with 'make test'."

# install-golangci-lint installs the pinned golangci-lint version only if it
# is missing or the installed version differs. It uses `go install` so the
# binary is built with the local Go toolchain; the prebuilt download script
# ships a binary built with an older Go that panics type-checking files
# requiring a newer toolchain.
.PHONY: install-golangci-lint
install-golangci-lint:
	@want=$$(echo "$(GOLANGCI_LINT_VERSION)" | sed 's/^v//'); \
	if command -v golangci-lint >/dev/null 2>&1 && golangci-lint --version 2>/dev/null | grep -q "$$want"; then \
		echo "golangci-lint $$want already installed at $$(which golangci-lint)"; \
	else \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) via go install..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi

.PHONY: lint
lint: install-golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run --timeout 5m ./...
	@echo "Linting completed."
	@echo "No issues found."

.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@go mod tidy

.PHONY: test
test: deps
	@echo "Running tests..."	
	@go test -v ./...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Code formatted."

.PHONY: build
build: deps
	@echo "Building the server..."
	@$(GOBUILD) build -ldflags '$(LDFLAGS)' -o "$(BINARY_NAME)" ./cmd/tiny-server
	@echo "Build completed. You can run the server with './$(BINARY_NAME)'"
	
.PHONY: dev
dev:
	@echo "Running the server in development mode..."
	@go run ./cmd/tiny-server

.PHONY: install
install: deps
	@echo "Installing the server to $$(go env GOPATH)/bin..."
	@$(GOBUILD) install -ldflags '$(LDFLAGS)' ./cmd/tiny-server
	@echo "Installation completed. You can now run '$(BINARY_NAME)' from anywhere"

.PHONY: release
release:
	@set -e; \
	if [ -z "$(VERSION)" ]; then \
		echo "Usage: make release VERSION=v1.0.0"; \
		echo "Tags are immutable and cannot be recreated. Provide an explicit, unused version."; \
		exit 1; \
	fi; \
	if git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null; then \
		echo "Tag $(VERSION) already exists; tags are immutable. Bump the version."; \
		exit 1; \
	fi; \
	echo "Tagging version $(VERSION)"; \
	git tag $(VERSION); \
	echo "Pushing tag to GitHub..."; \
	git push origin $(VERSION); \
	echo "Pushed tag $(VERSION) to GitHub"; \
	echo "The Release workflow is now building and uploading assets (~1m)."; \
	echo "The GitHub release will appear as 'Latest' once the run completes:"; \
	echo "  https://github.com/dilipgurung/tiny-server/actions"
