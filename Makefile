BINARY      := jcli
BIN_DIR     := bin
DIST_DIR    := dist
FORMULA     := Formula/jcli.rb
SKILL_SRC   := .agents/skills/jira/SKILL.md
MODULE      := $(shell go list -m)
GITHUB_REPO := steveohara/jcli

# Platforms for the local dev build and Windows release asset
PLATFORMS   := linux/amd64 darwin/amd64 darwin/arm64 windows/amd64

# Platforms packaged as tarballs for the Homebrew formula
HOMEBREW_PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

GO          := go

# ── Version info injected at build time ──────────────────────────────────────

VERSION_PKG := $(MODULE)/internal/version
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# ldflags for a regular build (uses the current git state for the version).
LDFLAGS := -s -w \
	-X '$(VERSION_PKG).Version=$(GIT_VERSION)' \
	-X '$(VERSION_PKG).Commit=$(GIT_COMMIT)' \
	-X '$(VERSION_PKG).BuildDate=$(BUILD_DATE)'

# ldflags for a release build (version is the explicit VERSION variable).
RELEASE_LDFLAGS := -s -w \
	-X '$(VERSION_PKG).Version=$(VERSION)' \
	-X '$(VERSION_PKG).Commit=$(GIT_COMMIT)' \
	-X '$(VERSION_PKG).BuildDate=$(BUILD_DATE)'

.DEFAULT_GOAL := help

# ── Targets ──────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "  %-14s %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary for the current platform (output: bin/jcli)
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .
	@echo "Built $(BIN_DIR)/$(BINARY)"

.PHONY: run
run: build ## Build then run the binary (no arguments)
	./$(BIN_DIR)/$(BINARY)

# ── Distribution / Homebrew ──────────────────────────────────────────────────

.PHONY: dist
dist: ## [1/3] Compile and package tarballs into dist/ — local only (requires VERSION=vX.Y.Z)
ifndef VERSION
	$(error VERSION is not set. Usage: make dist VERSION=v1.0.0)
endif
	@rm -rf $(DIST_DIR) && mkdir -p $(DIST_DIR)
	@echo "Building $(VERSION) tarballs for Homebrew platforms..."
	@$(foreach PLATFORM,$(HOMEBREW_PLATFORMS), \
		$(eval OS   := $(word 1,$(subst /, ,$(PLATFORM)))) \
		$(eval ARCH := $(word 2,$(subst /, ,$(PLATFORM)))) \
		$(eval TAR  := $(DIST_DIR)/$(BINARY)-$(VERSION)-$(OS)-$(ARCH).tar.gz) \
		GOOS=$(OS) GOARCH=$(ARCH) $(GO) build \
			-ldflags="$(RELEASE_LDFLAGS)" \
			-o $(DIST_DIR)/$(BINARY) . && \
		cp $(SKILL_SRC) $(DIST_DIR)/SKILL.md && \
		tar -czf $(TAR) -C $(DIST_DIR) $(BINARY) SKILL.md && \
		rm $(DIST_DIR)/$(BINARY) $(DIST_DIR)/SKILL.md && \
		echo "  $(TAR)" ; \
	)
	@echo "Also building Windows amd64 binary..."
	@GOOS=windows GOARCH=amd64 $(GO) build \
		-ldflags="$(RELEASE_LDFLAGS)" \
		-o $(DIST_DIR)/$(BINARY)-$(VERSION)-windows-amd64.exe .
	@echo "  $(DIST_DIR)/$(BINARY)-$(VERSION)-windows-amd64.exe"

.PHONY: formula
formula: dist ## [2/3] Run dist then rewrite Formula/jcli.rb with new checksums — local only (requires VERSION=vX.Y.Z)
ifndef VERSION
	$(error VERSION is not set. Usage: make formula VERSION=v1.0.0)
endif
	@bash scripts/update-formula.sh $(VERSION) $(DIST_DIR) $(GITHUB_REPO)

.PHONY: release
release: formula ## [3/3] Run formula then commit, tag, push and publish GitHub release (requires VERSION=vX.Y.Z)
ifndef VERSION
	$(error VERSION is not set. Usage: make release VERSION=v1.0.0)
endif
	@echo "Committing updated formula..."
	git add $(FORMULA)
	git commit -m "chore: update Homebrew formula for $(VERSION)"
	@echo "Tagging $(VERSION) and pushing..."
	git tag $(VERSION)
	git push origin HEAD $(VERSION)
	@echo "Creating GitHub release $(VERSION)..."
	gh release create $(VERSION) \
		--title "$(VERSION)" \
		--generate-notes \
		$(DIST_DIR)/$(BINARY)-$(VERSION)-darwin-amd64.tar.gz \
		$(DIST_DIR)/$(BINARY)-$(VERSION)-darwin-arm64.tar.gz \
		$(DIST_DIR)/$(BINARY)-$(VERSION)-linux-amd64.tar.gz \
		$(DIST_DIR)/$(BINARY)-$(VERSION)-linux-arm64.tar.gz \
		$(DIST_DIR)/$(BINARY)-$(VERSION)-windows-amd64.exe
	@echo "Release $(VERSION) published."

.PHONY: test
test: ## Run all tests
	$(GO) test ./...

.PHONY: test-race
test-race: ## Run tests with race detector
	$(GO) test -race ./...

.PHONY: coverage
coverage: ## Generate HTML coverage report (opens in browser)
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

.PHONY: clean
clean: ## Remove build artefacts (bin/ and dist/)
	rm -rf $(BIN_DIR) $(DIST_DIR)
	@echo "Cleaned $(BIN_DIR)/ and $(DIST_DIR)/"
