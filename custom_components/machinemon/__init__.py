"""The MachineMon integration."""

from __future__ import annotations

from homeassistant.config_entries import ConfigEntry
from homeassistant.core import HomeAssistant
from homeassistant.helpers.aiohttp_client import async_get_clientsession

from .api import MachineMonApiClient
from .const import CONF_COLLECTION_URL, CONF_VERIFY_SSL, DOMAIN, RUNTIME
from .runtime import MachineMonRuntime


async def async_setup_entry(hass: HomeAssistant, entry: ConfigEntry) -> bool:
    """Set up MachineMon from a config entry."""
    session = async_get_clientsession(hass, verify_ssl=entry.data.get(CONF_VERIFY_SSL, True))
    api = MachineMonApiClient(
        session=session,
        collection_url=entry.data[CONF_COLLECTION_URL],
    )
    runtime = MachineMonRuntime(hass, entry, api)

    hass.data.setdefault(DOMAIN, {})
    hass.data[DOMAIN][entry.entry_id] = {RUNTIME: runtime}

    await runtime.async_start()
    return True


async def async_unload_entry(hass: HomeAssistant, entry: ConfigEntry) -> bool:
    """Unload a config entry."""
    runtime: MachineMonRuntime = hass.data[DOMAIN][entry.entry_id][RUNTIME]
    await runtime.async_stop()

    hass.data[DOMAIN].pop(entry.entry_id)
    if not hass.data[DOMAIN]:
        hass.data.pop(DOMAIN)
    return True
