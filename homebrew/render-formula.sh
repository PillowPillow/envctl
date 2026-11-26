#!/usr/bin/env bash
# Usage: homebrew/render-formula.sh <version-without-v> <checksums.txt> > envctl.rb
set -euo pipefail

[[ $# -eq 2 ]] || { echo "usage: $0 <version> <checksums.txt>" >&2; exit 2; }
version="$1"
checksums="$2"
template="$(dirname "$0")/envctl.rb"

[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "version must be X.Y.Z, got: $version" >&2; exit 1; }

sha_for() {
    local sha
    sha="$(awk -v name="envctl_$1.tar.gz" '$2 == name {print $1}' "$checksums")"
    [[ "$sha" =~ ^[0-9a-f]{64}$ ]] || { echo "no sha256 for envctl_$1.tar.gz in $checksums" >&2; exit 1; }
    echo "$sha"
}

darwin_arm64="$(sha_for darwin_arm64)"
darwin_amd64="$(sha_for darwin_amd64)"
linux_arm64="$(sha_for linux_arm64)"
linux_amd64="$(sha_for linux_amd64)"

formula="$(sed \
    -e "s/@VERSION@/${version}/g" \
    -e "s/@SHA256_DARWIN_ARM64@/${darwin_arm64}/g" \
    -e "s/@SHA256_DARWIN_AMD64@/${darwin_amd64}/g" \
    -e "s/@SHA256_LINUX_ARM64@/${linux_arm64}/g" \
    -e "s/@SHA256_LINUX_AMD64@/${linux_amd64}/g" \
    "$template")"

if grep -q '@[A-Z0-9_]*@' <<<"$formula"; then
    echo "unreplaced placeholder left in the formula" >&2
    exit 1
fi
printf '%s\n' "$formula"
