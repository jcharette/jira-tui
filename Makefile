APP_NAME := jira
MAIN_PKG := ./cmd/jira-tui
TMPDIR ?= /tmp
TMP_BUILD := $(TMPDIR)/$(APP_NAME)-check
GOCACHE := $(TMPDIR)/$(APP_NAME)-go-build-cache
GOFLAGS := -buildvcs=false
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP_NAME)
USER_BIN_DIR := $(HOME)/bin
USER_BIN := $(USER_BIN_DIR)/$(APP_NAME)
export GOCACHE
export GOFLAGS

.PHONY: help fmt test build build-local install-user run tidy check clean docs-list docs-status docs-check milestone-complete release

help:
	@printf "Targets:\n"
	@printf "  make fmt          Format Go code\n"
	@printf "  make test         Run tests\n"
	@printf "  make build        Verify build to a temp path\n"
	@printf "  make build-local  Build ./bin/jira\n"
	@printf "  make install-user Build ~/bin/jira for everyday use\n"
	@printf "  make run          Run the TUI\n"
	@printf "  make tidy         Tidy Go modules\n"
	@printf "  make check        Format, tidy, test, and verify build\n"
	@printf "  make clean        Remove generated binaries\n"
	@printf "  make docs-list    List project docs\n"
	@printf "  make docs-status  Show roadmap milestone status\n"
	@printf "  make docs-check   Verify required docs exist\n"
	@printf "  make milestone-complete M=M0  Mark a roadmap milestone complete\n"
	@printf "  make release VERSION=0.1.0   Move Unreleased changelog entries to a release\n"

fmt:
	gofmt -w cmd internal

test:
	go test ./...

build:
	go build -o $(TMP_BUILD) $(MAIN_PKG)

build-local:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN) $(MAIN_PKG)

install-user:
	mkdir -p $(USER_BIN_DIR)
	go build -o $(USER_BIN) $(MAIN_PKG)

run:
	go run $(MAIN_PKG)

tidy:
	go mod tidy

check: fmt tidy test build

clean:
	rm -f $(APP_NAME) $(BIN) $(TMP_BUILD)

docs-list:
	find docs -maxdepth 3 -type f | sort

docs-status:
	sh scripts/docs/status.sh

docs-check:
	test -f docs/README.md
	test -f docs/roadmap.md
	test -f docs/planning.md
	test -f docs/backlog.md
	test -f docs/project-state.md
	test -f docs/releases/CHANGELOG.md
	test -f docs/working-agreement.md
	test -d docs/decisions

milestone-complete:
	test -n "$(M)"
	sh scripts/docs/milestone-complete.sh "$(M)"

release:
	test -n "$(VERSION)"
	sh scripts/docs/release.sh "$(VERSION)"
