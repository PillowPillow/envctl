# Use envctl in CI

The Docker image `ghcr.io/pillowpillow/envctl` bundles envctl, sops and age, so a runner needs nothing installed. CI only needs one age private key, stored as the `SOPS_AGE_KEY` secret (see [Add a CI key](keys.md#add-a-ci-key)).

## GitHub Actions: composite action (recommended)

Use the action as steps in your build job. The `.env` file stays in the workspace: no artifact, no secret leaving the job.

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v7

      - uses: PillowPillow/envctl@v2.1.0
        with:
          command: verify
          environment: production
          envctl-version: 2.1.0
        env:
          SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}

      - uses: PillowPillow/envctl@v2.1.0
        with:
          command: generate
          environment: production
          envctl-version: 2.1.0
          output-path: .env.production
        env:
          SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}

      - run: npm run build  # .env.production is in the workspace
```

Pin both the action (`@v2.1.0`) and `envctl-version`: the action runs the Docker image tagged `envctl-version`, which defaults to `latest`, so the action ref alone does not pin the binary.

Action inputs:

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `command` | yes | | `verify` or `generate` |
| `environment` | yes | | Target environment name |
| `envctl-version` | no | `latest` | Docker image tag, without the `v` (e.g. `2.1.0`) |
| `output-path` | no | `.env` | Output path for generate |
| `strict` | no | `true` | Treat warnings as errors (verify only) |

`verify` exits with code 1 when a required variable has no value, which fails the step and stops the job.

## GitHub Actions: reusable workflow (verify only)

For a standalone check, for example a required status check on pull requests:

```yaml
jobs:
  envctl:
    uses: PillowPillow/envctl/.github/workflows/envctl-verify.yaml@v2.1.0
    with:
      environment: production
      github-environment: production
      envctl-version: 2.1.0
    secrets: inherit
```

Workflow inputs:

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `environment` | yes | | Environment to verify |
| `github-environment` | yes | | GitHub environment that holds `SOPS_AGE_KEY` |
| `envctl-version` | no | `latest` | Docker image tag, without the `v` |

The workflow calls the composite action from `main`, so its steps follow the latest action; `envctl-version` pins the Docker image, and with it the envctl binary that runs.

The `github-environment` input maps to the GitHub Actions [environment](https://docs.github.com/en/actions/deployment/targeting-different-environments/using-environments-for-deployment) where `SOPS_AGE_KEY` is stored.

## Docker, anywhere else

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace -e SOPS_AGE_KEY \
  ghcr.io/pillowpillow/envctl:2.1.0 verify --env production
```
