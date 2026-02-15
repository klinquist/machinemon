# MachineMon

Lightweight, self-hosted server and client monitoring with real-time alerting. Think monit/M/Monit, but simpler.

MachineMon consists of a **server** (single Go binary with embedded web dashboard) and **client agents** that run on your machines. Clients check in every 2 minutes with CPU, memory, disk metrics, process status, and health check results. The server stores everything in SQLite, detects problems, and sends alerts via SMS, push notifications, or email.

**No external dependencies.** No Docker, no Postgres, no Redis. Just two binaries.

## Features

- **System Metrics** — CPU, memory, and disk usage tracked over time with configurable warning/critical thresholds
- **Process Monitoring** — Watch specific processes by name or regex. Get alerted when they die or restart (PID change)
- **Health Checks** — Run custom scripts on each check-in. Exit 0 = healthy, non-zero = unhealthy. Extensible to HTTP checks and file-touch checks
- **Web Dashboard** — Modern React SPA embedded in the server binary. View all your machines at a glance
- **Alerting** — Twilio (SMS), Pushover (push notifications), and SMTP (email). Smart hysteresis — only alerts on state changes, not every check-in
- **Per-Client Thresholds** — Override global defaults for individual machines
- **Muting** — Silence alerts for a client, optionally with an expiry time
- **Self-Hosted Binary Distribution** — The server can serve client binaries, so you can install clients with a single `curl | sh` command
- **Built-in HTTPS** — Let's Encrypt autocert, self-signed, or manual certificates. Or run behind nginx
- **Cross-Platform** — Client runs on Linux (x86_64, ARM64, ARMv6, ARMv7) and macOS (Intel, Apple Silicon). Perfect for Raspberry Pis
- **Single Binary, Zero Dependencies** — Pure Go with embedded SQLite (no CGO). Just copy the binary and run

---

## Quick Start

### 1. Set Up the Server

```bash
# Auto-detect platform and install
curl -sSL https://raw.githubusercontent.com/klinquist/machinemon/main/scripts/install-server.sh | sh

# Run interactive setup (sets admin password, client password, TLS mode, port)
machinemon-server --setup

# Install as a system service (auto-detects systemd, sysvinit, openrc, upstart, launchd)
sudo machinemon-server --service-install
```

Or download manually for a specific platform:

| Platform | Download |
|---|---|
| Linux x86_64 | `machinemon-server-linux-amd64` |
| Linux ARM64 | `machinemon-server-linux-arm64` |
| macOS Apple Silicon | `machinemon-server-darwin-arm64` |
| macOS Intel | `machinemon-server-darwin-amd64` |

```bash
# Example: manual download for Linux x86_64
curl -sSL https://github.com/klinquist/machinemon/releases/latest/download/machinemon-server-linux-amd64.tar.gz | tar xz
sudo mv machinemon-server-linux-amd64 /usr/local/bin/machinemon-server
```

