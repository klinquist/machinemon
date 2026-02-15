# MachineMon Client Auto-Update Plan

## Goal

Enable MachineMon clients to automatically update themselves from the MachineMon server's hosted binaries (`/download`) when a newer version is available.

## Current State

- Server already hosts client tarballs under `/download/<filename>.tar.gz`.
- Clients already report `client_version` on every check-in.
- Service management supports restart across init systems, including `systemd` on Ubuntu.
- Check-in response currently returns only:
  - `client_id`
  - `next_checkin_seconds`
  - `server_time`

## Proposed Design (V1)

### 1. Server Update Metadata

- During release/publish, generate a manifest file (e.g. `client-manifest.json`) in `binaries_dir`.
- Manifest includes:
  - target client version
  - per-platform archive filename
  - SHA-256 checksum
  - size
- Expose manifest at a stable endpoint:
  - `GET /download/client-manifest.json`

### 2. Update Signal via Check-In Response

- Extend check-in response with optional update fields:
  - `update_version`
  - `update_url`
  - `update_sha256`
  - `update_size`
- Server compares incoming `client_version` to manifest version.
- If different, server includes update directive for that client’s OS/arch.
- If not different, fields are omitted.
- Backward compatibility: older clients ignore unknown response fields.

### 3. Client Updater Flow

- Add optional updater path in client daemon:
  1. Receive update directive from server.
  2. Download tarball to temp directory.
  3. Verify SHA-256.
  4. Extract and validate expected binary name.
  5. Atomically replace installed client binary.
  6. Restart service safely (Ubuntu/systemd first-class path).
- If running under `systemd`, prefer exit/restart behavior compatible with `Restart=always`.
- Add retry throttling and persist last failure to avoid repeated tight loops.

### 4. Policy / Controls

- Add client config settings:
  - `auto_update_enabled` (recommended default: `false` initially)
  - `auto_update_check_every_seconds` (e.g. 3600)
- Add server-side kill switch setting to disable update directives globally if needed.

### 5. Visibility / Observability

- Surface update state in dashboard/API:
  - current client version (already present)
  - update available (yes/no)
  - last update attempt
  - last update error/success

## Security Notes

- SHA-256 verification is required for integrity checks.
- If a client uses `insecure_skip_tls=true`, update trust is weakened.
- V2 hardening should add signed manifests/artifacts so integrity does not rely only on transport.

## Rollout Strategy

1. Implement V1 for Linux/systemd (Ubuntu primary target).
2. Gate behavior with `auto_update_enabled`.
3. Test on staging clients.
4. Enable on a small subset of clients.
5. Expand rollout after stability.

## Test Plan (V1)

- Happy path:
  - older client receives directive, updates, restarts, and checks in with new version.
- Failure paths:
  - bad checksum
  - missing tarball
  - download failure
  - permission denied replacing binary
  - restart failure
- Loop prevention:
  - ensure failed update does not retry every check-in.

## Estimated Effort

- V1 (Ubuntu/systemd, checksum verification, no rollback): **~1-2 days**
- V2 hardening (rollback + signature verification + broader init edge cases): **+2-4 days**

## Out of Scope for V1

- Cryptographic artifact signing
- Full rollback mechanism on failed startup after update
- Advanced staged rollout percentages from server UI
- Full parity across all init systems on day one
