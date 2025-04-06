# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
BINARY_NAME=parity-server
MAIN_PATH=cmd/main.go
AIR_VERSION=v1.49.0
GOPATH=$(shell go env GOPATH)
AIR=$(GOPATH)/bin/air

# Docker parameters
DOCKER_COMPOSE=docker compose
DOCKER_IMAGE_NAME=parity-server
DOCKER_TAG=latest

# Test related variables
COVERAGE_DIR=coverage
COVERAGE_PROFILE=$(COVERAGE_DIR)/coverage.out
COVERAGE_HTML=$(COVERAGE_DIR)/coverage.html
TEST_FLAGS=-race -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic
TEST_PACKAGES=./...  # This will test all packages
TEST_PATH=./test/...

# Build flags
BUILD_FLAGS=-v

# Add these lines after the existing parameters
INSTALL_PATH=/usr/local/bin

# Tool paths
GOFUMPT_PATH := $(GOPATH)/bin/gofumpt
GOIMPORTS_PATH := $(GOPATH)/bin/goimports
GOLANGCI_LINT := $(shell which golangci-lint)

# Lint configuration
LINT_FLAGS := --timeout=5m
LINT_CONFIG := .golangci.yml
LINT_OUTPUT_FORMAT := colored-line-number

.PHONY: all build test run clean deps fmt help docker-up docker-down docker-logs docker-build docker-clean docker-ps docker-exec install-air watch tools install uninstall install-lint-tools lint install-hooks migrate-env

all: clean build

build: ## Build the application
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) ./cmd
	chmod +x $(BINARY_NAME)

test: setup-coverage ## Run tests with coverage
	$(GOTEST) $(TEST_FLAGS) -v $(TEST_PACKAGES)
	@go tool cover -func=$(COVERAGE_PROFILE)
	@go tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)

setup-coverage: ## Create coverage directory
	@mkdir -p $(COVERAGE_DIR)

run:  ## Run the application
	$(GOCMD) run $(MAIN_PATH) $(ARGS)

server:  ## Start the parity server
	$(GOCMD) run $(MAIN_PATH) server

	$(GOCMD) run $(MAIN_PATH) stake --amount 10

balance:  ## Check token balances
	$(GOCMD) run $(MAIN_PATH) balance

auth:  ## Authenticate with the network
	$(GOCMD) run $(MAIN_PATH) auth

clean: ## Clean build files
	rm -f $(BINARY_NAME)
	find . -type f -name '*.test' -delete
	find . -type f -name '*.out' -delete
	rm -rf tmp/

deps: ## Download dependencies
	git submodule update --init --recursive
	$(GOMOD) download
	$(GOMOD) tidy

fmt: ## Format code using gofumpt (preferred) or gofmt
	@echo "Formatting code..."
	@if [ -x "$(GOFUMPT_PATH)" ]; then \
		echo "Using gofumpt for better formatting..."; \
		$(GOFUMPT_PATH) -l -w .; \
	else \
		echo "gofumpt not found, using standard gofmt..."; \
		$(GOFMT) ./...; \
		echo "Consider installing gofumpt for better formatting: go install mvdan.cc/gofumpt@latest"; \
	fi

imports: ## Fix imports formatting and add missing imports
	@echo "Organizing imports..."
	@if [ -x "$(GOIMPORTS_PATH)" ]; then \
		$(GOIMPORTS_PATH) -w -local github.com/theblitlabs/parity-runner .; \
	else \
		echo "goimports not found. Installing..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
		$(GOIMPORTS_PATH) -w -local github.com/theblitlabs/parity-runner .; \
	fi

format: fmt imports ## Run all formatters (gofumpt + goimports)
	@echo "All formatting completed."

lint: ## Run linting with options (make lint VERBOSE=true CONFIG=custom.yml OUTPUT=json)
	@echo "Running linters..."
	$(eval FINAL_LINT_FLAGS := $(LINT_FLAGS))
	@if [ "$(VERBOSE)" = "true" ]; then \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) -v"; \
	fi
	@if [ -n "$(CONFIG)" ]; then \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) --config=$(CONFIG)"; \
	else \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) --config=$(LINT_CONFIG)"; \
	fi
	@if [ -n "$(OUTPUT)" ]; then \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) --out-format=$(OUTPUT)"; \
	else \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) --out-format=$(LINT_OUTPUT_FORMAT)"; \
	fi
	golangci-lint run $(FINAL_LINT_FLAGS)

format-lint: format lint ## Format code and run linters in one step

check-format: ## Check code formatting without applying changes (useful for CI)
	@echo "Checking code formatting..."
	@./scripts/check_format.sh

install-lint-tools: ## Install formatting and linting tools
	@echo "Installing linting and formatting tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Tools installation complete."

install-hooks: ## Install git hooks
	@echo "Installing git hooks..."
	@./scripts/hooks/install-hooks.sh
	
install-air: ## Install air for hot reloading
	@if ! command -v air > /dev/null; then \
		echo "Installing air..." && \
		go install github.com/air-verse/air@latest; \
	fi

watch: install-air ## Run the application with hot reload
	$(AIR)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

install: build ## Install parity command globally
	@echo "Installing parity to $(INSTALL_PATH)..."
	@sudo mv $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete"

uninstall: ## Remove parity command from system
	@echo "Uninstalling parity from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstallation complete"

migrate-env: ## Migrate config.yaml to .env file
	$(GORUN) scripts/migrate_env.go

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE_NAME):$(DOCKER_TAG) .

docker-up: ## Start Docker containers
	$(DOCKER_COMPOSE) up -d

docker-down: ## Stop Docker containers
	$(DOCKER_COMPOSE) down

docker-logs: ## View Docker container logs
	$(DOCKER_COMPOSE) logs -f

docker-clean: ## Remove Docker containers, images, and volumes
	$(DOCKER_COMPOSE) down -v
	docker rmi $(DOCKER_IMAGE_NAME):$(DOCKER_TAG)

docker-ps: ## List running Docker containers
	$(DOCKER_COMPOSE) ps

docker-exec: ## Execute command in Docker container (make docker-exec CMD="sh")
	$(DOCKER_COMPOSE) exec app $(CMD)

.DEFAULT_GOAL := help
