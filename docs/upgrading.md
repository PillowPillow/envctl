# Upgrading from v1

v2 is a major release because every `keys add|remove` now re-keys and rotates the secrets files, and `secrets` paths are resolved from the `.envctl.yaml` folder instead of the current directory. There is no config migration: an existing `.envctl.yaml` keeps working and keeps the same recipients, with one caveat.

Recipient names are now validated when the config loads: letters, digits, `.`, `_` or `-`, starting with a letter or a digit. A v1 config with any other name (for example `alice smith` or `@ci`) fails to load, so every command that reads it (`verify`, `generate`, `diff`, `edit`, `keys`) fails until you rename it in `.envctl.yaml` by hand. The same applies to an environment named `global`, which is now reserved. Recipient keys are validated as well: each must be a single `age1…` recipient with a valid bech32 checksum, so a mistyped key or a comma-separated list in one value fails to load.

The GitHub Action pulls the `latest` image unless `envctl-version` is set, so an unpinned workflow moves to the newest release as soon as it is published. Pin `envctl-version` (see [CI](ci.md)) to upgrade when you choose.

The Go module path changed, so install with the `/v2` path:

```bash
go install github.com/PillowPillow/envctl/v2@latest
```
