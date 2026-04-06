#!/usr/bin/env bash
#
# Installer for envctl CLI.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/PillowPillow/envctl/main/scripts/install.sh | bash
#
# For private repos:
#   curl -fsSL ... | GITHUB_TOKEN=ghp_xxx bash
#
# Environment variables:
#   ENVCTL_VERSION  - Version to install (default: latest)
#   GITHUB_TOKEN    - GitHub personal access token (required for private repos)
#   INSTALL_DIR     - Installation directory (default: ~/.envctl/bin)

set -euo pipefail

GITHUB_OWNER="PillowPillow"
GITHUB_REPO="envctl"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.envctl/bin}"
API_BASE="https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}"
TMPDIR_CLEANUP=""

info()  { printf '\033[1;34m==> %s\033[0m\n' "$*"; }
ok()    { printf '\033[1;32m==> %s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m==> %s\033[0m\n' "$*"; }
error() { printf '\033[1;31m==> ERROR: %s\033[0m\n' "$*" >&2; exit 1; }

detect_os() {
    case "$(uname -s)" in
        Darwin) echo "darwin" ;;
        Linux)  echo "linux" ;;
        *)      error "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             error "Unsupported architecture: $(uname -m)" ;;
    esac
}

# gh_curl wraps curl with GitHub auth and Accept headers.
gh_curl() {
    local -a args=(-fsSL -H "Accept: application/vnd.github+json")
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        args+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
    fi
    curl "${args[@]}" "$@"
}

# gh_download downloads a release asset via the GitHub API (works for private repos).
# Usage: gh_download <asset_api_url> <output_path>
gh_download() {
    local -a args=(-fsSL -H "Accept: application/octet-stream" -o "$2")
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        args+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
    fi
    curl "${args[@]}" "$1"
}

fetch_latest_version() {
    local version
    version=$(gh_curl "${API_BASE}/releases/latest" | grep -o '"tag_name":[[:space:]]*"[^"]*"' | cut -d'"' -f4)

    if [ -z "${version}" ]; then
        error "Could not determine latest version. Set ENVCTL_VERSION or check your GITHUB_TOKEN."
    fi
    echo "${version}"
}

# find_asset_url extracts the API URL for a named asset from a release JSON blob.
# Usage: find_asset_url <release_json> <asset_name>
find_asset_url() {
    # Look for the "name" field matching the asset, then extract the "url" from the
    # preceding lines (GitHub API returns "url" before "name" in each asset object).
    echo "$1" | grep -B10 "\"name\": *\"$2\"" | grep '"url"' | head -1 \
        | sed 's/.*"url": *"\([^"]*\)".*/\1/'
}

main() {
    local os arch version archive_name

    os=$(detect_os)
    arch=$(detect_arch)

    info "Detecting platform: ${os}/${arch}"

    if [ -n "${ENVCTL_VERSION:-}" ]; then
        version="${ENVCTL_VERSION}"
        info "Using specified version: ${version}"
    else
        info "Fetching latest version..."
        version=$(fetch_latest_version)
        info "Latest version: ${version}"
    fi

    archive_name="envctl_${os}_${arch}.tar.gz"

    TMPDIR_CLEANUP=$(mktemp -d)
    trap 'rm -rf "${TMPDIR_CLEANUP}"' EXIT TERM INT
    local tmpdir="${TMPDIR_CLEANUP}"

    # Fetch release metadata once.
    info "Downloading ${archive_name}..."
    local release_json
    release_json=$(gh_curl "${API_BASE}/releases/tags/${version}")

    # Find asset URLs.
    local archive_api_url checksums_api_url
    archive_api_url=$(find_asset_url "${release_json}" "${archive_name}")
    checksums_api_url=$(find_asset_url "${release_json}" "checksums.txt")

    if [ -z "${archive_api_url}" ]; then
        error "Asset ${archive_name} not found in release ${version}."
    fi
    if [ -z "${checksums_api_url}" ]; then
        error "checksums.txt not found in release ${version}."
    fi

    # Download archive and checksums via API (works for both public and private repos).
    gh_download "${archive_api_url}" "${tmpdir}/${archive_name}"
    gh_download "${checksums_api_url}" "${tmpdir}/checksums.txt"

    # Verify SHA256.
    info "Verifying checksum..."
    local expected_hash
    expected_hash=$(grep "  ${archive_name}$" "${tmpdir}/checksums.txt" | awk '{print $1}')
    if [ -z "${expected_hash}" ]; then
        error "Checksum not found for ${archive_name}. Aborting installation."
    fi

    local actual_hash
    if command -v sha256sum &>/dev/null; then
        actual_hash=$(sha256sum "${tmpdir}/${archive_name}" | awk '{print $1}')
    else
        actual_hash=$(shasum -a 256 "${tmpdir}/${archive_name}" | awk '{print $1}')
    fi

    if [ "${expected_hash}" != "${actual_hash}" ]; then
        error "Checksum mismatch!\n  Expected: ${expected_hash}\n  Actual:   ${actual_hash}"
    fi
    ok "Checksum verified."

    # Extract and install.
    info "Installing to ${INSTALL_DIR}..."
    mkdir -p "${INSTALL_DIR}"
    tar -xzf "${tmpdir}/${archive_name}" -C "${tmpdir}"
    mv "${tmpdir}/envctl" "${INSTALL_DIR}/envctl"
    chmod +x "${INSTALL_DIR}/envctl"

    ok "envctl ${version} installed to ${INSTALL_DIR}/envctl"

    # Check PATH.
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            warn "${INSTALL_DIR} is not in your PATH."
            echo "  Add this to your shell profile:"
            echo "    export PATH=\"${INSTALL_DIR}:\${PATH}\""
            ;;
    esac

    # Show version.
    "${INSTALL_DIR}/envctl" version 2>/dev/null || true

    # Completion hint.
    local shell_name
    shell_name="$(basename "${SHELL:-/bin/bash}")"
    case "${shell_name}" in
        zsh|bash|fish) ;;
        *) shell_name="bash" ;;
    esac
    info "To enable shell completions, add this to your shell profile:"
    echo "  eval \"\$(envctl completion ${shell_name})\""
}

main "$@"
