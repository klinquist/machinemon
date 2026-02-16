"""Config flow for the MachineMon integration."""

from __future__ import annotations

import socket
import uuid
from typing import Any
from urllib.parse import urlsplit

import voluptuous as vol

from homeassistant import config_entries
from homeassistant.const import CONF_PASSWORD
from homeassistant.core import HomeAssistant
from homeassistant.exceptions import HomeAssistantError
from homeassistant.helpers.aiohttp_client import async_get_clientsession

from .api import (
    MachineMonApiClient,
    MachineMonApiError,
    MachineMonAuthError,
    MachineMonConnectionError,
)
from .const import CONF_CLIENT_ID, CONF_COLLECTION_URL, CONF_VERIFY_SSL, DOMAIN


class CannotConnect(HomeAssistantError):
    """Error to indicate we cannot connect."""


class InvalidAuth(HomeAssistantError):
    """Error to indicate auth is invalid."""


class MachineMonConfigFlow(config_entries.ConfigFlow, domain=DOMAIN):
    """Handle a config flow for MachineMon."""

    VERSION = 1

    async def async_step_user(self, user_input: dict[str, Any] | None = None):
        """Handle the initial step."""
        errors: dict[str, str] = {}

        if user_input is not None:
            data = dict(user_input)
            data[CONF_COLLECTION_URL] = _normalize_url(data[CONF_COLLECTION_URL])
            data[CONF_CLIENT_ID] = str(uuid.uuid4())

            await self.async_set_unique_id(data[CONF_COLLECTION_URL].lower())
            self._abort_if_unique_id_configured()

            try:
                await _validate_input(self.hass, data)
            except CannotConnect:
                errors["base"] = "cannot_connect"
            except InvalidAuth:
                errors["base"] = "invalid_auth"
            except Exception:
                errors["base"] = "unknown"
            else:
                return self.async_create_entry(
                    title=_entry_title(data[CONF_COLLECTION_URL]),
                    data=data,
                )

        return self.async_show_form(
            step_id="user",
            data_schema=vol.Schema(
                {
                    vol.Required(CONF_COLLECTION_URL): str,
                    vol.Required(CONF_PASSWORD): str,
                    vol.Optional(CONF_VERIFY_SSL, default=True): bool,
                }
            ),
            errors=errors,
        )


async def _validate_input(hass: HomeAssistant, data: dict[str, Any]) -> None:
    """Validate the user input allows us to connect and check in."""
    session = async_get_clientsession(hass, verify_ssl=data.get(CONF_VERIFY_SSL, True))
    client = MachineMonApiClient(session=session, collection_url=data[CONF_COLLECTION_URL])

    try:
        await client.async_health_check()
        await client.async_checkin(
            {
                "hostname": socket.gethostname(),
                "os": "homeassistant",
                "arch": "unknown",
                "client_version": "ha-integration-0.1.0",
                "client_id": data[CONF_CLIENT_ID],
                "session_id": str(uuid.uuid4()),
                "interface_ips": [],
                "metrics": {
                    "cpu_pct": 0.0,
                    "mem_pct": 0.0,
                    "mem_total_bytes": 0,
                    "mem_used_bytes": 0,
                    "disk_pct": 0.0,
                    "disk_total_bytes": 0,
                    "disk_used_bytes": 0,
                },
                "processes": [],
                "checks": [],
            },
            data[CONF_PASSWORD],
        )
    except MachineMonAuthError as err:
        raise InvalidAuth from err
    except (MachineMonConnectionError, MachineMonApiError) as err:
        raise CannotConnect from err


def _normalize_url(url: str) -> str:
    """Normalize URL for stable storage and unique ID."""
    return url.strip().rstrip("/")


def _entry_title(url: str) -> str:
    """Build a human-friendly config entry title."""
    parts = urlsplit(url)
    host = parts.hostname or url
    return f"MachineMon Client ({host})"
