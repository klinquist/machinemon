#!/bin/sh
set -e

# MachineMon Client Installer (GitHub Releases)
# Usage: curl -sSL https://raw.githubusercontent.com/klinquist/machinemon/main/scripts/install-client.sh | sh
#
# TIP: If your MachineMon server is set up with binaries, use the server-hosted
# installer instead — it auto-detects the server URL:
#   curl -sSL https://your-server.com/download/install.sh | sh

INSTALL_DIR="/usr/local/bin"
REPO="klinquist/machinemon"
BINARY="machinemon-client"

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7*)        ARCH="armv7" ;;
        armv6*)        ARCH="armv6" ;;
        arm*)
            # Detect ARM version from /proc/cpuinfo on Linux
            if [ -f /proc/cpuinfo ]; then
                ARM_VER=$(grep -oP 'model name.*ARMv\K[0-9]+' /proc/cpuinfo 2>/dev/null || echo "6")
                if [ "$ARM_VER" -ge 7 ] 2>/dev/null; then
                    ARCH="armv7"
                else
                    ARCH="armv6"
                fi
            else
                ARCH="armv6"
            fi
            ;;
        *)
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        *)
            echo "Unsupported OS: $OS"
            exit 1
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    echo "Detected platform: $PLATFORM"
}

# Download the binary
download_binary() {
    DOWNLOAD_NAME="${BINARY}-${PLATFORM}"

    if [ -n "$VERSION" ]; then
        URL="https://github.com/${REPO}/releases/download/${VERSION}/${DOWNLOAD_NAME}.tar.gz"
    else
        URL="https://github.com/${REPO}/releases/latest/download/${DOWNLOAD_NAME}.tar.gz"
    fi

    echo "Downloading ${DOWNLOAD_NAME}..."
    echo "  URL: ${URL}"

    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    HTTP_CODE=""
    if command -v curl >/dev/null 2>&1; then
        HTTP_CODE=$(curl -sSL -w '%{http_code}' "$URL" -o "$TMP_DIR/archive.tar.gz")
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$URL" -O "$TMP_DIR/archive.tar.gz"
    else
        echo "Error: curl or wget is required"
        exit 1
    fi

    # Verify we got a valid download
    if [ -n "$HTTP_CODE" ] && [ "$HTTP_CODE" != "200" ]; then
        echo "Error: download failed with HTTP status $HTTP_CODE"
        echo "  Check that the release exists at: https://github.com/${REPO}/releases"
        exit 1
    fi

    FILESIZE=$(wc -c < "$TMP_DIR/archive.tar.gz" | tr -d ' ')
    if [ "$FILESIZE" -lt 1000 ]; then
        echo "Error: downloaded file is too small (${FILESIZE} bytes) — likely not a valid binary"
        exit 1
    fi

    cd "$TMP_DIR"
    tar xzf archive.tar.gz

    # Install binary
    if [ "$(id -u)" -eq 0 ]; then
        mv "$DOWNLOAD_NAME" "${INSTALL_DIR}/${BINARY}"
        chmod 755 "${INSTALL_DIR}/${BINARY}"
    else
        echo "Installing to ${INSTALL_DIR} requires root. Using sudo..."
        sudo mv "$DOWNLOAD_NAME" "${INSTALL_DIR}/${BINARY}"
        sudo chmod 755 "${INSTALL_DIR}/${BINARY}"
    fi

    echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
}

main() {
    echo "=== MachineMon Client Installer ==="
    echo ""

    detect_platform
    download_binary

    echo ""
    echo "Installation complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Run setup:          machinemon-client --setup"
    if [ "$OS" = "darwin" ]; then
        echo "  2. Install as service: machinemon-client --service-install"
    else
        echo "  2. Install as service: sudo machinemon-client --service-install"
    fi
    echo "     (auto-detects systemd, sysvinit, openrc, upstart, or launchd)"
}

main "$@"
