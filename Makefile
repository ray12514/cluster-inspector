# Build and dev tasks for cluster-inspector.
#
# Targets:
#   build         compile the binary into ./cluster-inspector
#   test          run unit tests
#   lint          run gofmt check, go vet, golangci-lint, and the
#                 shell-discipline grep
#   validate      build then run cluster-inspector --help as a smoke check
#   sync-schema   copy profile-v1.json from ../stack-planning into
#                 internal/resources/profile_schema.json
#   install-hooks enable the version-controlled git hooks
#   clean         remove the built binary

BINARY := cluster-inspector
PKG    := ./...
STACK_PLANNING ?= ../stack-planning

.PHONY: build test lint validate sync-schema install-hooks clean help

help:
	@echo "Targets: build test lint validate sync-schema install-hooks clean"

build:
	go build -o $(BINARY) ./cmd/cluster-inspector

test:
	go test $(PKG)

lint: gofmt-check vet shell-discipline golangci-lint

gofmt-check:
	@if [ -n "$$(gofmt -l . 2>&1 | grep -v '^vendor/')" ]; then \
		echo "gofmt: files need formatting:"; \
		gofmt -l . | grep -v '^vendor/'; \
		exit 1; \
	fi

vet:
	go vet $(PKG)

shell-discipline:
	@# Enforce the design doc's hard rule: no login shells in probes.
	@# See stack-planning/docs/cluster_inspector_stack_profile_design_v1.md
	@# § Shell Invocation Discipline.
	@if grep -rn 'bash -l\|bash --login\|sh -l\|bash -lc' cmd/ internal/ 2>/dev/null; then \
		echo "shell-discipline: login-shell invocation found above — forbidden by design"; \
		exit 1; \
	fi

golangci-lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed — skipping (install with: brew install golangci-lint)"; \
	fi

validate: build
	./$(BINARY) --help >/dev/null
	@echo "validate: $(BINARY) --help ran cleanly"

sync-schema:
	@if [ ! -f $(STACK_PLANNING)/schemas/profile-v1.json ]; then \
		echo "sync-schema: $(STACK_PLANNING)/schemas/profile-v1.json not found"; \
		echo "             clone stack-planning adjacent or set STACK_PLANNING=<path>"; \
		exit 1; \
	fi
	cp $(STACK_PLANNING)/schemas/profile-v1.json internal/resources/profile_schema.json
	@echo "sync-schema: copied profile-v1.json from $(STACK_PLANNING)"

install-hooks:
	./scripts/install-hooks.sh

clean:
	rm -f $(BINARY)
