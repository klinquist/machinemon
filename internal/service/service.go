package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type InitSystem string

const (
	Systemd  InitSystem = "systemd"
	SysVInit InitSystem = "sysvinit"
	OpenRC   InitSystem = "openrc"
	Upstart  InitSystem = "upstart"
	Launchd  InitSystem = "launchd"
	Unknown  InitSystem = ""
)

// Detect returns the init system in use on this machine.
func Detect() InitSystem {
	if runtime.GOOS == "darwin" {
		return Launchd
	}

	// systemd: check for systemctl binary
	if _, err := exec.LookPath("systemctl"); err == nil {
		return Systemd
	}

	// OpenRC: check for rc-service binary
	if _, err := exec.LookPath("rc-service"); err == nil {
		return OpenRC
	}

	// Upstart: check for initctl binary and that it's actually upstart (not systemd compat)
	if _, err := exec.LookPath("initctl"); err == nil {
		out, err := exec.Command("initctl", "version").CombinedOutput()
		if err == nil && strings.Contains(string(out), "upstart") {
			return Upstart
		}
	}

	// SysVInit: check for /etc/init.d directory
	if info, err := os.Stat("/etc/init.d"); err == nil && info.IsDir() {
		return SysVInit
	}

	return Unknown
}

// Install installs a system service for the given binary.
// name is the service name (e.g. "machinemon-server").
// binPath is the absolute path to the binary.
// configPath is the absolute path to the config file to pass via --config flag.
func Install(name, binPath, configPath string) error {
	initSys := Detect()

	fmt.Printf("Detected init system: %s\n", initSys)

	switch initSys {
	case Systemd:
		return installSystemd(name, binPath, configPath)
	case SysVInit:
		return installSysVInit(name, binPath, configPath)
	case OpenRC:
		return installOpenRC(name, binPath, configPath)
	case Upstart:
		return installUpstart(name, binPath, configPath)
	case Launchd:
		return installLaunchd(name, binPath, configPath)
	default:
		return fmt.Errorf("could not detect init system — install service manually")
	}
}

// Uninstall removes the system service.
func Uninstall(name string) error {
	initSys := Detect()

	fmt.Printf("Detected init system: %s\n", initSys)

	switch initSys {
	case Systemd:
		return uninstallSystemd(name)
	case SysVInit:
		return uninstallSysVInit(name)
	case OpenRC:
		return uninstallOpenRC(name)
	case Upstart:
		return uninstallUpstart(name)
	case Launchd:
		return uninstallLaunchd(name)
	default:
		return fmt.Errorf("could not detect init system — remove service manually")
	}
}

// execLine builds the command line for service files.
func execLine(binPath, configPath string) string {
	if configPath != "" {
		return fmt.Sprintf("%s --config %s", binPath, configPath)
	}
	return binPath
}

