# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is envctl

A Go CLI tool for managing environment variables across multi-repo projects with SOPS/age encryption. It replaces scattered `.env` files by providing a single source of truth: a template (`.env.example`) + encrypted secrets (`.envctl/*.enc.yaml`) + config (`.envctl.yaml`).

## Build & Development Commands

```bash
make build            # Build binary (version injected from git tags)
make test             # Run tests with race detector (-race -count=1)
make lint             # Run golangci-lint
make check            # Full pre-commit: mod-verify + lint + test
make vulncheck        # Security vulnerability check (govulncheck)
make clean            # Remove binary and coverage files
make hooks            # Install pre-commit git hook
```

Run a single test:
```bash
go test -race -run TestName ./internal/resolver/
```

## Architecture

**Data flow**: Template + Config → Parse → Load Secrets → Resolve → Render/Report

```
cmd/           CLI commands (cobra). Each file = one command.
internal/
  config/      Loads .envctl.yaml, walks up dirs to find it
  template/    Parser interface with two implementations:
                 DotenvParser  (*.env* files)
                 GenericParser (any text file, regex substitution)
  secret/      Provider interface:
                 SOPSProvider  (shells out to `sops --decrypt`)
                 MemoryProvider (testing)
  resolver/    Resolution priority: overrides > secrets > defaults > empty
               Produces ResolvedSet with vars, diagnostics, unused secrets
  pipeline/    Orchestrates ParseTemplate() and ResolveEnv()
  reporter/    Color-coded console output for verify results
```

**Variable classification** (in `template/variable.go`):
- **Static**: `VAR=hardcoded` -- literal, no substitution
- **RequiredSecret**: `VAR=${VAR}` -- must exist in secret store
- **OptionalSecret**: `VAR=${VAR:-default}` -- falls back to default

## Key Interfaces

```go
// template/parser.go
type Parser interface {
    Parse(r io.Reader) (*ParseResult, error)
    Render(result *ParseResult, values map[string]string) string
}

// secret/provider.go
type Provider interface {
    Load(path string) (map[string]string, error)
}
```

## CLI Commands

- `init` -- Bootstrap envctl in a project (detects template, creates config)
- `verify` -- Check all required secrets are present (exit 0/1/2)
- `generate` -- Produce .env file from template + secrets
- `diff` -- Compare two environments side-by-side
- `edit` -- Open encrypted secrets in $EDITOR via sops
- `keys list|add|remove [--env]` -- Manage age encryption recipients (global, or per environment with `--env`); add/remove re-key and rotate the secrets files

Persistent flags: `--config` (path to .envctl.yaml), `--env` (target environment name).

## Linting

Uses golangci-lint v2 with gofumpt formatting. Key enabled linters: errcheck, govet, staticcheck, errorlint, gocritic, revive, misspell, gosec. Config in `.golangci.yml`.

## Release

GoReleaser builds multi-arch binaries (linux/darwin, amd64/arm64). Docker images pushed to `ghcr.io/pillowpillow/envctl`. CI runs on GitHub Actions (`ci.yaml` for PR checks, `release.yaml` on tag push).

## GitHub Actions Integration

- `action.yaml` -- Composite action for consumers to use envctl as a step (verify/generate)
- `.github/workflows/envctl-verify.yaml` -- Reusable workflow for standalone verify checks
- Consumer repos use the composite action in their build job; the `.env` stays in the workspace (no artifacts).
