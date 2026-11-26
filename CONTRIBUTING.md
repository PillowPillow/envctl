# Contributing to envctl

Thanks for helping. envctl is a small CLI with one job: keep a project's environment variables complete and encrypted in its own repository. Contributions that keep it small are the most welcome: bug fixes, documentation and tests in particular.

## Before you start

- For anything larger than a bug fix, open an issue first so we can agree on the approach.
- envctl stays a thin layer over [sops](https://getsops.io) and [age](https://github.com/FiloSottile/age). Please discuss any new dependency in an issue first.

## Set up

You need Go (the version in `go.mod`), `sops`, `age` and [golangci-lint](https://golangci-lint.run) v2.

```sh
git clone https://github.com/PillowPillow/envctl.git
cd envctl
make check
```

`make check` verifies the modules, runs the linter and runs the tests with the race detector. `make hooks` installs a pre-commit hook that runs the module check and the linter.

## Pull requests

- Keep each pull request focused on one change.
- Add or update tests for any behaviour change. `make test` runs them with `-race`; tests that need sops and age skip when the tools are missing.
- Run `make lint` and `make test`. CI runs both on every pull request.
- Comments explain why, not what: write one only for a constraint the code cannot show, on one line.
- The README visuals in `docs/brand/` are generated. Change the generator, then run `make brand` and commit the SVG files with it. Never edit an SVG by hand.

By contributing, you agree that your contribution is licensed under the [GNU GPL v3.0](LICENSE).
