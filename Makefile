.PHONY: build build-local build-docker install fmt test test-race test-git test-sync \
       test-helpers test-all test-coverage test-coverage-func lint clean release \
       release-dry release-check examples

## Build targets

build: ## Multi-platform build via GoReleaser snapshot
	go tool goreleaser build --snapshot --clean

build-local: fmt ## Fast local dev build
	@mkdir -p dist
	go build -ldflags "-X github.com/blairham/ghorg/internal/cmd.version=$$(git describe --tags --always --dirty)" -o dist/ghorg .

build-docker: ## Build Docker images locally (no push)
	go tool goreleaser release --snapshot --clean --skip=publish

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

release: test ## Full GoReleaser release (requires GITHUB_TOKEN)
	go tool goreleaser release --clean

release-dry: ## Dry-run release (no publish)
	go tool goreleaser release --snapshot --clean

release-check: ## Validate goreleaser configuration
	go tool goreleaser check

## Misc targets

examples: ## Copy example files
	cp -rf examples/ internal/cmd/examples-copy/

clean: ## Remove build artifacts
	rm -rf dist coverage.out coverage.html

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
