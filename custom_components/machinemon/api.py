"""API client for MachineMon check-ins."""

from __future__ import annotations

import asyncio
from typing import Any

import aiohttp


class MachineMonApiError(Exception):
    """Base exception for API errors."""


class MachineMonAuthError(MachineMonApiError):
    """Raised when API auth fails."""


class MachineMonConnectionError(MachineMonApiError):
    """Raised when API is unreachable."""


class MachineMonApiClient:
    """Small API wrapper around MachineMon endpoints used by the HA client."""

    def __init__(self, session: aiohttp.ClientSession, collection_url: str) -> None:
        self._session = session
        self._base_url = collection_url.rstrip("/")

    async def async_health_check(self) -> None:
        """Check basic server connectivity."""
        await self._request_json("GET", "/healthz")

    async def async_checkin(self, payload: dict[str, Any], client_password: str) -> dict[str, Any]:
        """Send a check-in payload to MachineMon."""
        return await self._request_json(
            "POST",
            "/api/v1/checkin",
            headers={"X-Client-Password": client_password},
            json_body=payload,
        )

    async def _request_json(
        self,
        method: str,
        path: str,
        *,
        headers: dict[str, str] | None = None,
        json_body: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Perform a JSON request."""
        request_headers = {"Accept": "application/json"}
        if headers:
            request_headers.update(headers)

        try:
            async with self._session.request(
                method,
                f"{self._base_url}{path}",
                headers=request_headers,
                json=json_body,
                timeout=aiohttp.ClientTimeout(total=15),
            ) as response:
                if response.status == 401:
                    raise MachineMonAuthError("Invalid client password")
                if response.status >= 400:
                    body = await response.text()
                    raise MachineMonApiError(
                        f"HTTP {response.status} from MachineMon API: {body[:200]}"
                    )

                payload = await response.json(content_type=None)
                if not isinstance(payload, dict):
                    raise MachineMonApiError("Invalid response: expected JSON object")
                return payload

        except MachineMonApiError:
            raise
        except (aiohttp.ClientError, asyncio.TimeoutError) as err:
            raise MachineMonConnectionError(str(err)) from err
