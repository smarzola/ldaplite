.PHONY: help build build-css test docker-build docker-run docker-stop clean lint install-tools

# Variables
BINARY_NAME=ldaplite
VERSION?=0.1.0
GO=go
DOCKER=docker
LDFLAGS=-ldflags "-w -s"

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build              Build the binary (with CSS)"
	@echo "  build-css          Build Tailwind CSS"
	@echo "  test               Run tests"
	@echo "  test-race          Run tests with race detector"
	@echo "  test-coverage      Run tests with coverage report"
	@echo "  lint               Run linter"
	@echo "  fmt                Format code"
	@echo "  install-tools      Install development tools"
	@echo "  docker-build       Build Docker image"
	@echo "  docker-run         Start Docker container with compose"
	@echo "  docker-stop        Stop Docker container"
	@echo "  docker-logs        Show Docker logs"
	@echo "  clean              Clean build artifacts"

build-css:
	@echo "Building Tailwind CSS..."
	@if [ ! -d "node_modules" ]; then \
		echo "Installing Node dependencies..."; \
		npm install; \
	fi
	npm run build:css
	@echo "CSS built: internal/web/static/output.css"

build: build-css
	@echo "Building ${BINARY_NAME}..."
	CGO_ENABLED=0 ${GO} build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/ldaplite
	@echo "Binary created: bin/${BINARY_NAME}"

build-static:
	@echo "Building static binary..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ${GO} build \
		-ldflags='-w -s -extldflags "-static"' \
		-o bin/${BINARY_NAME}-linux-amd64 \
		./cmd/ldaplite
	@echo "Static binary created: bin/${BINARY_NAME}-linux-amd64"

test:
	@echo "Running tests..."
	${GO} test -v -race ./...

test-race:
	@echo "Running tests with race detector..."
	${GO} test -race ./...

test-coverage:
	@echo "Running tests with coverage..."
	${GO} test -v -race -coverprofile=coverage.out ./...
	${GO} tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	@echo "Running linter..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	${GO} fmt ./...

install-tools:
	@echo "Installing development tools..."
	${GO} install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

docker-build:
	@echo "Building Docker image..."
	${DOCKER} build -t ${BINARY_NAME}:${VERSION} .
	${DOCKER} build -t ${BINARY_NAME}:latest .
	@echo "Image built: ${BINARY_NAME}:${VERSION}"

docker-run:
	@echo "Starting container with Docker Compose..."
	mkdir -p data
	${DOCKER} compose up -d
	@echo "Container started. Access LDAP at localhost:3389"

docker-stop:
	@echo "Stopping Docker container..."
	${DOCKER} compose down

docker-logs:
	@echo "Showing Docker logs..."
	${DOCKER} compose logs -f ldaplite

docker-shell:
	@echo "Opening shell in container..."
	${DOCKER} compose exec ldaplite /bin/bash

docker-push:
	@echo "Pushing Docker image..."
	${DOCKER} push ${REGISTRY}/${BINARY_NAME}:${VERSION}
	${DOCKER} push ${REGISTRY}/${BINARY_NAME}:latest

clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f internal/web/static/output.css
	${DOCKER} compose down -v
	@echo "Clean complete"

# Development run (requires setting env vars)
dev-run: build
	@echo "Starting development server..."
	mkdir -p /tmp/ldaplite-data
	export LDAP_DATABASE_PATH=/tmp/ldaplite-data/ldaplite.db && \
	export LDAP_BASE_DN=dc=example,dc=com && \
	export LDAP_ADMIN_PASSWORD=admin123 && \
	export LDAP_LOG_LEVEL=debug && \
	./bin/ldaplite server

# Testing LDAP connection
ldap-test:
	ldapsearch -H ldap://localhost:3389 \
		-D "cn=admin,dc=example,dc=com" \
		-w ChangeMe123! \
		-b "dc=example,dc=com" \
		"(objectClass=*)"

ldap-whoami:
	ldapwhoami -H ldap://localhost:3389 \
		-D "cn=admin,dc=example,dc=com" \
		-w ChangeMe123!

# Version information
version:
	@echo "LDAPLite version: ${VERSION}"
	${GO} version

# Show help on empty target
.DEFAULT_GOAL := help
