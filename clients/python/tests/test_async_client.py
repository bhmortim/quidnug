"""Tests for AsyncQuidnugClient.

Skipped if httpx is not installed.
"""

from __future__ import annotations

import json
import pytest

httpx = pytest.importorskip("httpx")

from quidnug import Quid
from quidnug.async_client import AsyncQuidnugClient
from quidnug.errors import ConflictError, UnavailableError, ValidationError


def _envelope_handler(*responses):
    """Return an httpx.MockTransport that replies with queued envelopes."""
    iterator = iter(responses)

    def handler(request: httpx.Request) -> httpx.Response:
        try:
            status, payload = next(iterator)
        except StopIteration:
            return httpx.Response(500, json={"success": False, "error": {"code": "NO_REPLY"}})
        return httpx.Response(status, json=payload)

    return httpx.MockTransport(handler)


@pytest.mark.asyncio
async def test_health_success_unwraps_envelope():
    transport = _envelope_handler(
        (200, {"success": True, "data": {"status": "ok"}}),
    )
    async with httpx.AsyncClient(transport=transport) as http:
        client = AsyncQuidnugClient("http://node.local", client=http, max_retries=0)
        data = await client.health()
        assert data == {"status": "ok"}


@pytest.mark.asyncio
async def test_grant_trust_posts_and_signs():
    tx_bodies = []

    def handler(request: httpx.Request) -> httpx.Response:
        tx_bodies.append(json.loads(request.content.decode()))
        return httpx.Response(200, json={"success": True, "data": {"txId": "abc"}})

    transport = httpx.MockTransport(handler)
    async with httpx.AsyncClient(transport=transport) as http:
        client = AsyncQuidnugClient("http://node.local", client=http, max_retries=0)
        alice = Quid.generate()
        result = await client.grant_trust(alice, trustee="bob", level=0.9)
        assert result == {"txId": "abc"}
        assert tx_bodies[0]["type"] == "TRUST"
        assert tx_bodies[0]["trustee"] == "bob"
        assert tx_bodies[0]["trustLevel"] == 0.9
        assert "signature" in tx_bodies[0]


@pytest.mark.asyncio
async def test_conflict_raises_conflict_error():
    transport = _envelope_handler(
        (409, {"success": False, "error": {"code": "NONCE_REPLAY", "message": "replay"}}),
    )
    async with httpx.AsyncClient(transport=transport) as http:
        client = AsyncQuidnugClient("http://node.local", client=http, max_retries=0)
        alice = Quid.generate()
        with pytest.raises(ConflictError):
            await client.grant_trust(alice, trustee="bob", level=0.5)


@pytest.mark.asyncio
async def test_service_unavailable_raises_unavailable():
    transport = _envelope_handler(
        (503, {"success": False, "error": {"code": "BOOTSTRAPPING", "message": "warm"}}),
    )
    async with httpx.AsyncClient(transport=transport) as http:
        client = AsyncQuidnugClient("http://node.local", client=http, max_retries=0)
        with pytest.raises(UnavailableError):
            await client.health()


@pytest.mark.asyncio
async def test_not_found_returns_none_for_get_identity():
    transport = _envelope_handler(
        (404, {"success": False, "error": {"code": "NOT_FOUND", "message": "absent"}}),
    )
    async with httpx.AsyncClient(transport=transport) as http:
        client = AsyncQuidnugClient("http://node.local", client=http, max_retries=0)
        result = await client.get_identity("abc1234")
        assert result is None


@pytest.mark.asyncio
async def test_level_validation_happens_pre_network():
    async with httpx.AsyncClient() as http:
        client = AsyncQuidnugClient("http://node.local", client=http, max_retries=0)
        alice = Quid.generate()
        with pytest.raises(ValidationError):
            await client.grant_trust(alice, trustee="bob", level=1.5)


@pytest.mark.asyncio
async def test_context_manager_closes_owned_client():
    """When the caller doesn't pass a client, the context manager owns it."""
    async with AsyncQuidnugClient("http://node.local") as client:
        assert client._client is not None
    # After exit, the owned client should be closed.
    assert client._client is None