The setup wizard will ask you for:
- An **admin password** (for the web dashboard and API)
- A **client password** (shared by all monitoring clients)
- A **TLS mode** (see [TLS Modes](#tls-modes) below)

Open your browser to `http://your-server:8080` (or the HTTPS URL if configured) and log in with username `admin` and your admin password.

### 2. Distribute Client Binaries (Optional)

If you want to install clients with a one-liner, put the pre-built client binaries on your server:

```bash
# On your build machine
make prepare-binaries

# Copy to your server's binaries directory
scp binaries/*.tar.gz you@server:~/.local/share/machinemon/binaries/
```

The default `binaries_dir` is `~/.local/share/machinemon/binaries` on Linux and `~/Library/Application Support/MachineMon/binaries` on macOS.

### 3. Install a Client

**From your server (recommended):**

```bash
curl -sSL https://your-server.com/download/install.sh | sh
```

This auto-detects your OS and architecture, downloads the right binary from your server, and installs a systemd/launchd service.

If your server uses a self-signed certificate:
```bash
curl -sSL --insecure https://your-server.com/download/install.sh | sh -s -- --insecure
```

**Then configure and start:**

```bash
# Interactive setup (asks for server URL, password, picks processes to watch)
machinemon-client --setup

# Or non-interactive
machinemon-client --setup \
  --server=https://your-server.com \
  --password=your_client_password \
  --no-daemon

# Install as a system service
sudo machinemon-client --service-install
```

### 4. See It in Action

Open your dashboard. Within 2 minutes, the client will appear with live metrics.

---

## Building from Source

### Prerequisites

- Go 1.21+
- Node.js 18+ and npm
- Make

### Build Everything

```bash
git clone https://github.com/klinquist/machinemon.git
cd machinemon

# Build React SPA + cross-compile client (6 platforms) + server (4 platforms)
make all

# Binaries are in dist/
ls dist/
```

### Individual Targets

```bash
make web              # Build React SPA only
make dev-client       # Build client for current platform → dist/machinemon-client
make dev-server       # Build server for current platform → dist/machinemon-server
make build-client     # Cross-compile client for all 6 platforms
make build-server     # Cross-compile server for all 4 platforms (includes web build)
make release          # Create .tar.gz archives + checksums.txt
make prepare-binaries # Package client .tar.gz files into binaries/ for server distribution
make test             # Run tests
make clean            # Remove all build artifacts
```

### Client Platforms

| OS | Architecture | Binary |
|---|---|---|
| Linux | x86_64 | `machinemon-client-linux-amd64` |
| Linux | ARM64 | `machinemon-client-linux-arm64` |
| Linux | ARMv7 | `machinemon-client-linux-armv7` |
| Linux | ARMv6 | `machinemon-client-linux-armv6` |
| macOS | Apple Silicon | `machinemon-client-darwin-arm64` |
| macOS | Intel | `machinemon-client-darwin-amd64` |

### Server Platforms

| OS | Architecture | Binary |
|---|---|---|
| Linux | x86_64 | `machinemon-server-linux-amd64` |
| Linux | ARM64 | `machinemon-server-linux-arm64` |
| macOS | Apple Silicon | `machinemon-server-darwin-arm64` |
| macOS | Intel | `machinemon-server-darwin-amd64` |

---

## Server Configuration

Config file location:
- **macOS:** `~/Library/Application Support/MachineMon/server.toml`
- **Linux:** `~/.config/machinemon/server.toml`

### Full Example

```toml
listen_addr = "0.0.0.0:8080"
external_url = "https://monitor.example.com"  # public URL (set when behind reverse proxy)
base_path = ""                                 # URL subpath (e.g. "/machinemon") for subpath deployments
database_path = "~/.local/share/machinemon/machinemon.db"
binaries_dir = "~/.local/share/machinemon/binaries"

# TLS: "none", "autocert", "selfsigned", "manual"
tls_mode = "none"
domain = ""             # required for autocert
cert_file = ""          # required for manual
key_file = ""           # required for manual
cert_cache_dir = ""     # auto-set

# Auth (set via --setup, don't edit directly)
admin_password_hash = "$2a$10$..."
client_password_hash = "$2a$10$..."

# Dev mode (for local development with Vite)
dev_mode = false
dev_proxy_url = "http://localhost:5173"
```

### Reference

| Field | Description | Default |
|---|---|---|
| `listen_addr` | Bind address (host:port) | `:8080` |
| `external_url` | Public URL for reverse proxy setups (e.g. `https://monitor.example.com`) | — |
| `base_path` | URL subpath prefix (e.g. `/machinemon`) for serving behind a subpath | — |
| `database_path` | SQLite database file path | `~/.local/share/machinemon/machinemon.db` |
| `binaries_dir` | Directory containing client `.tar.gz` files for download | `~/.local/share/machinemon/binaries` |
| `tls_mode` | `none`, `autocert`, `selfsigned`, or `manual` | `none` |
| `domain` | Domain for Let's Encrypt autocert | — |
| `cert_file` | Path to TLS certificate (manual mode) | — |
| `key_file` | Path to TLS private key (manual mode) | — |
| `cert_cache_dir` | Certificate cache directory | OS-specific |
| `admin_password_hash` | Bcrypt hash of admin password | Set via `--setup` |
| `client_password_hash` | Bcrypt hash of client password | Set via `--setup` |

---

## Client Configuration

Config file location:
- **macOS:** `~/Library/Application Support/MachineMon/client.toml`
- **Linux:** `~/.config/machinemon/client.toml`

### Full Example

```toml
client_id = ""                          # auto-assigned on first check-in
server_url = "https://monitor.example.com"
password = "your_client_password"
check_in_interval = 120                 # seconds
insecure_skip_tls = false               # set true for self-signed server certs

# Watch processes
[[process]]
friendly_name = "nginx"
match_pattern = "nginx"
match_type = "substring"                # "substring" (default) or "regex"

[[process]]
friendly_name = "my-api"
match_pattern = "node.*server\\.js"
match_type = "regex"

[[process]]
friendly_name = "postgres"
match_pattern = "postgres.*main"
match_type = "regex"

# Health checks
[[check]]
friendly_name = "API Health"
type = "script"
script_path = "curl -sf http://localhost:3000/health"

[[check]]
friendly_name = "Redis Ping"
type = "script"
script_path = "redis-cli ping | grep -q PONG"

[[check]]
friendly_name = "Backup Freshness"
type = "script"
script_path = "find /backup -name 'daily-*.tar.gz' -mmin -1440 | grep -q ."

[[check]]
friendly_name = "Disk SMART"
type = "script"
script_path = "/usr/local/bin/check_smart.sh"
```

### Reference

| Field | Description | Default |
|---|---|---|
| `client_id` | Unique identifier (auto-assigned) | — |
| `server_url` | Server URL | Required |
| `password` | Client authentication password | Required |
| `check_in_interval` | Seconds between check-ins | `120` |
| `insecure_skip_tls` | Skip TLS certificate verification | `false` |

### Process Configuration

Each `[[process]]` block watches a process:

| Field | Description |
|---|---|
| `friendly_name` | Display name in dashboard and alerts |
| `match_pattern` | String or regex to match against process command line |
| `match_type` | `substring` (default) or `regex` |

Process matching checks the full command line, not just the binary name. This means you can differentiate between multiple Node.js processes (e.g., `node server.js` vs `node worker.js`).

### Check Configuration

Each `[[check]]` block defines a health check:

| Field | Description |
|---|---|
| `friendly_name` | Display name in dashboard and alerts |
| `type` | Check type: `script` (more types planned) |
| `script_path` | Shell command or script path (for `script` type) |

**Script checks** run via `/bin/sh -c` with a 30-second timeout. Exit code 0 = healthy, anything else = unhealthy. The last 500 characters of output are captured and stored.

**Planned check types:**
- `http` — Check URL, verify status code and response time
- `file_touch` — Verify a file was modified within a time window (e.g., backup freshness)

---

## TLS Modes

### None (Reverse Proxy)

Best for production. Run MachineMon behind nginx, Caddy, or Traefik.

```toml
tls_mode = "none"
listen_addr = "127.0.0.1:8080"
external_url = "https://monitor.example.com"
```

The `--setup` wizard will ask you for the listen address (port) and the external URL. The `external_url` is used for generating install scripts and dashboard links — set it to the public URL that clients and browsers will use.

#### Subdomain (simplest)

Serve MachineMon at the root of a subdomain like `monitor.example.com`:

```nginx
server {
    listen 443 ssl http2;
    server_name monitor.example.com;

    ssl_certificate /etc/letsencrypt/live/monitor.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/monitor.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

#### Subpath (e.g. `/machinemon/`)

Serve MachineMon under a path on an existing domain. Set `base_path` in your server config:

```toml
base_path = "/machinemon"
external_url = "https://example.com/machinemon"
listen_addr = "127.0.0.1:8080"
tls_mode = "none"
```

Then configure nginx with a rewrite to strip the prefix:

```nginx
location /machinemon/ {
    rewrite ^/machinemon/(.*) /$1 break;
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

The nginx rewrite strips `/machinemon/` before forwarding to the server. The `base_path` config tells the SPA to generate correct links and API calls under the subpath.

#### Caddy

```
monitor.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

### Autocert (Let's Encrypt)

Automatic HTTPS certificate management. Requires ports 80 and 443 open, and a valid DNS record pointing to your server.

```toml
tls_mode = "autocert"
domain = "monitor.example.com"
listen_addr = ":443"
```

The server will automatically obtain and renew certificates from Let's Encrypt. It runs an HTTP challenge server on port 80.

### Self-Signed

Generates a self-signed ECDSA certificate (valid for 1 year, auto-regenerates). Good for internal/development use.

```toml
tls_mode = "selfsigned"
listen_addr = "0.0.0.0:8443"
```

Clients connecting to a self-signed server need `insecure_skip_tls = true` in their config, or pass `--insecure` during setup.

### Manual Certificate

Use your own certificate files (e.g., from a corporate CA).

```toml
tls_mode = "manual"
cert_file = "/etc/ssl/certs/machinemon.crt"
key_file = "/etc/ssl/private/machinemon.key"
listen_addr = ":443"
```

---

## Alert Providers

Configure notification channels via the web dashboard (Settings page) or the API.

### Twilio (SMS)

```json
{
  "type": "twilio",
  "name": "On-Call SMS",
  "enabled": true,
  "config": "{\"account_sid\":\"ACxxxxxxxx\",\"auth_token\":\"xxxxxxxx\",\"from_number\":\"+15551234567\",\"to_number\":\"+15559876543\"}"
}
```

### Pushover

```json
{
  "type": "pushover",
  "name": "Mobile Push",
  "enabled": true,
  "config": "{\"user_key\":\"xxxxxxxx\",\"api_token\":\"xxxxxxxx\"}"
}
```

### SMTP (Email)

```json
{
  "type": "smtp",
  "name": "Email Alerts",
  "enabled": true,
  "config": "{\"host\":\"smtp.gmail.com\",\"port\":587,\"username\":\"alerts@example.com\",\"password\":\"app-password\",\"from\":\"alerts@example.com\",\"to\":\"admin@example.com\",\"use_tls\":true}"
}
```

### Alert Types

| Alert Type | Severity | Trigger |
|---|---|---|
| `offline` | Critical | Client hasn't checked in for 4 minutes |
| `online` | Info | Client came back after being offline |
| `cpu_warn` / `cpu_crit` | Warning / Critical | CPU exceeds threshold |
| `cpu_recover` | Info | CPU dropped below warning threshold |
| `mem_warn` / `mem_crit` | Warning / Critical | Memory exceeds threshold |
| `mem_recover` | Info | Memory dropped below warning threshold |
| `disk_warn` / `disk_crit` | Warning / Critical | Disk exceeds threshold |
| `disk_recover` | Info | Disk dropped below warning threshold |
| `process_died` | Critical | Watched process stopped running |
| `pid_change` | Warning | Watched process restarted (new PID) |
| `check_failed` | Critical | Health check went from healthy to unhealthy |
| `check_recovered` | Info | Health check went from unhealthy to healthy |

### Default Thresholds

| Metric | Warning | Critical |
|---|---|---|
| CPU | 80% | 95% |
| Memory | 85% | 95% |
| Disk | 80% | 90% |

Override globally via Settings, or per-client via the client detail page.

---

## API Reference

All admin endpoints require HTTP Basic Auth: `admin:<admin_password>`.

### Client Check-In

```
POST /api/v1/checkin
Header: X-Client-Password: <client_password>
```

Used by clients. Not for manual use.

### Clients

```bash
# List all clients
curl -u admin:password https://monitor.example.com/api/v1/admin/clients

# Get client details (includes latest metrics, processes, checks)
curl -u admin:password https://monitor.example.com/api/v1/admin/clients/{id}

# Delete client (soft delete — will reappear if client checks in again)
curl -X DELETE -u admin:password https://monitor.example.com/api/v1/admin/clients/{id}

# Set per-client thresholds
curl -X PUT -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"cpu_warn_pct":90,"cpu_crit_pct":98,"mem_warn_pct":90,"mem_crit_pct":98,"disk_warn_pct":85,"disk_crit_pct":95}' \
  https://monitor.example.com/api/v1/admin/clients/{id}/thresholds

# Mute alerts (with optional duration in minutes)
curl -X PUT -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"muted":true,"duration_minutes":60,"reason":"Maintenance window"}' \
  https://monitor.example.com/api/v1/admin/clients/{id}/mute

# Unmute
curl -X PUT -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"muted":false}' \
  https://monitor.example.com/api/v1/admin/clients/{id}/mute

