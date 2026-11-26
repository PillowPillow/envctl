#!/usr/bin/env bash
# Captures the real output of a failing `envctl verify` for the README terminal card.
# Usage: docs/brand/capture-verify.sh <envctl-binary> <out-file>
set -euo pipefail

bin=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
out=$2
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

age-keygen -o "$work/key.txt" 2>/dev/null
pub=$(age-keygen -y "$work/key.txt")

mkdir -p "$work/app/.envctl"
cd "$work/app"

cat > .env.example <<'TPL'
PORT=3000
DATABASE_URL=${DATABASE_URL}
STRIPE_SECRET_KEY=${STRIPE_SECRET_KEY}
STRIPE_WEBHOOK_SECRET=${STRIPE_WEBHOOK_SECRET}
SENTRY_DSN=${SENTRY_DSN}
LOG_LEVEL=${LOG_LEVEL:-info}
TPL

cat > .envctl.yaml <<YAML
version: "1"
template: .env.example
environments:
  production:
    secrets: .envctl/production.enc.yaml
sops:
  age_recipients:
    capture: $pub
YAML

cat > plain.yaml <<'SECRETS'
DATABASE_URL: postgres://app:example@db:5432/app
STRIPE_SECRET_KEY: sk_live_example
SENTRY_DSN: https://example@sentry.invalid/1
SECRETS
sops --encrypt --age "$pub" --input-type yaml --output-type yaml plain.yaml > .envctl/production.enc.yaml
rm plain.yaml

status=0
transcript=$(env -u CI NO_COLOR=1 SOPS_AGE_KEY_FILE="$work/key.txt" "$bin" verify --env production 2>&1) || status=$?
if [ "$status" -ne 1 ]; then
  echo "expected exit 1 from envctl verify, got $status" >&2
  echo "$transcript" >&2
  exit 1
fi

cd - >/dev/null
printf '$ envctl verify --env production\n%s\n' "$transcript" > "$out"
