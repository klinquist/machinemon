"""Runtime check-in loop for MachineMon Home Assistant client."""

from __future__ import annotations

import asyncio
import logging
import platform
import socket
import uuid
from datetime import datetime
from ipaddress import ip_address
from typing import Any

import psutil

from homeassistant.config_entries import ConfigEntry
from homeassistant.const import CONF_PASSWORD
from homeassistant.core import CALLBACK_TYPE, HomeAssistant, callback
from homeassistant.helpers.event import async_track_time_interval

from .api import MachineMonApiClient, MachineMonApiError, MachineMonAuthError
from .const import CONF_CLIENT_ID, DEFAULT_CHECKIN_INTERVAL

_LOGGER = logging.getLogger(__name__)


class MachineMonRuntime:
    """Background runtime that posts check-ins to MachineMon."""

    def __init__(self, hass: HomeAssistant, entry: ConfigEntry, api: MachineMonApiClient) -> None:
        self._hass = hass
        self._entry = entry
        self._api = api
        self._session_id = str(uuid.uuid4())
        self._client_id = str(entry.data[CONF_CLIENT_ID])
        self._password = str(entry.data[CONF_PASSWORD])
        self._lock = asyncio.Lock()
        self._unsub: CALLBACK_TYPE | None = None

    async def async_start(self) -> None:
        """Start periodic check-ins."""
        self._unsub = async_track_time_interval(
            self._hass, self._async_interval_tick, DEFAULT_CHECKIN_INTERVAL
        )
        self._hass.async_create_task(self._async_checkin())

    async def async_stop(self) -> None:
        """Stop periodic check-ins."""
        if self._unsub:
            self._unsub()
            self._unsub = None

    @callback
    def _async_interval_tick(self, _: datetime) -> None:
        """Schedule one check-in tick."""
        self._hass.async_create_task(self._async_checkin())

    async def _async_checkin(self) -> None:
        """Run one check-in if no check-in is currently running."""
        if self._lock.locked():
            return

        async with self._lock:
            payload = await self._hass.async_add_executor_job(
                _build_checkin_payload,
                self._client_id,
                self._session_id,
            )

            try:
                response = await self._api.async_checkin(payload, self._password)
            except MachineMonAuthError:
                _LOGGER.error("MachineMon check-in rejected: invalid client password")
                return
            except MachineMonApiError as err:
                _LOGGER.warning("MachineMon check-in failed: %s", err)
                return

            server_client_id = str(response.get("client_id") or "").strip()
            if server_client_id and server_client_id != self._client_id:
                self._client_id = server_client_id
                data = dict(self._entry.data)
                data[CONF_CLIENT_ID] = server_client_id
                self._hass.config_entries.async_update_entry(self._entry, data=data)


def _build_checkin_payload(client_id: str, session_id: str) -> dict[str, Any]:
    """Build the check-in payload for the local host running Home Assistant."""
    virtual_mem = psutil.virtual_memory()
    disk_path = "C:\\" if platform.system().lower() == "windows" else "/"
    disk_usage = psutil.disk_usage(disk_path)

    return {
        "hostname": socket.gethostname(),
        "os": platform.system().lower(),
        "arch": platform.machine().lower(),
        "client_version": "ha-integration-0.1.0",
        "client_id": client_id,
        "session_id": session_id,
        "interface_ips": _interface_ips(),
        "metrics": {
            "cpu_pct": float(psutil.cpu_percent(interval=None)),
            "mem_pct": float(virtual_mem.percent),
            "mem_total_bytes": int(virtual_mem.total),
            "mem_used_bytes": int(virtual_mem.used),
            "disk_pct": float(disk_usage.percent),
            "disk_total_bytes": int(disk_usage.total),
            "disk_used_bytes": int(disk_usage.used),
        },
        "processes": [],
        "checks": [],
    }


def _interface_ips() -> list[str]:
    """Return non-loopback IPv4/IPv6 addresses for this host."""
    addresses: set[str] = set()

    for if_addrs in psutil.net_if_addrs().values():
        for addr in if_addrs:
            raw = str(addr.address).strip()
            if not raw:
                continue

            if raw.startswith("fe80:"):
                raw = raw.split("%", 1)[0]

            try:
                parsed = ip_address(raw)
            except ValueError:
                continue

            if parsed.is_loopback:
                continue

            addresses.add(parsed.compressed)

    return sorted(addresses)
