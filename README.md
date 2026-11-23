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

### Install script

Requires [GitHub CLI](https://cli.github.com/) (`gh`) to be installed and authenticated (`gh auth login`).

```bash
gh api repos/PillowPillow/envctl/contents/scripts/install.sh --jq '.content' | base64 -d | GITHUB_TOKEN=$(gh auth token) bash
```

Installs to `~/.envctl/bin/envctl`. Optional environment variables:
- `ENVCTL_VERSION` -- pin a specific version (default: latest)
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
go install github.com/PillowPillow/envctl/v2@latest
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

### 2. Generate an age key pair and add it as a recipient

```bash
age-keygen -o ~/.config/sops/age/keys.txt
```

Copy the public key (starts with `age1...`) and register it:

```bash
envctl keys add your-name age1ql3z7hjy54pw3hyww5ayf...
```

This adds the key to `.envctl.yaml` and re-encrypts any existing secrets files.

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

# SOPS encryption recipients (managed via envctl keys)
sops:
  age_recipients:
    alice: age1ql3z7hjy54pw3hyww5ayf...
    ci: age1abc123...
```

### `sops.age_recipients`

Named map of age public keys. Managed via `envctl keys`:

```yaml
sops:
  age_recipients:
    alice: age1ql3z7hjy54pw3hyww5ayf...
    ci: age1abc123...
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

### `envctl keys`

Manage age encryption recipients:

```bash
envctl keys list                    # List configured recipients
envctl keys add <name> <age1...>    # Add recipient + re-encrypt secrets
envctl keys remove <name>           # Remove recipient + re-encrypt secrets
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

### GitHub Actions -- composite action (recommended)

Use the action as steps in your build job. The `.env` file stays in the workspace -- no artifacts, no secrets leaking:

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v7

      - uses: PillowPillow/envctl@main
        with:
          command: verify
          environment: production
        env:
          SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}

      - uses: PillowPillow/envctl@main
        with:
          command: generate
          environment: production
          output-path: .env.production
        env:
          SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}

      - run: npm run build  # .env.production is in the workspace
```

Action inputs:

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `command` | yes | | `verify` or `generate` |
| `environment` | yes | | Target environment name |
| `envctl-version` | no | `latest` | Docker image tag |
| `output-path` | no | `.env` | Output path for generate |
| `strict` | no | `true` | Treat warnings as errors (verify only) |

### GitHub Actions -- reusable workflow (verify only)

For a standalone verification check (e.g. as a required status check on PRs):

```yaml
jobs:
  envctl:
    uses: PillowPillow/envctl/.github/workflows/envctl-verify.yaml@main
    with:
      environment: production
      github-environment: production
    secrets: inherit
```

The `github-environment` input maps to the GitHub Actions [environment](https://docs.github.com/en/actions/deployment/targeting-different-environments/using-environments-for-deployment) where `SOPS_AGE_KEY` is stored.

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

## Managing recipients

### Add a team member

```bash
# They generate a key pair
age-keygen -o ~/.config/sops/age/keys.txt

# They share their public key, you add it
envctl keys add alice age1ql3z7hjy54pw3hyww5ayf...
```

This adds the key to `.envctl.yaml` and re-encrypts all secrets files automatically.

### Add a CI key

```bash
age-keygen  # copy private key → GitHub repo secret SOPS_AGE_KEY
envctl keys add ci age1abc123...
```

### List / remove recipients

```bash
envctl keys list
envctl keys remove alice
```

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
