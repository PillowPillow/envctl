# envctl

Environment configuration manager with SOPS/age encryption.

Single source of truth for environment variables across projects: `.env.example` defines what exists, SOPS-encrypted files store the values.

## Problem

Managing environment variables in multi-repo projects is error-prone:

- **Forgotten secrets** -- new variables added to `.env.example` but not configured in CI, causing silent empty values in production
- **Manual workflow maintenance** -- every new secret requires a manual edit of the CI workflow `env:` section
- **No portable secret store** -- sensitive values live in developers' heads, local `.env` files, or scattered across cloud provider settings

## How it works

envctl parses your `.env.example` and classifies each variable:

| Syntax in `.env.example` | Classification | Behavior |
|---|---|---|
| `VAR=hardcoded` | **static** | Literal value, no substitution |
| `VAR=${VAR}` | **required-secret** | Must exist in secret store, error if missing |
| `VAR=${VAR:-default}` | **optional-secret** | Uses secret store value if present, falls back to default |

Secrets are stored in SOPS/age-encrypted YAML files committed to the repo. Keys are readable (for diffing), values are encrypted.

## Installation

### Homebrew (macOS/Linux)

```bash
# One-time setup: configure access to the private tap
export HOMEBREW_GITHUB_API_TOKEN="ghp_your_token_here"  # add to your ~/.zshrc

brew install pillowpillow/tap/envctl
```

To create your `HOMEBREW_GITHUB_API_TOKEN`:
1. Go to GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens
2. Repositories: select `PillowPillow/homebrew-tap` **and** `PillowPillow/envctl`
3. Permission: **Contents: Read-only**
4. Add `export HOMEBREW_GITHUB_API_TOKEN="ghp_..."` to your shell profile (`~/.zshrc` or `~/.bashrc`)

Update:
```bash
brew upgrade envctl
```

### Install script

```bash
curl -fsSL https://raw.githubusercontent.com/PillowPillow/envctl/main/scripts/install.sh | bash
```

Installs to `~/.envctl/bin/envctl`. Optional environment variables:
- `ENVCTL_VERSION` -- pin a specific version (default: latest)
- `GITHUB_TOKEN` -- for private repo access
- `INSTALL_DIR` -- custom install path (default: `~/.envctl/bin`)

### Self-update

```bash
envctl update
```

envctl also checks for updates in the background (once every 24h) and notifies you when a new version is available.

### Docker

```bash
docker pull ghcr.io/pillowpillow/envctl:latest
```

### From source

```bash
go install github.com/PillowPillow/envctl@latest
```

### Prerequisites

