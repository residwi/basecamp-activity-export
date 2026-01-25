# Load the .env file if it exists and export variables
-include .env
export

.PHONY: build clean install test test-coverage test-ci help deps

BINARY_NAME=bclog
BUILD_DIR=bin
CMD_DIR=cmd/bclog

help:
	@echo "bclog - Makefile commands"
	@echo ""
	@echo "Usage:"
	@echo "  make build          Build the binary"
	@echo "  make install        Install to GOPATH/bin"
	@echo "  make clean          Remove built binaries"
	@echo "  make test           Run all tests"
	@echo "  make test-coverage  Run tests with coverage report"
	@echo "  make test-ci        Run tests with coverage threshold (70%)"
	@echo "  make deps           Download dependencies"
	@echo ""

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Binary created at $(BUILD_DIR)/$(BINARY_NAME)"

install:
	@echo "Installing $(BINARY_NAME) to GOPATH/bin..."
	@go install ./$(CMD_DIR)
	@echo "Installed successfully"

clean:
	@echo "Cleaning build artifacts..."
	@rm $(BUILD_DIR)/$(BINARY_NAME)
	@go clean
	@echo "Clean complete"

test:
	@echo "Running all tests..."
	@go test -v -race -failfast ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out | grep total

test-ci:
	@echo "Running tests with coverage threshold check..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@echo "Checking coverage threshold (70%)..."
	@go tool cover -func=coverage.out | grep total | awk '{if ($$3 < 70.0) {print "Coverage below 70% threshold:", $$3; exit 1} else {print "Coverage passed:", $$3}}'

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"

.DEFAULT_GOAL := help
