#!/bin/sh
set -e

# MachineMon Uninstaller
# Removes client and/or server binaries and services.

echo "=== MachineMon Uninstaller ==="
echo ""

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    SUDO="sudo"
fi

uninstall_client() {
    echo "Uninstalling MachineMon Client..."

    # Stop and disable systemd service
    if [ -f /etc/systemd/system/machinemon-client.service ]; then
        $SUDO systemctl stop machinemon-client 2>/dev/null || true
        $SUDO systemctl disable machinemon-client 2>/dev/null || true
        $SUDO rm -f /etc/systemd/system/machinemon-client.service
        $SUDO systemctl daemon-reload 2>/dev/null || true
        echo "  Removed systemd service"
    fi

    # Unload and remove launchd plist
    PLIST="$HOME/Library/LaunchAgents/com.machinemon.client.plist"
    if [ -f "$PLIST" ]; then
        launchctl unload "$PLIST" 2>/dev/null || true
        rm -f "$PLIST"
        echo "  Removed launchd plist"
    fi

    # Remove binary
    if [ -f /usr/local/bin/machinemon-client ]; then
        $SUDO rm -f /usr/local/bin/machinemon-client
        echo "  Removed binary"
    fi

    echo "  Client uninstalled."
    echo "  Config files preserved. Remove manually if desired:"
    case "$OS" in
        darwin) echo "    ~/Library/Application Support/MachineMon/client.toml" ;;
        *)      echo "    ~/.config/machinemon/client.toml" ;;
    esac
}

uninstall_server() {
    echo "Uninstalling MachineMon Server..."

    # Stop and disable systemd service
    if [ -f /etc/systemd/system/machinemon-server.service ]; then
        $SUDO systemctl stop machinemon-server 2>/dev/null || true
        $SUDO systemctl disable machinemon-server 2>/dev/null || true
        $SUDO rm -f /etc/systemd/system/machinemon-server.service
        $SUDO systemctl daemon-reload 2>/dev/null || true
        echo "  Removed systemd service"
    fi

    # Unload and remove launchd plist
    PLIST="$HOME/Library/LaunchAgents/com.machinemon.server.plist"
    if [ -f "$PLIST" ]; then
        launchctl unload "$PLIST" 2>/dev/null || true
        rm -f "$PLIST"
        echo "  Removed launchd plist"
    fi

    # Remove binary
    if [ -f /usr/local/bin/machinemon-server ]; then
        $SUDO rm -f /usr/local/bin/machinemon-server
        echo "  Removed binary"
    fi

    echo "  Server uninstalled."
    echo "  Config and database files preserved. Remove manually if desired:"
    case "$OS" in
        darwin)
            echo "    ~/Library/Application Support/MachineMon/server.toml"
            echo "    ~/Library/Application Support/MachineMon/machinemon.db"
            ;;
        *)
            echo "    ~/.config/machinemon/server.toml"
            echo "    ~/.local/share/machinemon/machinemon.db"
            ;;
    esac
}

echo "What would you like to uninstall?"
echo "  1. Client only"
echo "  2. Server only"
echo "  3. Both"
printf "Choose [3]: "
read -r choice

case "$choice" in
    1) uninstall_client ;;
    2) uninstall_server ;;
    *) uninstall_client; echo ""; uninstall_server ;;
esac

echo ""
echo "Uninstall complete."
