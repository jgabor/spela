BINARY := spela
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
FRONTEND_DIR := internal/gui/frontend

# Check for bun (required for frontend build)
BUN := $(shell command -v bun 2>/dev/null)
ifndef BUN
$(error bun is required but not installed. Install it from https://bun.sh)
endif

.PHONY: build test test-frontend lint install clean frontend-build dev dev-stop

frontend-build:
	cd $(FRONTEND_DIR) && bun install && bun run build

build: frontend-build
	go build $(LDFLAGS) -o $(BINARY) ./cmd/spela

# Dev mode with hot-reload: starts Vite dev server and Go binary with dev tag
dev:
	@cd $(FRONTEND_DIR) && bun install
	@echo "Starting Vite dev server..."
	@cd $(FRONTEND_DIR) && bun run dev &
	@sleep 2
	@echo "Building Go binary with dev tag..."
	go build -tags dev $(LDFLAGS) -o $(BINARY) ./cmd/spela
	@echo "Starting spela..."
	./$(BINARY) gui

dev-stop:
	@pkill -f "bun run dev" || true
	@echo "Vite dev server stopped"

test:
	go test -v ./...

test-frontend:
	cd $(FRONTEND_DIR) && bun run test

lint:
	golangci-lint run

install:
	go install $(LDFLAGS) ./cmd/spela

clean:
	rm -f $(BINARY)
	rm -rf $(FRONTEND_DIR)/dist $(FRONTEND_DIR)/node_modules