# Get metrics history
curl -u admin:password \
  "https://monitor.example.com/api/v1/admin/clients/{id}/metrics?from=2025-01-01T00:00:00Z&limit=100"

# Get process snapshots
curl -u admin:password https://monitor.example.com/api/v1/admin/clients/{id}/processes
```

### Alerts

```bash
# List alerts (paginated, filterable)
curl -u admin:password \
  "https://monitor.example.com/api/v1/admin/alerts?client_id={id}&severity=critical&limit=50&offset=0"
```

### Alert Providers

```bash
# List providers
curl -u admin:password https://monitor.example.com/api/v1/admin/providers

# Create provider
curl -X POST -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"type":"smtp","name":"Email","enabled":true,"config":"{...}"}' \
  https://monitor.example.com/api/v1/admin/providers

# Update provider
curl -X PUT -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"name":"Email (Updated)","enabled":true,"config":"{...}"}' \
  https://monitor.example.com/api/v1/admin/providers/{id}

# Delete provider
curl -X DELETE -u admin:password https://monitor.example.com/api/v1/admin/providers/{id}

# Test provider (sends a test notification)
curl -X POST -u admin:password https://monitor.example.com/api/v1/admin/providers/{id}/test
```

### Settings

```bash
# Get all settings
curl -u admin:password https://monitor.example.com/api/v1/admin/settings

