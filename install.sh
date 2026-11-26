#!/usr/bin/env bash
# Installs the latest envctl release: curl -fsSL https://raw.githubusercontent.com/PillowPillow/envctl/main/install.sh | bash
#
# Environment variables:
#   ENVCTL_VERSION  release tag to install, e.g. v2.1.0 (default: latest)
#   INSTALL_DIR     installation directory (default: ~/.envctl/bin)
set -euo pipefail

REPO="PillowPillow/envctl"

info()  { printf '\033[1;34m==> %s\033[0m\n' "$*"; }
ok()    { printf '\033[1;32m==> %s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m==> %s\033[0m\n' "$*"; }
fail()  { printf '\033[1;31m==> ERROR: %s\033[0m\n' "$*" >&2; exit 1; }

detect_os() {
    case "$(uname -s)" in
        Darwin) echo "darwin" ;;
        Linux)  echo "linux" ;;
        *)      fail "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             fail "Unsupported architecture: $(uname -m)" ;;
    esac
}

download() {
    curl -fL --proto '=https' --proto-redir '=https' --progress-bar "$1" -o "$2"
}

sha256_of() {
    if command -v sha256sum >/dev/null; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

# Wrapped in a function so a truncated download never runs a partial script.
main() {
    local install_dir="${INSTALL_DIR:-${HOME}/.envctl/bin}"
    local version="${ENVCTL_VERSION:-}"
    local os arch archive base_url
    os="$(detect_os)"
    arch="$(detect_arch)"
    archive="envctl_${os}_${arch}.tar.gz"

    if [[ -n "$version" ]]; then
        [[ "$version" == v* ]] || version="v${version}"
        base_url="https://github.com/${REPO}/releases/download/${version}"
    else
        base_url="https://github.com/${REPO}/releases/latest/download"
    fi
    info "Installing envctl ${version:-latest} for ${os}/${arch}"

    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT

    download "${base_url}/${archive}" "${work}/${archive}"
    download "${base_url}/checksums.txt" "${work}/checksums.txt"

    local expected actual
    expected="$(awk -v name="$archive" '$2 == name {print $1}' "${work}/checksums.txt")"
    [[ -n "$expected" ]] || fail "No checksum for ${archive} in checksums.txt. Nothing was installed."
    actual="$(sha256_of "${work}/${archive}")"
    [[ "$expected" == "$actual" ]] || fail "Checksum mismatch for ${archive} (expected ${expected}, got ${actual}). Nothing was installed."
    ok "Checksum verified."

    # The checksum proves integrity only; the attestation proves the archive came from the release workflow.
    if command -v gh >/dev/null && gh auth status >/dev/null 2>&1 && gh attestation --help >/dev/null 2>&1; then
        local report
        report="$(gh attestation verify "${work}/${archive}" --repo "$REPO" \
            --signer-workflow "${REPO}/.github/workflows/release.yaml" 2>&1)" || {
            echo "$report" >&2
            fail "Could not verify that the download is an envctl release build (see the error above). Nothing was installed."
        }
        ok "Provenance verified."
    else
        warn "Provenance not checked: install the GitHub CLI and sign in to verify the release build."
    fi

    tar -xzf "${work}/${archive}" -C "$work" envctl
    mkdir -p "$install_dir"
    install -m 0755 "${work}/envctl" "${install_dir}/envctl"
    ok "envctl installed to ${install_dir}/envctl"

    case ":${PATH}:" in
        *":${install_dir}:"*) ;;
        *)
            warn "${install_dir} is not in your PATH."
            echo "  Add this to your shell profile:"
            echo "    export PATH=\"${install_dir}:\${PATH}\""
            ;;
    esac

    "${install_dir}/envctl" version 2>/dev/null || true

    local shell_name
    shell_name="$(basename "${SHELL:-/bin/bash}")"
    case "$shell_name" in
        zsh|bash|fish) ;;
        *) shell_name="bash" ;;
    esac
    info "To enable shell completions, add this to your shell profile:"
    echo "  eval \"\$(envctl completion ${shell_name})\""
}

main "$@"
