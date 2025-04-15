BINARY_NAME :=tiny-server
APP_VERSION := $(shell git describe --tags 2>/dev/null || echo v0.1.0)

LDFLAGS += -X "main.tinyServerVersion=$(APP_VERSION)"
LDFLAGS += -X "main.goVersion=$(shell go version | sed 's/.*go\([^ ]*\).*/\1/')"
GOBUILD := CGO_ENABLED=0 go

.PHONY: all
all: deps test dev
	@echo "All tasks completed."
	@echo "You can run the program with 'make dev' or test it with 'make test'."

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
	@go run ./cmd/tiny-server/*.go

.PHONY: install
install: deps
	@echo "Installing the server to $$(go env GOPATH)/bin..."
	@$(GOBUILD) install -ldflags '$(LDFLAGS)' ./cmd/tiny-server
	@echo "Installation completed. You can now run '$(BINARY_NAME)' from anywhere"

.PHONY: release
release:
	@set -e; \
	current_tag=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	echo "Current tag: $$current_tag"; \
	if [ -z "$(VERSION)" ]; then \
		version=$$(echo $$current_tag | sed 's/^v//' | awk -F. '{printf "%d.%d.%d", $$1, $$2+1, $$3}'); \
		new_tag="v$$version"; \
	else \
		new_tag="$(VERSION)"; \
	fi; \
	echo "Tagging version $$new_tag"; \
	git tag -f $$new_tag; \
	if [ "$$current_tag" != "v0.0.0" ]; then \
		git tag -d $$current_tag || true; \
		echo "Deleted old tag $$current_tag"; \
	fi; \
	echo "Pushing tag to GitHub..."; \
	@git push origin $$new_tag; \
	echo "Pushed tag $$new_tag to GitHub"