// runPrivileged runs a command, prepending sudo if not root.
func runPrivileged(name string, args ...string) error {
	if os.Getuid() != 0 {
		args = append([]string{name}, args...)
		name = "sudo"
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// writePrivileged writes content to a file, using sudo tee if not root.
func writePrivileged(path, content string) error {
	if os.Getuid() == 0 {
		return os.WriteFile(path, []byte(content), 0644)
	}
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = nil // suppress tee's stdout echo
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// removePrivileged removes a file, using sudo if not root.
func removePrivileged(path string) error {
	if os.Getuid() == 0 {
		return os.Remove(path)
	}
	return exec.Command("sudo", "rm", "-f", path).Run()
}

// --- systemd ---

func installSystemd(name, binPath, configPath string) error {
	unit := fmt.Sprintf(`[Unit]
Description=MachineMon %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, serviceLabel(name), execLine(binPath, configPath))

	path := fmt.Sprintf("/etc/systemd/system/%s.service", name)
	if err := writePrivileged(path, unit); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	if err := runPrivileged("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	fmt.Printf("Systemd service installed: %s\n", path)
	fmt.Println()
	fmt.Printf("  Start now:   sudo systemctl enable --now %s\n", name)
	fmt.Printf("  Check status: sudo systemctl status %s --no-pager -l\n", name)
	fmt.Printf("  Check logs:   sudo journalctl -u %s -f\n", name)
	return nil
}

func uninstallSystemd(name string) error {
	_ = runPrivileged("systemctl", "stop", name)
	_ = runPrivileged("systemctl", "disable", name)
	path := fmt.Sprintf("/etc/systemd/system/%s.service", name)
	if err := removePrivileged(path); err != nil {
		return err
	}
	_ = runPrivileged("systemctl", "daemon-reload")
	fmt.Printf("Systemd service removed: %s\n", name)
	return nil
}

// --- SysVInit ---

func installSysVInit(name, binPath, configPath string) error {
	cmd := execLine(binPath, configPath)
	script := fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides:          %s
# Required-Start:    $network $remote_fs
# Required-Stop:     $network $remote_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: MachineMon %s
### END INIT INFO

DAEMON_CMD="%s"
PIDFILE=/var/run/%s.pid
LOGFILE=/var/log/%s.log

case "$1" in
  start)
    echo "Starting %s..."
    if [ -f "$PIDFILE" ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; then
      echo "Already running (PID $(cat "$PIDFILE"))"
      exit 0
    fi
    nohup $DAEMON_CMD >> "$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    echo "Started (PID $!)"
    ;;
  stop)
    echo "Stopping %s..."
    if [ -f "$PIDFILE" ]; then
      kill $(cat "$PIDFILE") 2>/dev/null
      rm -f "$PIDFILE"
      echo "Stopped"
    else
      echo "Not running"
    fi
    ;;
  restart)
    $0 stop
    sleep 1
    $0 start
    ;;
  status)
    if [ -f "$PIDFILE" ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; then
      echo "%s is running (PID $(cat "$PIDFILE"))"
    else
      echo "%s is not running"
      exit 1
    fi
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|status}"
    exit 1
    ;;
esac
`, name, serviceLabel(name), cmd, name, name, name, name, name, name)

	path := fmt.Sprintf("/etc/init.d/%s", name)
	if err := writePrivileged(path, script); err != nil {
		return fmt.Errorf("write init script: %w", err)
	}
	if err := runPrivileged("chmod", "755", path); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Try to register with update-rc.d (Debian/Ubuntu) or chkconfig (RHEL/CentOS)
	if _, err := exec.LookPath("update-rc.d"); err == nil {
		_ = runPrivileged("update-rc.d", name, "defaults")
	} else if _, err := exec.LookPath("chkconfig"); err == nil {
		_ = runPrivileged("chkconfig", "--add", name)
	}

	fmt.Printf("SysVInit service installed: %s\n", path)
	fmt.Println()
	fmt.Printf("  Start now:   sudo service %s start\n", name)
	fmt.Printf("  Auto-start:  (already enabled via update-rc.d/chkconfig)\n")
	fmt.Printf("  Check logs:  tail -f /var/log/%s.log\n", name)
	return nil
}

func uninstallSysVInit(name string) error {
	path := fmt.Sprintf("/etc/init.d/%s", name)
	_ = runPrivileged(path, "stop")

	if _, err := exec.LookPath("update-rc.d"); err == nil {
		_ = runPrivileged("update-rc.d", "-f", name, "remove")
	} else if _, err := exec.LookPath("chkconfig"); err == nil {
		_ = runPrivileged("chkconfig", "--del", name)
	}

	if err := removePrivileged(path); err != nil {
		return err
	}
	fmt.Printf("SysVInit service removed: %s\n", name)
	return nil
}

// --- OpenRC ---

func installOpenRC(name, binPath, configPath string) error {
	script := fmt.Sprintf(`#!/sbin/openrc-run

name="%s"
description="MachineMon %s"
command="%s"
command_args="%s"
command_background=true
pidfile="/run/${RC_SVCNAME}.pid"
output_log="/var/log/%s.log"
error_log="/var/log/%s.log"

depend() {
    need net
    after firewall
}
`, name, serviceLabel(name), binPath, configFlag(configPath), name, name)

	path := fmt.Sprintf("/etc/init.d/%s", name)
	if err := writePrivileged(path, script); err != nil {
		return fmt.Errorf("write init script: %w", err)
	}
	if err := runPrivileged("chmod", "755", path); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	fmt.Printf("OpenRC service installed: %s\n", path)
	fmt.Println()
	fmt.Printf("  Start now:   sudo rc-service %s start\n", name)
	fmt.Printf("  Auto-start:  sudo rc-update add %s default\n", name)
	fmt.Printf("  Check logs:  tail -f /var/log/%s.log\n", name)
	return nil
}

func uninstallOpenRC(name string) error {
	_ = runPrivileged("rc-service", name, "stop")
	_ = runPrivileged("rc-update", "del", name)
	path := fmt.Sprintf("/etc/init.d/%s", name)
	if err := removePrivileged(path); err != nil {
		return err
	}
	fmt.Printf("OpenRC service removed: %s\n", name)
	return nil
}

// --- Upstart ---

func installUpstart(name, binPath, configPath string) error {
	conf := fmt.Sprintf(`description "MachineMon %s"

start on runlevel [2345]
stop on runlevel [!2345]

respawn
respawn limit 10 5

exec %s >> /var/log/%s.log 2>&1
`, serviceLabel(name), execLine(binPath, configPath), name)

	path := fmt.Sprintf("/etc/init/%s.conf", name)
	if err := writePrivileged(path, conf); err != nil {
		return fmt.Errorf("write upstart conf: %w", err)
	}

	fmt.Printf("Upstart service installed: %s\n", path)
	fmt.Println()
	fmt.Printf("  Start now:   sudo start %s\n", name)
	fmt.Printf("  Check logs:  tail -f /var/log/%s.log\n", name)
	return nil
}

func uninstallUpstart(name string) error {
	_ = runPrivileged("stop", name)
	path := fmt.Sprintf("/etc/init/%s.conf", name)
	if err := removePrivileged(path); err != nil {
		return err
	}
	fmt.Printf("Upstart service removed: %s\n", name)
	return nil
}

// --- launchd ---

func installLaunchd(name, binPath, configPath string) error {
	label := "com.machinemon." + strings.TrimPrefix(name, "machinemon-")

	args := fmt.Sprintf(`        <string>%s</string>`, binPath)
	if configPath != "" {
		args += fmt.Sprintf("\n        <string>--config</string>\n        <string>%s</string>", configPath)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
%s
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/%s.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/%s.log</string>
</dict>
</plist>
`, label, args, name, name)

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Library", "LaunchAgents")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, label+".plist")

	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	fmt.Printf("LaunchAgent installed: %s\n", path)
	fmt.Println()
	fmt.Printf("  Start now:   launchctl load %s\n", path)
	fmt.Printf("  Check logs:  tail -f /tmp/%s.log\n", name)
	return nil
}

func uninstallLaunchd(name string) error {
	label := "com.machinemon." + strings.TrimPrefix(name, "machinemon-")
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "LaunchAgents", label+".plist")

	_ = exec.Command("launchctl", "unload", path).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Printf("LaunchAgent removed: %s\n", name)
	return nil
}

func serviceLabel(name string) string {
	switch name {
	case "machinemon-server":
		return "Server"
	case "machinemon-client":
		return "Client"
	default:
		return name
	}
}

func configFlag(configPath string) string {
	if configPath != "" {
		return "--config " + configPath
	}
	return ""
}
