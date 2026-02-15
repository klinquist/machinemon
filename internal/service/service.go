package service

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
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

// Start starts (and enables where appropriate) the service under the detected init system.
func Start(name string) error {
	initSys := Detect()
	switch initSys {
	case Systemd:
		return runPrivileged("systemctl", "enable", "--now", name)
	case SysVInit:
		return runPrivileged("service", name, "start")
	case OpenRC:
		if err := runPrivileged("rc-service", name, "start"); err != nil {
			return err
		}
		return runPrivileged("rc-update", "add", name, "default")
	case Upstart:
		return runPrivileged("start", name)
	case Launchd:
		return startLaunchd(name)
	default:
		return fmt.Errorf("could not detect init system")
	}
}

// IsRunning reports whether a service appears to be running under the detected init system.
func IsRunning(name string) (bool, error) {
	initSys := Detect()
	switch initSys {
	case Systemd:
		cmd := exec.Command("systemctl", "is-active", "--quiet", name)
		if err := cmd.Run(); err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return false, nil
			}
			return false, err
		}
		return true, nil
	case SysVInit:
		cmd := exec.Command("service", name, "status")
		if err := cmd.Run(); err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return false, nil
			}
			return false, err
		}
		return true, nil
	case OpenRC:
		cmd := exec.Command("rc-service", name, "status")
		if err := cmd.Run(); err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return false, nil
			}
			return false, err
		}
		return true, nil
	case Upstart:
		out, err := exec.Command("status", name).CombinedOutput()
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return false, nil
			}
			return false, err
		}
		return strings.Contains(string(out), "start/running"), nil
	case Launchd:
		target, err := launchdTarget()
		if err != nil {
			return false, err
		}
		label := "com.machinemon." + strings.TrimPrefix(name, "machinemon-")
		out, err := runLaunchctl(target, "list", label)
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return false, nil
			}
			return false, err
		}
		return strings.Contains(string(out), label), nil
	default:
		return false, nil
	}
}

// Restart restarts the service under the detected init system.
func Restart(name string) error {
	initSys := Detect()
	switch initSys {
	case Systemd:
		return runPrivileged("systemctl", "restart", name)
	case SysVInit:
		return runPrivileged("service", name, "restart")
	case OpenRC:
		return runPrivileged("rc-service", name, "restart")
	case Upstart:
		return runPrivileged("restart", name)
	case Launchd:
		return startLaunchd(name)
	default:
		return fmt.Errorf("could not detect init system")
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

type launchdUser struct {
	Username string
	UID      string
	HomeDir  string
}

func launchdTarget() (*launchdUser, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("launchd is only available on macOS")
	}

	// If invoked with sudo, target the invoking desktop user, not root.
	if os.Getuid() == 0 {
		sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER"))
		if sudoUser == "" {
			return nil, fmt.Errorf("running as root on macOS is not supported without SUDO_USER; run without sudo")
		}
		u, err := user.Lookup(sudoUser)
		if err != nil {
			return nil, fmt.Errorf("lookup sudo user %q: %w", sudoUser, err)
		}
		if strings.TrimSpace(u.Uid) == "" || strings.TrimSpace(u.HomeDir) == "" {
			return nil, fmt.Errorf("invalid sudo user account info for %q", sudoUser)
		}
		return &launchdUser{
			Username: sudoUser,
			UID:      u.Uid,
			HomeDir:  u.HomeDir,
		}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil, fmt.Errorf("determine user home: %w", err)
	}
	uid := strconv.Itoa(os.Getuid())
	username := ""
	if u, err := user.Current(); err == nil {
		if strings.TrimSpace(u.Username) != "" {
			username = u.Username
		}
		if strings.TrimSpace(u.Uid) != "" {
			uid = u.Uid
		}
	}
	return &launchdUser{
		Username: username,
		UID:      uid,
		HomeDir:  home,
	}, nil
}

func runLaunchctl(target *launchdUser, args ...string) ([]byte, error) {
	if os.Getuid() == 0 && target != nil && strings.TrimSpace(target.Username) != "" {
		sudoArgs := append([]string{"-u", target.Username, "launchctl"}, args...)
		return exec.Command("sudo", sudoArgs...).CombinedOutput()
	}
	return exec.Command("launchctl", args...).CombinedOutput()
}

func startLaunchd(name string) error {
	target, err := launchdTarget()
	if err != nil {
		return err
	}

	label := "com.machinemon." + strings.TrimPrefix(name, "machinemon-")
	path := filepath.Join(target.HomeDir, "Library", "LaunchAgents", label+".plist")
	domain := "gui/" + target.UID

	bootstrapOut, bootstrapErr := runLaunchctl(target, "bootstrap", domain, path)
	if bootstrapErr != nil {
		msg := strings.ToLower(string(bootstrapOut))
		// Already loaded/bootstrapped is fine.
		if !strings.Contains(msg, "already") && !strings.Contains(msg, "in bootstrap") && !strings.Contains(msg, "exists") {
			return fmt.Errorf("launchctl bootstrap failed: %v (%s)", bootstrapErr, strings.TrimSpace(string(bootstrapOut)))
		}
	}

	kickstartOut, kickstartErr := runLaunchctl(target, "kickstart", "-k", domain+"/"+label)
	if kickstartErr != nil {
		return fmt.Errorf("launchctl kickstart failed: %v (%s)", kickstartErr, strings.TrimSpace(string(kickstartOut)))
	}
	return nil
}

func installLaunchd(name, binPath, configPath string) error {
	target, err := launchdTarget()
	if err != nil {
		return err
	}

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

	dir := filepath.Join(target.HomeDir, "Library", "LaunchAgents")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, label+".plist")

	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	if os.Getuid() == 0 {
		if uid, err := strconv.Atoi(target.UID); err == nil {
			_ = os.Chown(path, uid, -1)
		}
	}

	fmt.Printf("LaunchAgent installed: %s\n", path)
	fmt.Println()
	fmt.Printf("  Start now:   launchctl bootstrap gui/%s %s\n", target.UID, path)
	fmt.Printf("               launchctl kickstart -k gui/%s/%s\n", target.UID, label)
	fmt.Printf("  Check logs:  tail -f /tmp/%s.log\n", name)
	return nil
}

func uninstallLaunchd(name string) error {
	target, err := launchdTarget()
	if err != nil {
		return err
	}

	label := "com.machinemon." + strings.TrimPrefix(name, "machinemon-")
	path := filepath.Join(target.HomeDir, "Library", "LaunchAgents", label+".plist")

	_, _ = runLaunchctl(target, "bootout", "gui/"+target.UID+"/"+label)
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