# Update settings
curl -X PUT -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"offline_threshold_seconds":"300","cpu_warn_pct_default":"85"}' \
  https://monitor.example.com/api/v1/admin/settings

# Change admin or client password
curl -X PUT -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"type":"admin","password":"new_password"}' \
  https://monitor.example.com/api/v1/admin/password
```

### Downloads (Public, No Auth)

```bash
# Get install script (auto-detects server URL)
curl -sSL https://monitor.example.com/download/install.sh | sh

# List available binaries
curl https://monitor.example.com/download/

# Download a specific binary
curl -O https://monitor.example.com/download/machinemon-client-linux-arm64.tar.gz
```

### Health Check

```bash
curl https://monitor.example.com/healthz
# {"status":"ok"}
```

---

## Deployment

### Installing as a Service

Both binaries have a built-in `--service-install` flag that auto-detects your init system and creates the appropriate service file:

```bash
# Install server as a service
sudo machinemon-server --service-install

# Install client as a service
sudo machinemon-client --service-install
```

Supported init systems:

| Init System | Platforms | Service File |
|---|---|---|
| **systemd** | Most modern Linux (Ubuntu 16+, Debian 8+, CentOS 7+, Fedora, Arch) | `/etc/systemd/system/machinemon-*.service` |
| **SysVInit** | Older Linux (Ubuntu 14 and earlier, Debian 7 and earlier) | `/etc/init.d/machinemon-*` |
| **OpenRC** | Alpine Linux, Gentoo | `/etc/init.d/machinemon-*` |
| **Upstart** | Ubuntu 9.10–14.10 | `/etc/init/machinemon-*.conf` |
| **launchd** | macOS | `~/Library/LaunchAgents/com.machinemon.*.plist` |

To remove a service:
```bash
sudo machinemon-server --service-uninstall
sudo machinemon-client --service-uninstall
```

### Binary Distribution from Server

After building, package the client binaries for your server to serve:

```bash
# Build client binaries for all platforms
make prepare-binaries

