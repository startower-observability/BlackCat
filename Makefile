# BlackCat Makefile
# Usage: make [target]

BINARY      := blackcat
GOOS        ?= $(shell go env GOOS)
GOARCH      ?= $(shell go env GOARCH)
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo '')
LDFLAGS     := -X github.com/startower-observability/blackcat/internal/version.Version=$(VERSION) \
               -X github.com/startower-observability/blackcat/internal/version.Commit=$(COMMIT) \
               -X github.com/startower-observability/blackcat/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: build build-linux test vet deploy deploy-no-push verify clean help web-install web build-all dev-web

## build: Build binary for current OS/arch
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

## build-linux: Cross-compile for Linux amd64 (for VM deploy)
build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 .

## test: Run all tests
test:
	CGO_ENABLED=0 go test ./...

## vet: Run go vet
vet:
	go vet ./...

## deploy: Deploy to VM (push, build on VM, install, restart, health check)
deploy:
	bash scripts/deploy.sh

## deploy-no-push: Deploy without git push (useful for quick redeploys)
deploy-no-push:
	bash scripts/deploy.sh --no-push

## verify: Run health check against deployed VM
verify:
	@bash -c 'source deploy/deploy.env 2>/dev/null || true; \
	  HOST=$${DEPLOY_HOST:-}; \
	  if [ -z "$$HOST" ]; then echo "DEPLOY_HOST not set in deploy/deploy.env"; exit 1; fi; \
	  URL="http://$$HOST:8080/health"; \
	  echo "Checking $$URL ..."; \
	  curl -sf "$$URL" && echo " OK" || (echo " FAIL"; exit 1)'

## clean: Remove built binary artifacts
clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64

## web-install: Install web dependencies (npm ci)
web-install:
	cd web && npm ci

## web: Build React SPA (outputs to internal/dashboard/dist)
web:
	cd web && npm run build

## build-all: Build React SPA then Go binary with embedded assets
build-all: web
	CGO_ENABLED=1 go build -tags fts5 -ldflags "$(LDFLAGS)" -o blackcat .

## dev-web: Start Vite dev server (proxies /dashboard/api to :8081)
dev-web:
	cd web && npm run dev

## help: Show this help message
help:
	@grep -E '^## [a-z]' Makefile | sed 's/## /  make /' | column -t -s ':'
