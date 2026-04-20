.PHONY: build build-local build-docker install fmt test test-race test-git test-sync \
       test-helpers test-all test-coverage test-coverage-func lint clean release \
       release-dry release-check examples deps-install deps-verify

## Build targets

build: deps-verify ## Multi-platform build via GoReleaser snapshot
	goreleaser build --snapshot --clean

build-local: fmt ## Fast local dev build
	@mkdir -p dist
	go build -ldflags "-X github.com/blairham/ghorg/internal/cmd.version=$$(git describe --tags --always --dirty)" -o dist/ghorg .

build-docker: deps-verify ## Build Docker images locally (no push)
	goreleaser release --snapshot --clean --skip=publish

## Install targets

install: build-local ## Build + copy sample config to ~/.config/ghorg/
	mkdir -p $(HOME)/.config/ghorg
	cp sample-conf.yaml $(HOME)/.config/ghorg/conf.yaml

## Test targets

test: ## Run all tests
	go test ./... -v

test-race: ## Run all tests with race detector
	go test ./... -v -race

test-git: ## Run git package tests only
	go test ./internal/git -v

test-sync: ## Run sync-related tests only
	go test ./internal/git -v -run TestSync

test-helpers: ## Run git helper function tests only
	go test ./internal/git -v -run '^Test(GetRemoteURL|HasLocalChanges|HasUnpushedCommits|GetCurrentBranch|HasCommitsNotOnDefaultBranch|IsDefaultBranchBehindHead|MergeIntoDefaultBranch|UpdateRef)'

test-all: fmt lint test ## fmt + lint + test (full quality gate)
	@echo ""
	@echo "=== All Tests Complete ==="

test-coverage: ## Tests with HTML coverage report
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-coverage-func: ## Coverage with function-level detail for git helpers
	cd internal/git && go test -coverprofile=coverage.out -covermode=atomic
	mv internal/git/coverage.out coverage.out
	go tool cover -func=coverage.out
	@echo ""
	@echo "=== New Git Helper Functions Coverage ==="
	@go tool cover -func=coverage.out | grep -E '(GetRemoteURL|HasLocalChanges|HasUnpushedCommits|GetCurrentBranch|HasCommitsNotOnDefaultBranch|IsDefaultBranchBehindHead|MergeIntoDefaultBranch|UpdateRef)'

## Code quality

fmt: ## Format all Go files
	go tool gofumpt -w .

lint: ## Run golangci-lint
	go tool golangci-lint run ./...

## Release targets

release: deps-verify test ## Full GoReleaser release (requires GITHUB_TOKEN)
	goreleaser release --clean

release-dry: deps-verify ## Dry-run release (no publish)
	goreleaser release --snapshot --clean

release-check: ## Validate goreleaser configuration
	goreleaser check

## Misc targets

examples: ## Copy example files
	cp -rf examples/ internal/cmd/examples-copy/

clean: ## Remove build artifacts
	rm -rf dist coverage.out coverage.html

## Dependency management

deps-install: ## Install goreleaser
	go install github.com/goreleaser/goreleaser@latest

deps-verify: ## Check required tools are available
	@which go > /dev/null || (echo "missing: go" && exit 1)
	@which git > /dev/null || (echo "missing: git" && exit 1)
	@which goreleaser > /dev/null || (echo "missing: goreleaser" && exit 1)

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
