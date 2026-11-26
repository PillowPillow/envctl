# Keys and re-encryption

Each person and each CI runner has its own [age](https://github.com/FiloSottile/age) key pair. The public keys are listed in `.envctl.yaml`; every secrets file is encrypted for the keys allowed to open it. A global key opens every environment; a key added with `--env` opens only that environment's file.

## Add a team member

```bash
# They generate a key pair and tell sops where it is
mkdir -p ~/.config/sops/age && age-keygen -o ~/.config/sops/age/keys.txt
export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt

# They share their public key, you add it
envctl keys add alice age1ql3z7hjy54pw3hyww5ayf...
```

This adds the key to `.envctl.yaml` and re-encrypts all secrets files automatically.

Without `SOPS_AGE_KEY_FILE`, sops reads `$XDG_CONFIG_HOME/sops/age/keys.txt`, which defaults to `~/.config/sops/age/keys.txt` on Linux and `~/Library/Application Support/sops/age/keys.txt` on macOS.

## Scope a recipient to one environment

```bash
envctl keys add dev age1xyz789... --env local
envctl keys list --env local
```

`dev` can now decrypt `local` only; the global recipients still can. A global recipient cannot be removed from a single environment: `envctl keys remove <name> --env <env>` fails and tells you to remove it without `--env`.

Environments that point to the same `secrets` file share their recipients: a key added to one of them can decrypt the shared file.

## Add a CI key

```bash
age-keygen  # copy the private key into the repository secret SOPS_AGE_KEY
envctl keys add ci age1abc123...
```

Add it with `--env staging` and `--env production` instead to keep CI out of `local`.

## List / remove recipients

```bash
envctl keys list
envctl keys remove alice
envctl keys remove dev --env local
```

envctl refuses a removal that would leave an environment whose secrets file exists with no recipients (`cannot remove "<name>": environment "<env>" would have no recipients left`) and changes nothing. While an environment has no secrets file yet, you can remove its last recipient, for example to fix a wrong key right after `envctl init`; `envctl edit` then refuses to create the file until a recipient is added.

## Re-encryption and git history

- Every `keys add` and `keys remove` rotates the data key of each affected file, so **the whole file changes in git**, not only the recipient block. Expect a full diff on review.
- Rotation does not rewrite git history. A removed key can still decrypt older versions of the files from history, and `keys remove` prints a warning saying so. To fully revoke access, also change the secret values themselves.
- Before changing anything, `keys add|remove` checks that your age key can decrypt every targeted secrets file. If one cannot be decrypted, the command lists it and changes nothing: run it as a recipient of every targeted file (for a global change, someone who holds every environment's key).
- `keys add|remove` only manages files whose sops metadata lists age recipients alone. A file that also lists another key source (`hc_vault`, `kms`, `gcp_kms`, `azure_kv`, `pgp`, `key_groups`…) is refused before anything changes, because rotating it would hand the new data key to that source. Remove the extra source with sops, then re-run.
- If re-keying still fails part-way, `.envctl.yaml` is left untouched and the error lists the files already re-keyed. Fix the cause and re-run the same command: a file you can no longer decrypt (because you removed your own key) and that already has the expected recipients is reported as already up to date and skipped, with a warning that its data key was not rotated; a remaining recipient should re-run the command to rotate it. Every file you can decrypt is rotated again.
