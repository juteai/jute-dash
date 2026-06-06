SHELL := /bin/bash

WEB_DIR := apps/web
NPM ?= npm

.PHONY: setup pre-commit-install lint test test-coverage generate-mocks web-lint web-format-check web-check web-test web-test-coverage web-build check reset

setup:
	@echo "Checking for Homebrew dependencies..."
	@if command -v brew >/dev/null 2>&1; then \
		echo "Running brew bundle install..."; \
		brew bundle install; \
	else \
		echo "Homebrew not found. Skipping system package installation."; \
	fi
	@echo "Setting up web app dependencies..."
	cd $(WEB_DIR) && $(NPM) install
	@echo "Setting up pre-commit hooks..."
	$(MAKE) pre-commit-install

pre-commit-install:
	@if command -v pre-commit >/dev/null 2>&1; then \
		echo "Installing pre-commit hooks..."; \
		pre-commit install --install-hooks -t pre-commit -t commit-msg; \
	else \
		echo "pre-commit command not found. Attempting brew install..."; \
		if command -v brew >/dev/null 2>&1; then \
			brew install pre-commit; \
			pre-commit install --install-hooks -t pre-commit -t commit-msg; \
		else \
			echo "Failed to install pre-commit automatically. Please install pre-commit manually (https://pre-commit.com)."; \
		fi \
	fi

lint:
	cd apps/hub && golangci-lint run --timeout=5m

test:
	cd apps/hub && go test ./...

test-coverage:
	cd apps/hub && go test -coverprofile=coverage.out ./...
	cd apps/hub && go tool cover -func=coverage.out

generate-mocks:
	go run github.com/vektra/mockery/v2@latest --config=.mockery.yaml
	go run github.com/vektra/mockery/v2@latest --config=/dev/null --name=Syncer --srcpkg=jute-dash/apps/hub/internal/app/agents --output=apps/hub/internal/app/agents --filename=agent_syncer_mock_test.go --structname=AgentSyncer --with-expecter --inpackage --testonly

web-lint:
	cd $(WEB_DIR) && $(NPM) run lint

web-format-check:
	cd $(WEB_DIR) && $(NPM) run format:check

web-check:
	cd $(WEB_DIR) && $(NPM) run check

web-test:
	cd $(WEB_DIR) && $(NPM) run test

web-test-coverage:
	cd $(WEB_DIR) && $(NPM) run test:coverage

web-build:
	cd $(WEB_DIR) && $(NPM) run build

check: generate-mocks lint test web-lint web-format-check web-check web-test web-build

reset:
	@echo "Resetting development store directories..."
	rm -rf .jute/dev-mock-a2a .jute/dev-kronk-a2a .jute/dev-kronk-a2a-mcp
