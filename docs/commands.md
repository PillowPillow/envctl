# Commands

Every command accepts `--config <path>` (default: the nearest `.envctl.yaml`, searched from the current directory upwards) and `--env <name>` (the target environment).

## `envctl init`

Bootstrap envctl in an existing project. Use this command rather than writing the configuration by hand: it detects your template, classifies variables, creates the config and directory structure, and updates `.gitignore`.

```bash
envctl init
```

What it does:

1. Checks that `sops` and `age` are installed
2. Detects your template file (`.env.example`, `.env.sample`, `.env.template`, `.env.dist`)
3. Parses variables and reports stats (required, optional, static)
4. Creates `.envctl.yaml` with `local` and `production` environments pre-configured
5. Creates the `.envctl/` directory for encrypted secret files
6. Appends envctl entries to `.gitignore` (`.env`, `.env.local`, decrypted `secrets.*.yaml`)

## `envctl verify`

Check that every required variable of the template has a value for an environment:

```bash
envctl verify --env production
envctl verify --env production --strict  # treat warnings as errors (auto-enabled when CI=true)
```

A warning is a value in the secrets file that the template does not use. With `--strict` it fails the check like a missing variable.

## `envctl generate`

Produce a `.env` file from template + secrets + overrides:

```bash
envctl generate --env local --output .env
envctl generate --env production --output env-production --strict
envctl generate --env local              # writes the environment's `output`, or stdout when it has none
envctl generate --env local --dry-run    # print to stdout, write nothing
```

`--strict` refuses to write the file when a required variable has no value, and also when the secrets file holds a value the template does not use (add the template line before the secret, or both in one commit). It is on automatically when `CI=true`. The file is written with mode `0600`.

## `envctl diff`

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

## `envctl edit`

Open the encrypted secrets file of an environment in your editor, through sops:

```bash
envctl edit --env production
```

## `envctl keys`

Manage age encryption recipients. Without `--env` the command targets the global recipients (all environments); with `--env <name>` it targets one environment:

```bash
envctl keys list                              # List all recipients, with their scope
envctl keys list --env production             # List the effective recipients of production
envctl keys add <name> <age1...>              # Add a global recipient + re-encrypt all secrets files
envctl keys add <name> <age1...> --env local  # Add a recipient to local only + re-encrypt its file
envctl keys remove <name>                     # Remove a global recipient + re-encrypt all secrets files
envctl keys remove <name> --env local         # Remove a recipient from local only
```

`keys list` prints `NAME`, `KEY` and `SCOPE` columns (`global` or the environment name). Every `add` and `remove` re-keys and rotates the affected secrets files. See [Keys and re-encryption](keys.md).

## `envctl update`

Replace the binary with the latest release. It applies to installs made with the install script; the other install methods have their own upgrade path. For Homebrew and `go install`, `envctl update` prints the command to run instead.

| Installed with | Upgrade with |
|---|---|
| install script | `envctl update` |
| Homebrew | `brew upgrade envctl` |
| `go install` | `go install github.com/PillowPillow/envctl/v2@latest` |
| Docker | pull a newer image tag (do not run `envctl update` in a container) |

When it was installed with the install script, envctl also checks for a new release in the background once in a while and prints a one-line notice after the command output.

Set `ENVCTL_NO_UPDATE_CHECK=1` to turn that check off, for example in a sandbox or a CI image where the binary is pinned. `envctl update` still works when you run it.

## `envctl version`

Print the installed version.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Any failure: missing required secrets, invalid configuration, sops or age error |

On failure envctl prints the reason on stderr, prefixed with `Error:`.
