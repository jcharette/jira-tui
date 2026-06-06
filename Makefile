APP_NAME := jira-tui
MAIN_PKG := ./cmd/jira-tui
TMP_BUILD := /private/tmp/$(APP_NAME)-check
GOCACHE := /private/tmp/$(APP_NAME)-go-build-cache
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP_NAME)
export GOCACHE

.PHONY: help fmt test build build-local run tidy check clean docs-list

help:
	@printf "Targets:\n"
	@printf "  make fmt          Format Go code\n"
	@printf "  make test         Run tests\n"
	@printf "  make build        Verify build to a temp path\n"
	@printf "  make build-local  Build ./bin/jira-tui\n"
	@printf "  make run          Run the TUI\n"
	@printf "  make tidy         Tidy Go modules\n"
	@printf "  make check        Format, tidy, test, and verify build\n"
	@printf "  make clean        Remove generated binaries\n"
	@printf "  make docs-list    List project docs\n"

fmt:
	gofmt -w cmd internal

test:
	go test ./...

build:
	go build -o $(TMP_BUILD) $(MAIN_PKG)

build-local:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN) $(MAIN_PKG)

run:
	go run $(MAIN_PKG)

tidy:
	go mod tidy

check: fmt tidy test build

clean:
	rm -f $(APP_NAME) $(BIN) $(TMP_BUILD)

docs-list:
	find docs -maxdepth 3 -type f | sort
