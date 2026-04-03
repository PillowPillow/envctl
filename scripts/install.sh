#!/usr/bin/env bash
#
# Installer for envctl CLI.
# Usage: curl -fsSL https://raw.githubusercontent.com/PillowPillow/envctl/main/scripts/install.sh | bash
#
# Environment variables:
#   ENVCTL_VERSION  - Version to install (default: latest)
#   GITHUB_TOKEN    - GitHub personal access token (for private repos)
#   INSTALL_DIR     - Installation directory (default: ~/.envctl/bin)

set -euo pipefail

GITHUB_OWNER="PillowPillow"
GITHUB_REPO="envctl"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.envctl/bin}"
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

api_headers() {
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        echo "-H" "Authorization: Bearer ${GITHUB_TOKEN}"
    fi
}

fetch_latest_version() {
    local url="https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest"
    local version

    version=$(curl -fsSL $(api_headers) -H "Accept: application/vnd.github+json" "${url}" \
        | grep -o '"tag_name":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ -z "${version}" ]; then
        error "Could not determine latest version. Set ENVCTL_VERSION or check your GITHUB_TOKEN."
    fi
    echo "${version}"
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

    # Archive name does not include version (goreleaser name_template).
    archive_name="envctl_${os}_${arch}.tar.gz"

    local base_url="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/${version}"

    TMPDIR_CLEANUP=$(mktemp -d)
    trap 'rm -rf "${TMPDIR_CLEANUP}"' EXIT TERM INT
    local tmpdir="${TMPDIR_CLEANUP}"

    # Download archive.
    info "Downloading ${archive_name}..."
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        # For private repos, use the GitHub API to download release assets.
        local asset_url
        asset_url=$(curl -fsSL $(api_headers) -H "Accept: application/vnd.github+json" \
            "https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/tags/${version}" \
            | grep -o "\"url\":\"[^\"]*\",\"id\":[0-9]*,\"[^\"]*\",\"[^\"]*\",\"[^\"]*\",\"content_type\":\"application/gzip\"" \
            | grep -o '"url":"[^"]*"' | head -1 | cut -d'"' -f4 || true)

        if [ -n "${asset_url}" ]; then
            curl -fsSL $(api_headers) -H "Accept: application/octet-stream" \
                -o "${tmpdir}/${archive_name}" "${asset_url}"
        else
            # Fallback to direct download URL.
            curl -fsSL $(api_headers) -o "${tmpdir}/${archive_name}" "${base_url}/${archive_name}"
        fi
    else
        curl -fsSL -o "${tmpdir}/${archive_name}" "${base_url}/${archive_name}"
    fi

    # Download checksums.
    info "Verifying checksum..."
    local checksum_file="checksums.txt"
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        local checksums_asset_url
        checksums_asset_url=$(curl -fsSL $(api_headers) -H "Accept: application/vnd.github+json" \
            "https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/tags/${version}" \
            | grep -o "\"name\":\"checksums.txt\",\"[^}]*\"url\":\"[^\"]*\"" \
            | grep -o '"url":"[^"]*"' | tail -1 | cut -d'"' -f4 || true)

        if [ -n "${checksums_asset_url}" ]; then
            curl -fsSL $(api_headers) -H "Accept: application/octet-stream" \
                -o "${tmpdir}/${checksum_file}" "${checksums_asset_url}"
        else
            curl -fsSL $(api_headers) -o "${tmpdir}/${checksum_file}" "${base_url}/${checksum_file}"
        fi
    else
        curl -fsSL -o "${tmpdir}/${checksum_file}" "${base_url}/${checksum_file}"
    fi

    # Verify SHA256.
    local expected_hash
    expected_hash=$(grep "${archive_name}" "${tmpdir}/${checksum_file}" | awk '{print $1}')
    if [ -z "${expected_hash}" ]; then
        error "Checksum not found for ${archive_name}. Aborting installation."
    else
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
    fi

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
