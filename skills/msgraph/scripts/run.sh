#!/usr/bin/env bash
# Launcher script for msgraph-skill binary.
# Downloads the correct pre-compiled binary on first run, then executes it.
set -euo pipefail

REPO="merill/msgraph-skill"
BINARY_NAME="msgraph-skill"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${SCRIPT_DIR}/bin"

# Detect OS and architecture
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Darwin) os="darwin" ;;
        Linux)  os="linux" ;;
        *)      echo "Error: Unsupported OS: $(uname -s)" >&2; exit 1 ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        arm64|aarch64)  arch="arm64" ;;
        *)              echo "Error: Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
    esac

    echo "${os}_${arch}"
}

# Get the latest release version from GitHub
get_latest_version() {
    local version
    if command -v curl &>/dev/null; then
        version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    elif command -v wget &>/dev/null; then
        version=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    else
        echo "Error: curl or wget is required" >&2
        exit 1
    fi

    if [ -z "$version" ]; then
        echo "Error: Could not determine latest version" >&2
        exit 1
    fi

    echo "$version"
}

# Download the binary for the given platform and version
download_binary() {
    local platform="$1"
    local version="$2"
    local url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}_${platform}"
    local target="${BIN_DIR}/${BINARY_NAME}"

    mkdir -p "${BIN_DIR}"

    echo "Downloading ${BINARY_NAME} ${version} for ${platform}..." >&2

    if command -v curl &>/dev/null; then
        curl -fsSL -o "${target}" "${url}"
    elif command -v wget &>/dev/null; then
        wget -qO "${target}" "${url}"
    fi

    chmod +x "${target}"
    echo "Downloaded to ${target}" >&2

    # Save version for future checks
    echo "${version}" > "${BIN_DIR}/.version"
}

# Main logic
main() {
    local platform version binary_path

    platform=$(detect_platform)
    binary_path="${BIN_DIR}/${BINARY_NAME}"

    # Download if binary doesn't exist
    if [ ! -x "${binary_path}" ]; then
        version=$(get_latest_version)
        download_binary "${platform}" "${version}"
    fi

    # Execute the binary with all arguments passed through
    exec "${binary_path}" "$@"
}

main "$@"
