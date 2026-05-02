BINARY      := jcli
BIN_DIR     := bin
MODULE      := $(shell go list -m)

PLATFORMS   := linux/amd64 darwin/amd64 darwin/arm64 windows/amd64

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
		awk 'BEGIN {FS = ":.*##"}; {printf "  %-12s %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary for the current platform (output: bin/jcli)
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .
	@echo "Built $(BIN_DIR)/$(BINARY)"

.PHONY: run
run: build ## Build then run the binary (no arguments)
	./$(BIN_DIR)/$(BINARY)

.PHONY: release
release: ## Build cross-platform binaries and create a GitHub release (requires VERSION=vX.Y.Z)
ifndef VERSION
	$(error VERSION is not set. Usage: make release VERSION=v1.0.0)
endif
	@mkdir -p $(BIN_DIR)
	@echo "Building release $(VERSION) for all platforms..."
	$(foreach PLATFORM,$(PLATFORMS), \
		$(eval OS   := $(word 1,$(subst /, ,$(PLATFORM)))) \
		$(eval ARCH := $(word 2,$(subst /, ,$(PLATFORM)))) \
		$(eval EXT  := $(if $(filter windows,$(OS)),.exe,)) \
		$(eval OUT  := $(BIN_DIR)/$(BINARY)-$(OS)-$(ARCH)$(EXT)) \
		GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -ldflags="$(RELEASE_LDFLAGS)" -o $(OUT) . && \
		echo "  Built $(OUT)" ; \
	)
	@echo "Tagging $(VERSION)..."
	git tag $(VERSION)
	git push origin $(VERSION)
	@echo "Creating GitHub release $(VERSION)..."
	gh release create $(VERSION) \
		--title "$(VERSION)" \
		--generate-notes \
		$(BIN_DIR)/$(BINARY)-linux-amd64 \
		$(BIN_DIR)/$(BINARY)-darwin-amd64 \
		$(BIN_DIR)/$(BINARY)-darwin-arm64 \
		$(BIN_DIR)/$(BINARY)-windows-amd64.exe
	@echo "Release $(VERSION) published."

.PHONY: clean
clean: ## Remove the bin/ directory
	rm -rf $(BIN_DIR)
	@echo "Cleaned $(BIN_DIR)/"
