# CringeSweeper Makefile

# Version information
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Go build variables
BINARY_NAME = cringesweeper
LDFLAGS = -X github.com/gerrowadat/cringesweeper/internal.Version=${VERSION} \
          -X github.com/gerrowadat/cringesweeper/internal.Commit=${COMMIT} \
          -X github.com/gerrowadat/cringesweeper/internal.BuildTime=${BUILD_TIME}

# Docker variables
DOCKER_IMAGE = cringesweeper
DOCKER_TAG ?= ${VERSION}

.PHONY: help build clean test vet fmt docker-build docker-run docker-clean version info

# Default target
help: ## Show this help message
	@echo "CringeSweeper Build System"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build targets

build: ## Build the binary with version information
	@echo "Building ${BINARY_NAME} ${VERSION}"
	@echo "Commit: ${COMMIT}"
	@echo "Build Time: ${BUILD_TIME}"
	@go build -ldflags "${LDFLAGS}" -o ${BINARY_NAME} .
	@echo "✅ Build complete: ${BINARY_NAME}"

build-dev: ## Build the binary without version information (faster for development)
	@go build -o ${BINARY_NAME} .
	@echo "✅ Dev build complete: ${BINARY_NAME}"

clean: ## Remove built binaries and Docker images
	@echo "Cleaning up..."
	@rm -f ${BINARY_NAME}
	@docker rmi ${DOCKER_IMAGE}:${DOCKER_TAG} 2>/dev/null || true
	@echo "✅ Cleanup complete"

##@ Code Quality

test: ## Run tests
	@echo "Running tests..."
	@go test ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

fmt: ## Format code
	@echo "Formatting code..."
	@gofmt -w .

lint: vet fmt ## Run linting and formatting

##@ Docker targets

docker-build: ## Build Docker image with version information
	@echo "Building Docker image ${DOCKER_IMAGE}:${DOCKER_TAG}"
	@docker build \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_TIME=${BUILD_TIME} \
		-t ${DOCKER_IMAGE}:${DOCKER_TAG} .
	@echo "✅ Docker build complete: ${DOCKER_IMAGE}:${DOCKER_TAG}"

docker-build-latest: ## Build Docker image and tag as latest
	@$(MAKE) docker-build
	@docker tag ${DOCKER_IMAGE}:${DOCKER_TAG} ${DOCKER_IMAGE}:latest
	@echo "✅ Tagged as ${DOCKER_IMAGE}:latest"

docker-run: ## Run Docker container with example Bluesky configuration
	@echo "Running ${DOCKER_IMAGE}:${DOCKER_TAG} in server mode..."
	@echo "Note: Set BLUESKY_USERNAME and BLUESKY_APP_PASSWORD environment variables"
	@docker run -d \
		--name cringesweeper-server \
		-p 8080:8080 \
		-e BLUESKY_USERNAME=${BLUESKY_USERNAME} \
		-e BLUESKY_APP_PASSWORD=${BLUESKY_APP_PASSWORD} \
		-e LOG_LEVEL=info \
		${DOCKER_IMAGE}:${DOCKER_TAG} \
		server --platform=bluesky --max-post-age=30d --preserve-pinned --prune-interval=1h
	@echo "✅ Container started. Access health check at http://localhost:8080"
	@echo "   Metrics available at http://localhost:8080/metrics"

docker-run-env: ## Run Docker container with .env file
	@if [ ! -f .env ]; then \
		echo "❌ .env file not found. Copy .env.example to .env and configure it."; \
		exit 1; \
	fi
	@echo "Running ${DOCKER_IMAGE}:${DOCKER_TAG} with .env file..."
	@docker run -d \
		--name cringesweeper-server \
		-p 8080:8080 \
		--env-file .env \
		${DOCKER_IMAGE}:${DOCKER_TAG} \
		server --platform=bluesky --max-post-age=30d --preserve-pinned
	@echo "✅ Container started with .env configuration"

docker-stop: ## Stop running Docker container
	@docker stop cringesweeper-server 2>/dev/null || true
	@docker rm cringesweeper-server 2>/dev/null || true
	@echo "✅ Container stopped and removed"

docker-logs: ## Show Docker container logs
	@docker logs -f cringesweeper-server

docker-shell: ## Open shell in running container
	@docker exec -it cringesweeper-server sh

docker-clean: ## Remove Docker images and containers
	@echo "Removing Docker containers and images..."
	@docker stop cringesweeper-server 2>/dev/null || true
	@docker rm cringesweeper-server 2>/dev/null || true
	@docker rmi ${DOCKER_IMAGE}:${DOCKER_TAG} 2>/dev/null || true
	@docker rmi ${DOCKER_IMAGE}:latest 2>/dev/null || true
	@echo "✅ Docker cleanup complete"

##@ Release targets

version: build ## Build and show version information
	@./$(BINARY_NAME) version

tag: ## Create a new git tag (use VERSION=x.y.z)
	@if [ "${VERSION}" = "dev" ]; then \
		echo "❌ Please specify VERSION=x.y.z"; \
		exit 1; \
	fi
	@echo "Creating tag v${VERSION}..."
	@git tag -a v${VERSION} -m "Release version ${VERSION}"
	@echo "✅ Tag v${VERSION} created. Push with: git push origin v${VERSION}"

release: ## Build release binaries for multiple platforms
	@echo "Building release binaries for version ${VERSION}..."
	@mkdir -p dist
	@GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/${BINARY_NAME}-linux-amd64 .
	@GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/${BINARY_NAME}-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o dist/${BINARY_NAME}-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/${BINARY_NAME}-windows-amd64.exe .
	@echo "✅ Release binaries built in dist/"
	@ls -la dist/

##@ Information

info: ## Show build information
	@echo "Build Information:"
	@echo "  Version:    ${VERSION}"
	@echo "  Commit:     ${COMMIT}"
	@echo "  Build Time: ${BUILD_TIME}"
	@echo "  Binary:     ${BINARY_NAME}"
	@echo "  Docker:     ${DOCKER_IMAGE}:${DOCKER_TAG}"

env-example: ## Copy .env.example to .env for configuration
	@if [ -f .env ]; then \
		echo "❌ .env already exists. Remove it first or edit manually."; \
	else \
		cp .env.example .env; \
		echo "✅ Copied .env.example to .env. Edit it with your credentials."; \
	fi

##@ Development

dev: build-dev ## Quick development build and run
	@./$(BINARY_NAME) --help

serve: build ## Build and run server in development mode
	@echo "Starting development server..."
	@./$(BINARY_NAME) server --dry-run --platform=bluesky --max-post-age=7d --prune-interval=30s --log-level=debug

##@ CI/CD

ci: lint test build ## Run CI pipeline (lint, test, build)
	@echo "✅ CI pipeline completed successfully"

docker-ci: ci docker-build ## Run CI pipeline and build Docker image
	@echo "✅ CI pipeline with Docker build completed successfully"