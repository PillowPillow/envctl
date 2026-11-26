# Security policy

## Supported versions

Only the latest release receives security fixes.

## Reporting a vulnerability

Please do not open a public issue. Report it privately through [GitHub's private vulnerability reporting](https://github.com/PillowPillow/envctl/security/advisories/new) instead, with the steps to reproduce and the impact you see.

You can expect a first answer within a week. Once a fix is released, the advisory is published with credit to you unless you prefer otherwise.

## Scope

- The envctl code in this repository.
- `install.sh`, the composite GitHub Action (`action.yaml`), the reusable workflow and the release workflow that builds the published archives and the Docker image.

Out of scope: vulnerabilities in [sops](https://github.com/getsops/sops) or [age](https://github.com/FiloSottile/age) themselves. Please report those to their projects.

To check that a release archive was built by this repository's release workflow, run this with the GitHub CLI signed in:

```sh
gh attestation verify envctl_<os>_<arch>.tar.gz --repo PillowPillow/envctl --signer-workflow PillowPillow/envctl/.github/workflows/release.yaml
```