# Copy to your server
scp binaries/*.tar.gz you@server:~/.local/share/machinemon/binaries/

# Verify (from any machine)
curl https://your-server.com/download/
```

Now anyone can install a client with:
```bash
curl -sSL https://your-server.com/download/install.sh | sh
```

### Uninstalling

```bash
# Remove the service, then the binary
sudo machinemon-client --service-uninstall
sudo rm /usr/local/bin/machinemon-client

sudo machinemon-server --service-uninstall
sudo rm /usr/local/bin/machinemon-server
```

Or use the interactive uninstaller:
```bash
curl -sSL https://raw.githubusercontent.com/klinquist/machinemon/main/scripts/uninstall.sh | sh
```

Config files and database are preserved by default. Remove them manually if desired.

---

## Architecture

```text
+------------------------------------------------------------+
|                     MachineMon Server                      |
|                                                            |
|  +------------+   +--------------+                         |
|  | React SPA  |   | Chi Router   |                         |
|  | (embedded) |   | /api/v1/...  |                         |
|  +------+-----+   +------+-------+                         |
|         |                |                                 |
|         +----------------+                                 |
|                  |                                         |
|          +-------v------------------------------+          |
|          |             Alert Engine             |          |
|          |  Offline detection (30s loop)        |          |
|          |  Threshold hysteresis                |          |
|          |  Process state tracking              |          |
|          |  Check failure detection             |          |
|          +-------+----------------------+-------+          |
|                  |                      |                  |
|      +-----------v------+    +----------v----------+       |
|      | SQLite (single   |    | Dispatcher          |       |
|      | file DB)         |    | -> Twilio           |       |
|      +------------------+    | -> Pushover         |       |
|                              | -> SMTP             |       |
|                              +---------------------+       |
+---------------------------+--------------------------------+
                            |
          HTTPS POST /api/v1/checkin (every 2 min)
                            |
              +-------------+-------------+
              |             |             |
      +-------v------+ +----v-------+ +---v--------+
      | Client       | | Client     | | Client     |
      | Pi Zero      | | Ubuntu VM  | | Mac Mini   |
      | (ARMv6)      | | (x86_64)   | | (ARM64)    |
      | Metrics      | | Metrics    | | Metrics    |
      | Processes    | | Processes  | | Processes  |
      | Checks       | | Checks     | | Checks     |
      +--------------+ +------------+ +------------+
```

### Key Design Decisions

- **Pure Go SQLite** (`modernc.org/sqlite`) — No CGO needed. Cross-compiles to ARM without a C toolchain
- **Embedded SPA** — React dashboard is compiled into the server binary via `//go:embed`. One binary to deploy
- **Alert Hysteresis** — Alerts fire on state *changes* only (normal→warn, warn→crit, crit→recover). No alert storms
- **Extensible Checks** — Each check has a `type` and a `state` JSON blob. New check types can be added to the client without changing the server schema
- **Client-Side Matching** — Process matching and health checks run on the client. The server only stores results and evaluates transitions

---

## Troubleshooting

### Server won't start

```bash
# Check the config file is valid
machinemon-server --config /path/to/server.toml --version

# Check logs
journalctl -u machinemon-server -n 50 --no-pager

# Common issues:
# - Port already in use → change listen_addr
# - "admin password is required" → run machinemon-server --setup
# - Database permissions → check database_path directory is writable
```

### Client can't connect

```bash
# Test connectivity manually
machinemon-client --server=https://your-server.com --password=test --no-daemon

# Common issues:
# - "authentication failed" → wrong client password
# - TLS errors → add --insecure for self-signed certs, or set insecure_skip_tls in config
# - Connection refused → check firewall, server listen_addr
# - "connection reset" → server may not be running
```

### Alerts not sending

```bash
# Test a provider via API
curl -X POST -u admin:password https://your-server.com/api/v1/admin/providers/1/test

# Check for errors
journalctl -u machinemon-server | grep -i "alert\|dispatch\|provider"

# Common issues:
# - Provider not enabled → check enabled flag in Settings
# - Invalid credentials → update provider config
# - Client muted → unmute via dashboard or API
# - SMTP blocked → check your email provider allows app passwords
```

### Client not appearing on dashboard

- Wait 2 minutes for the first check-in
- Check client logs: `journalctl -u machinemon-client -f` or `tail -f /tmp/machinemon-client.log`
- Verify the server URL and password match what was set during server setup
- If the client was previously deleted, it will reappear on next check-in

### Process not being detected

- Check that the `match_pattern` matches part of the full command line (not just the binary name)
- For Node.js apps, match on the script name: `match_pattern = "node.*my-app.js"`
- Use `match_type = "regex"` for complex patterns
- Run `ps aux | grep your-pattern` to verify the pattern matches

---

## License

MIT
