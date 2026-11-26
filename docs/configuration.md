# Configuration

`envctl init` writes `.envctl.yaml` for you. This page is the reference for changing it afterwards.

## `.envctl.yaml`

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

Keys of an environment:

| Key | Meaning |
|---|---|
| `secrets` | Encrypted sops file holding this environment's values |
| `output` | Default file `envctl generate` writes when `--output` is not given (stdout when both are empty) |
| `overrides` | Values that win over the secrets file and the template defaults |
| `strip` | Variables removed from the generated output |
| `age_recipients` | Extra keys that can decrypt this environment only (see below) |

## Templates

The template is usually `.env.example`, but any text file works: files whose name contains `.env` (`.env.example`, `app.env.yaml`) are read as dotenv, every other file is treated as plain text where each `${VAR}` or `${VAR:-default}` placeholder is substituted.

| Line in the template | Kind | Behaviour |
|---|---|---|
| `VAR=value` | static | Kept as written |
| `VAR=${VAR}` | required | Must have a value in the secrets file or `overrides`; `verify` fails without one |
| `VAR=${VAR:-default}` | optional | Falls back to `default` |

## `sops.age_recipients`

Named map of age public keys. Managed via `envctl keys`:

```yaml
sops:
  age_recipients:
    alice: age1ql3z7hjy54pw3hyww5ayf...
    ci: age1abc123...
```

## `environments.<env>.age_recipients`

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

## Path resolution

`secrets` paths are resolved from the folder that contains `.envctl.yaml`, not from the current directory, so every command behaves the same wherever you run it from inside the project. `template` and `output` paths stay relative to the current directory.

## Substitution priority

When generating a `.env` file, values are resolved in this order (highest wins):

1. `overrides` from `.envctl.yaml`
2. Secret store value (SOPS file)
3. Default from `.env.example` (`${VAR:-default}`)
4. Empty string (an error for a required variable)

## Project structure

```
your-project/
  .env.example                          # Template (committed)
  .envctl.yaml                          # Configuration (committed)
  .envctl/
    secrets.local.enc.yaml              # Encrypted local secrets (committed)
    secrets.production.enc.yaml         # Encrypted production secrets (committed)
```

Encrypted files are safe to commit: key names stay readable for review, values are encrypted with age.
