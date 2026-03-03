#!/usr/bin/env bash
# Launcher script for msgraph CLI.
# Executes the pre-bundled binary for the detected platform.
set -euo pipefail

BINARY_NAME="msgraph"
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

# Main logic
main() {
    local platform binary_path

    platform=$(detect_platform)
    binary_path="${BIN_DIR}/${BINARY_NAME}_${platform}"

    if [ ! -f "${binary_path}" ]; then
        echo "Error: Binary not found: ${binary_path}" >&2
        echo "Expected a pre-bundled binary for platform '${platform}'." >&2
        echo "Please reinstall the skill or download the correct release." >&2
        exit 1
    fi

    chmod +x "${binary_path}"
    exec "${binary_path}" "$@"
}

main "$@"