envctl requires [sops](https://github.com/getsops/sops) and [age](https://github.com/FiloSottile/age) for encryption:

```bash
# macOS
brew install sops age

# Linux
go install github.com/getsops/sops/v3/cmd/sops@latest
go install filippo.io/age/cmd/...@latest
```

## Getting started

### 1. Initialize envctl in your project

Navigate to a project that has a `.env.example` file:

```bash
cd your-project
envctl init
```

This will:
- Detect your `.env.example` and classify its variables
- Create `.envctl.yaml` (project configuration)
- Create `.envctl/` directory for encrypted secret files
- Update `.gitignore`

### 2. Generate an age key pair

```bash
age-keygen -o ~/.config/sops/age/keys.txt
```

Copy the public key (starts with `age1...`) and add it to `.envctl.yaml`:

```yaml
sops:
  age_recipients:
    - age1ql3z7hjy54pw3hyww5ayf...  # Your name
```

### 3. Create your secrets file

```bash
envctl edit --env local
```

This opens your `$EDITOR` with a YAML file. Add your secret values:

```yaml
DB_PASSWORD: my-local-password
APP_KEY: base64:your-app-key
REDIS_PASSWORD: redis-pass
```

Save and close -- sops encrypts the file automatically.

### 4. Verify your setup

```bash
envctl verify --env local
```

Output:

```
Parsing template...
  Found 52 variables (14 required, 28 optional, 10 static)

Checking secrets...
  OK 14/14 required secrets found

All checks passed.
```

### 5. Generate your .env file

```bash
envctl generate --env local --output .env
```

## Configuration

> **Note**: `envctl init` generates this file automatically. The reference below is for customizing it after initialization.

### `.envctl.yaml`

```yaml
version: "1"

# Source template
template: .env.example

# Environment definitions
environments:
  local:
    secrets: .envctl/secrets.local.enc.yaml
    overrides:
      APP_ENV: local
      APP_DEBUG: "true"

  production:
    secrets: .envctl/secrets.production.enc.yaml
    overrides:
      APP_ENV: production
      APP_DEBUG: "false"
      APP_URL: https://api.example.com
    strip:
      - DEBUG_VAR         # dev-only, remove from output
      - COMPOSE_PROJECT_NAME

# SOPS encryption recipients
sops:
  age_recipients:
    - age1ql3z7hjy54pw3hyww5ayf...   # Developer 1
    # - age1abc123...                  # Developer 2
```

### Substitution priority

When generating a `.env` file, values are resolved in this order (highest wins):

1. `overrides` from `.envctl.yaml`
2. Secret store value (SOPS file)
3. Default from `.env.example` (`${VAR:-default}`)
4. Empty string (with error if `required-secret`)

## Commands

### `envctl verify`

Check that all required secrets are present for an environment:

```bash
envctl verify --env production
envctl verify --env production --strict  # treat warnings as errors (auto-enabled in CI)
```

Exit codes: `0` all good, `1` missing required secrets, `2` config error.

### `envctl generate`

Produce a `.env` file from template + secrets + overrides:

```bash
envctl generate --env local --output .env
envctl generate --env production --output env-production --strict
envctl generate --env local  # outputs to stdout
```

### `envctl diff`

Compare resolved values between two environments:

```bash
envctl diff --from staging --to production
envctl diff --from local --to production --reveal  # show actual values
```

Output:

```
Comparing staging -> production:

  APP_URL:
    staging:    (set, 32 chars)
    production: (set, 28 chars)

  DB_PASSWORD:
    staging:    (set, 12 chars)
    production: (set, 24 chars)

  2 difference(s)
```

### `envctl edit`

Open the encrypted secrets file in your editor:

```bash
envctl edit --env production
```

### `envctl init`

Bootstrap envctl in an existing project. **Always use this command** rather than creating configuration files manually -- it detects your template, classifies variables, creates the config and directory structure, and updates `.gitignore`:

```bash
envctl init
```

What it does:
1. Checks that `sops` and `age` are installed
2. Detects your template file (`.env.example`, `.env.sample`, `.env.template`, `.env.dist`)
3. Parses variables and reports stats (required, optional, static)
4. Creates `.envctl.yaml` with `local` and `production` environments pre-configured
5. Creates `.envctl/` directory for encrypted secret files
6. Appends envctl entries to `.gitignore`

## CI/CD usage

The Docker image (`ghcr.io/pillowpillow/envctl`) bundles envctl, sops, and age -- no prerequisites to install.

### GitHub Actions -- reusable workflow (recommended)

The simplest way. Add to your project's workflow:

```yaml
jobs:
  envctl:
    uses: PillowPillow/envctl/.github/workflows/envctl-verify.yaml@main
    with:
      environment: production
    secrets:
      sops-age-key: ${{ secrets.SOPS_AGE_KEY }}
```

To also generate the `.env` file as a downloadable artifact:

```yaml
jobs:
  envctl:
    uses: PillowPillow/envctl/.github/workflows/envctl-verify.yaml@main
    with:
      environment: production
      generate: true
      output-path: .env
    secrets:
      sops-age-key: ${{ secrets.SOPS_AGE_KEY }}
```

### GitHub Actions -- Docker step

If you need envctl as a step inside an existing job:

```yaml
- name: Verify environment
  run: |
    docker run --rm \
      -v ${{ github.workspace }}:/workspace \
      -e SOPS_AGE_KEY=${{ secrets.SOPS_AGE_KEY }} \
      ghcr.io/pillowpillow/envctl:latest \
      verify --env production --strict

- name: Generate .env
  run: |
    docker run --rm \
      -v ${{ github.workspace }}:/workspace \
      -e SOPS_AGE_KEY=${{ secrets.SOPS_AGE_KEY }} \
      ghcr.io/pillowpillow/envctl:latest \
      generate --env production -o .env
```

### Without Docker

If you can't use Docker, install the binary directly. This requires sops and age to be installed separately:

```yaml
- name: Install envctl
  run: curl -fsSL https://raw.githubusercontent.com/PillowPillow/envctl/main/scripts/install.sh | INSTALL_DIR=/usr/local/bin bash

- name: Install sops & age
  run: |
    # sops
    curl -sLO https://github.com/getsops/sops/releases/download/v3.9.4/sops-v3.9.4.linux.amd64
    sudo mv sops-v3.9.4.linux.amd64 /usr/local/bin/sops && sudo chmod +x /usr/local/bin/sops
    # age
    curl -sL https://github.com/FiloSottile/age/releases/download/v1.2.1/age-v1.2.1-linux-amd64.tar.gz | tar -xz
    sudo mv age/age age/age-keygen /usr/local/bin/

- name: Verify & generate
  env:
    SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}
  run: |
    envctl verify --env production --strict
    envctl generate --env production --output .env
```

## Project structure

```
your-project/
  .env.example                          # Template (committed)
  .envctl.yaml                          # Configuration (committed)
  .envctl/
    secrets.local.enc.yaml              # Encrypted local secrets (committed)
    secrets.production.enc.yaml         # Encrypted production secrets (committed)
```

Encrypted files are safe to commit -- keys are readable for review, values are encrypted with age.

## Adding a new team member

1. They generate an age key pair: `age-keygen -o ~/.config/sops/age/keys.txt`
2. They share their public key (`age1...`)
3. You add it to `.envctl.yaml` under `sops.age_recipients`
4. Re-encrypt all secrets files with the updated recipients:
   ```bash
   sops updatekeys .envctl/secrets.local.enc.yaml
   sops updatekeys .envctl/secrets.production.enc.yaml
   ```
5. Commit and push

## Development

```bash
# Build
make build

# Test
make test

# Lint (requires golangci-lint)
make lint

# All checks
make check

# Install git pre-commit hook
make hooks
```

## License

MIT
