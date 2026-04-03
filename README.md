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

### From GitHub Releases

Download the latest binary from [Releases](https://github.com/PillowPillow/envctl/releases):

```bash
# macOS (Apple Silicon)
curl -sL https://github.com/PillowPillow/envctl/releases/latest/download/envctl_darwin_arm64.tar.gz | tar -xz
sudo mv envctl /usr/local/bin/

# macOS (Intel)
curl -sL https://github.com/PillowPillow/envctl/releases/latest/download/envctl_darwin_amd64.tar.gz | tar -xz
sudo mv envctl /usr/local/bin/

# Linux (amd64)
curl -sL https://github.com/PillowPillow/envctl/releases/latest/download/envctl_linux_amd64.tar.gz | tar -xz
sudo mv envctl /usr/local/bin/
```

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

Bootstrap envctl in an existing project:

```bash
envctl init
```

## CI/CD usage

### GitHub Actions (reusable workflow)

envctl provides a reusable workflow for verification in CI:

```yaml
jobs:
  envctl:
    uses: PillowPillow/envctl/.github/workflows/envctl-verify.yaml@main
    with:
      environment: production
    secrets:
      sops-age-key: ${{ secrets.SOPS_AGE_KEY }}
```

### Direct usage

```yaml
- name: Install envctl
  run: |
    curl -sL https://github.com/PillowPillow/envctl/releases/latest/download/envctl_linux_amd64.tar.gz | tar -xz
    sudo mv envctl /usr/local/bin/

- name: Verify & generate
  env:
    SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}
  run: |
    envctl verify --env production --strict
    envctl generate --env production --output .env
```

### Docker

```yaml
- name: Verify environment
  run: |
    docker run --rm \
      -v ${{ github.workspace }}:/workspace \
      -e SOPS_AGE_KEY=${{ secrets.SOPS_AGE_KEY }} \
      ghcr.io/pillowpillow/envctl:latest \
      verify --env production --strict
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
