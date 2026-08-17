BINARY := kerberoskeepalive
GOPATH_BIN := $(shell go env GOPATH)/bin

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "%-14s %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the kerberoskeepalive binary
	go build -o $(BINARY) .

.PHONY: test
test: ## Run unit tests
	go test ./... -cover

.PHONY: fmt
fmt: ## Check gofmt formatting (non-mutating; use `go fmt ./...` to fix)
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

.PHONY: lint
lint: ## Run golangci-lint
	PATH="$(GOPATH_BIN):$$PATH" golangci-lint run ./...

.PHONY: security
security: ## Run govulncheck (known CVEs) and gosec (static analysis)
	PATH="$(GOPATH_BIN):$$PATH" govulncheck ./...
	gosec -quiet ./...

.PHONY: docs
docs: ## Regenerate docs/ from the Cobra command tree
	go run ./tools/gendocs

.PHONY: hooks
hooks: ## Install the lefthook git hooks (gitleaks secret-detection pre-commit)
	lefthook install

.PHONY: pipeline
pipeline: fmt lint security test build ## Run the full local pipeline (what CI also runs, minus release)

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BINARY) dist
