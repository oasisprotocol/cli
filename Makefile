include common.mk

# Each Oasis CLI example's input .in must have a corresponding output .out.
EXAMPLES := $(patsubst %.in,%.out,$(wildcard examples/*/*.in))

# Check if Go's linkers flags are set in common.mk and add them as extra flags.
ifneq ($(GOLDFLAGS),)
	GO_EXTRA_FLAGS += -ldflags $(GOLDFLAGS)
endif

# Set all target as the default target.
all: build

# Build.
build: oasis

build-windows: oasis.exe

oasis: $(shell find . -name "*.go" -type f) go.sum go.mod
	@$(PRINT) "$(MAGENTA)*** Building Go code...$(OFF)\n"
	@$(GO) build -v -o oasis $(GOFLAGS) $(GO_EXTRA_FLAGS)

oasis.exe: $(shell find . -name "*.go" -type f) go.sum go.mod
	@$(PRINT) "$(MAGENTA)*** Building for Windows...$(OFF)\n"
	GOOS=windows GOARCH=amd64 $(GO) build -v -o oasis.exe $(GOFLAGS) $(GO_EXTRA_FLAGS)

examples: $(EXAMPLES)

examples/%.out: examples/%.in oasis scripts/gen_example.sh
	@rm -f $@
	@scripts/gen_example.sh $< $@

clean-examples:
	@rm -f examples/*/*.out

# Format code.
fmt:
	@$(PRINT) "$(CYAN)*** Running Go formatters...$(OFF)\n"
	@gofumpt -w .
	@goimports -w -local github.com/oasisprotocol/cli .

# Lint code, commits and documentation.
lint-targets := lint-go lint-docs lint-git lint-go-mod-tidy

lint-go:
	@$(PRINT) "$(CYAN)*** Running Go linters...$(OFF)\n"
	@env -u GOPATH golangci-lint run --verbose

lint-git:
	@$(PRINT) "$(CYAN)*** Running gitlint...$(OFF)\n"
	@$(CHECK_GITLINT)

lint-docs:
	@$(PRINT) "$(CYAN)*** Running markdownlint-cli...$(OFF)\n"
	@npx --yes markdownlint-cli '**/*.md'

lint-go-mod-tidy:
	@$(PRINT) "$(CYAN)*** Checking go mod tidy...$(OFF)\n"
	@$(ENSURE_GIT_CLEAN)
	@$(CHECK_GO_MOD_TIDY)

lint: $(lint-targets)

# Release.
release-build:
	@goreleaser release --clean

# Test.
test-targets := test-unit

test-unit:
	@$(PRINT) "$(CYAN)*** Running unit tests...$(OFF)\n"
	@$(GO) test -v -race ./...

test: $(test-targets)

# Clean.
clean:
	@$(PRINT) "$(CYAN)*** Cleaning up ...$(OFF)\n"
	@$(GO) clean -x
	rm -f oasis
	$(GO) clean -testcache

# List of targets that are not actual files.
.PHONY: \
	all build \
	build-windows \
	examples \
	clean-examples \
	fmt \
	$(lint-targets) lint \
	$(test-targets) test \
	clean
