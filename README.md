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

This adds the key to `.envctl.yaml` and re-encrypts any existing secrets files. To give the key access to one environment only, add `--env <name>`.

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
    age_recipients:       # optional: added to the global recipients, this env only
      dev: age1xyz789...
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

### `environments.<env>.age_recipients`

Optional named map of age public keys that can decrypt **this environment's** secrets file only. It is additive: the recipients of an environment are the global `sops.age_recipients` plus the ones listed here.

```yaml
environments:
  local:
    secrets: .envctl/secrets.local.enc.yaml
    age_recipients:
      dev: age1xyz789...
```

A recipient name uses letters, digits, `.`, `_` or `-` and starts with a letter or a digit. The same name may appear in several environments (for example a `dev` key per environment), but never both globally and in an environment: `envctl keys add` refuses a global name that already exists in any scope, and an `--env <name>` add refuses a name that is global or already in that environment. A name that is both global and in an environment (for example after a manual edit) is rejected when the config loads, so every command fails until the duplicate is removed. A config without per-environment `age_recipients` behaves as before. The environment name `global` is reserved for the global recipients (it is the `SCOPE` shown by `keys list`) and is rejected when the config loads.

When several environments point to the same `secrets` file, that file is encrypted for the union of their recipients: a key added to one of them with `--env` can decrypt the shared file, and so the other environments' secrets too.

### Path resolution

`secrets` paths are resolved from the folder that contains `.envctl.yaml`, not from the current directory, so every command behaves the same wherever you run it from inside the project. `template` and `output` paths stay relative to the current directory.

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

Manage age encryption recipients. Without `--env` the command targets the global recipients (all environments); with `--env <name>` it targets one environment:

```bash
envctl keys list                              # List all recipients, with their scope
envctl keys list --env production             # List the effective recipients of production
envctl keys add <name> <age1...>              # Add a global recipient + re-encrypt all secrets files
envctl keys add <name> <age1...> --env local  # Add a recipient to local only + re-encrypt its file
envctl keys remove <name>                     # Remove a global recipient + re-encrypt all secrets files
envctl keys remove <name> --env local         # Remove a recipient from local only
```

`keys list` prints `NAME`, `KEY` and `SCOPE` columns (`global` or the environment name). Every `add` and `remove` re-keys and rotates the affected secrets files.

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

### Scope a recipient to one environment

```bash
envctl keys add dev age1xyz789... --env local
envctl keys list --env local
```

`dev` can now decrypt `local` only; the global recipients still can. A global recipient cannot be removed from a single environment: `envctl keys remove <name> --env <env>` fails and tells you to remove it without `--env`.

### Add a CI key

```bash
age-keygen  # copy private key → GitHub repo secret SOPS_AGE_KEY
envctl keys add ci age1abc123...
```

### List / remove recipients

```bash
envctl keys list
envctl keys remove alice
envctl keys remove dev --env local
```

envctl refuses a removal that would leave an environment whose secrets file exists with no recipients (`cannot remove "<name>": environment "<env>" would have no recipients left`) and changes nothing. While an environment has no secrets file yet, you can remove its last recipient, for example to fix a wrong key right after `envctl init`; `envctl edit` then refuses to create the file until a recipient is added.

### Re-encryption and git history

- Every `keys add` and `keys remove` rotates the data key of each affected file, so **the whole file changes in git**, not only the recipient block. Expect a full diff on review.
- Rotation does not rewrite git history. A removed key can still decrypt older versions of the files from history, and `keys remove` prints a warning saying so. To fully revoke access, also change the secret values themselves.
- Before changing anything, `keys add|remove` checks that your age key can decrypt every targeted secrets file. If one cannot be decrypted, the command lists it and changes nothing: run it as a recipient of every targeted file (for a global change, someone who holds every environment's key).
- If re-keying still fails part-way, `.envctl.yaml` is left untouched and the error lists the files already re-keyed. Fix the cause and re-run the same command: a file you can no longer decrypt (because you removed your own key) and that already has the expected recipients is reported as already up to date and skipped. Every file you can decrypt is rotated again.

## Upgrading from v1

v2 is a major release because every `keys add|remove` now re-keys and rotates the secrets files, and `secrets` paths are resolved from the `.envctl.yaml` folder instead of the current directory. There is no config migration: an existing `.envctl.yaml` keeps working and keeps the same recipients, with one caveat.

Recipient names are now validated when the config loads: letters, digits, `.`, `_` or `-`, starting with a letter or a digit. A v1 config with any other name (for example `alice smith` or `@ci`) fails to load, so every command that reads it (`verify`, `generate`, `diff`, `edit`, `keys`) fails until you rename it in `.envctl.yaml` by hand. The same applies to an environment named `global`, which is now reserved.

The GitHub Action pulls the `latest` image by default (`envctl-version: latest`), so consumer workflows move to v2 as soon as it is released. Pin `envctl-version` to a v1 tag if you need time to check your config.

The Go module path changed, so install with the `/v2` path:

```bash
go install github.com/PillowPillow/envctl/v2@latest
```

Install script, Docker image and `envctl update` are unchanged.

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
