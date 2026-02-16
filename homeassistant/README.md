# MachineMon Home Assistant Client (HACS)

This integration is a **MachineMon client running inside Home Assistant**.

It does one job:

- sends CPU, memory, and disk check-ins from the Home Assistant host to your MachineMon server every 120 seconds

It does **not** create Home Assistant sensors/entities.  
It is only a client heartbeat/metrics reporter so your HA host appears in MachineMon.

## Requirements

- A reachable MachineMon server
- MachineMon **client password** (not admin password)
- [HACS](https://hacs.xyz/) installed in Home Assistant

## Install Through HACS (Custom Repository)

1. Open Home Assistant.
2. Go to **HACS**.
3. Open the HACS menu (top-right 3 dots), then click **Custom repositories**.
4. Add repository URL:
   - `https://github.com/klinquist/machinemon`
5. Choose category:
   - `Integration`
6. Click **Add**.
7. Search HACS for **MachineMon** and install it.
8. Restart Home Assistant.

## Configure URL + Password

1. Go to **Settings -> Devices & Services**.
2. Click **Add Integration**.
3. Search for **MachineMon**.
4. Fill in:
   - **Collection URL**: MachineMon server base URL
   - **Client Password**: password configured on your MachineMon server for clients
   - **Verify SSL certificate**: leave enabled unless you intentionally use self-signed TLS

### Collection URL Examples

- `https://monitor.example.com`
- `http://192.168.1.50:8080`
- `https://monitor.example.com/machinemon` (if you configured `base_path`)

Use the same base URL clients should use to reach MachineMon.

## What Happens After Setup

- Integration validates it can connect to your server and authenticate with the client password.
- Home Assistant starts background check-ins to:
  - `POST /api/v1/checkin`
- MachineMon will show Home Assistant as a normal client.

## Troubleshooting

### Invalid client password

- Confirm you used the MachineMon **client** password.
- Do not use the MachineMon admin/dashboard password here.

### Cannot connect

- Confirm Home Assistant can route to the URL and port.
- Confirm `http` vs `https` is correct.
- If reverse proxy/subpath is used, include the full base path in Collection URL.

### SSL/certificate failures

- Disable **Verify SSL certificate** only when using self-signed certs.
- Prefer valid certs for normal deployments.

## Local Testing Without HACS

Copy `custom_components/machinemon` into your Home Assistant config folder under `custom_components/` and restart Home Assistant.
