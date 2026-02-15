#!/bin/sh
set -e

# MachineMon Server Installer
# Usage: curl -sSL https://your-server/install-server.sh | sh

INSTALL_DIR="/usr/local/bin"
REPO="klinquist/machinemon"
BINARY="machinemon-server"

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)
            echo "Unsupported architecture for server: $ARCH"
            echo "Server is supported on amd64 and arm64 only."
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
        echo "  Contents:"
        head -c 200 "$TMP_DIR/archive.tar.gz"
        echo ""
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

# Install systemd service (Linux)
install_systemd_service() {
    if [ ! -d /etc/systemd/system ]; then
        return
    fi

    cat > /tmp/machinemon-server.service <<'SYSTEMD'
[Unit]
Description=MachineMon Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/machinemon-server
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
# Allow binding to ports < 1024 (for HTTPS on 443)
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
SYSTEMD

    if [ "$(id -u)" -eq 0 ]; then
        mv /tmp/machinemon-server.service /etc/systemd/system/
        systemctl daemon-reload
    else
        sudo mv /tmp/machinemon-server.service /etc/systemd/system/
        sudo systemctl daemon-reload
    fi

    echo ""
    echo "Systemd service installed. To start:"
    echo "  1. Run setup:   machinemon-server --setup"
    echo "  2. Start:       sudo systemctl enable --now machinemon-server"
    echo "  3. Check logs:  journalctl -u machinemon-server -f"
}

# Install launchd plist (macOS)
install_launchd_plist() {
    if [ "$OS" != "darwin" ]; then
        return
    fi

    PLIST_DIR="$HOME/Library/LaunchAgents"
    mkdir -p "$PLIST_DIR"

    cat > "$PLIST_DIR/com.machinemon.server.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.machinemon.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/machinemon-server.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/machinemon-server.log</string>
</dict>
</plist>
PLIST

    echo ""
    echo "LaunchAgent plist installed. To start:"
    echo "  1. Run setup:   machinemon-server --setup"
    echo "  2. Start:       launchctl load $PLIST_DIR/com.machinemon.server.plist"
    echo "  3. Check logs:  tail -f /tmp/machinemon-server.log"
}

main() {
    echo "=== MachineMon Server Installer ==="
    echo ""

    detect_platform
    download_binary

    case "$OS" in
        linux)  install_systemd_service ;;
        darwin) install_launchd_plist ;;
    esac

    echo ""
    echo "Installation complete!"
    echo "Run 'machinemon-server --setup' to configure."
}

main "$@"
