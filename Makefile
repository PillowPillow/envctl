VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test lint clean release-dry-run mod-verify mod-tidy vulncheck check hooks brand

build:
	go build $(LDFLAGS) -o envctl .

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

mod-verify:
	go mod verify

mod-tidy:
	go mod tidy
	git diff --exit-code go.mod go.sum

vulncheck:
	govulncheck ./...

check: mod-verify lint test

clean:
	rm -f envctl coverage.out

release-dry-run:
	goreleaser release --snapshot --clean

hooks:
	@printf '#!/bin/sh\nmake mod-verify lint\n' > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed"

brand:
	go run ./docs/brand .
