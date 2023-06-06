include common.mk

# Check if Go's linkers flags are set in common.mk and add them as extra flags.
ifneq ($(GOLDFLAGS),)
	GO_EXTRA_FLAGS += -ldflags $(GOLDFLAGS)
endif

# Set all target as the default target.
all: build

# Build.
build:
	@$(PRINT) "$(MAGENTA)*** Building Go code...$(OFF)\n"
	@$(GO) build -v -o oasis $(GOFLAGS) $(GO_EXTRA_FLAGS)

# Format code.
fmt:
	@$(PRINT) "$(CYAN)*** Running Go formatters...$(OFF)"
	@gofumpt -w .
	@goimports -w -local github.com/oasisprotocol/cli .

# Lint code, commits and documentation.
lint-targets := lint-go lint-docs lint-git lint-go-mod-tidy

lint-go:
	@$(PRINT) "$(CYAN)*** Running Go linters...$(OFF)"
	@env -u GOPATH golangci-lint run --verbose

lint-git:
	@$(PRINT) "$(CYAN)*** Running gitlint...$(OFF)"
	@$(CHECK_GITLINT)

lint-docs:
	@$(PRINT) "$(CYAN)*** Running markdownlint-cli...$(OFF)"
	@npx --yes markdownlint-cli '**/*.md'

lint-go-mod-tidy:
	@$(PRINT) "$(CYAN)*** Checking go mod tidy...$(OFF)"
	@$(ENSURE_GIT_CLEAN)
	@$(CHECK_GO_MOD_TIDY)

lint: $(lint-targets)

# Release.
release-build:
	@goreleaser release --rm-dist

# Test.
test-targets := test-unit

test-unit:
	@$(PRINT) "$(CYAN)*** Running unit tests...$(OFF)"
	@$(GO) test -v -race ./...

test: $(test-targets)

# Clean.
clean:
	@$(PRINT) "$(CYAN)*** Cleaning up ...$(OFF)"
	@$(GO) clean -x
	rm -f oasis
	$(GO) clean -testcache

# List of targets that are not actual files.
.PHONY: \
	all build \
	fmt \
	$(lint-targets) lint \
	$(test-targets) test \
	clean
