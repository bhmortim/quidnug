"""Async Quidnug client using httpx — mirror of QuidnugClient.

Modern Python shops are async-first (FastAPI, aiohttp, Sanic,
Litestar). This module provides an asyncio-compatible client with
the exact same method names and return types as the synchronous
``QuidnugClient``, plus ``async``/``await`` semantics.

Install::

    pip install 'quidnug[async]'

(or install ``httpx`` yourself if you manage dependencies directly.)

Example::

    from quidnug import Quid
    from quidnug.async_client import AsyncQuidnugClient

    async def main():
        async with AsyncQuidnugClient("http://localhost:8080") as client:
            alice = Quid.generate()
            bob = Quid.generate()
            await client.register_identity(alice, name="Alice")
            await client.register_identity(bob, name="Bob")
            await client.grant_trust(alice, trustee=bob.id, level=0.9)
            tr = await client.get_trust(alice.id, bob.id)
            print(tr.trust_level)

    asyncio.run(main())

The async client is intentionally a thin wrapper — it reuses every
validator, type, and decoder from the sync path, replacing only the
HTTP layer with ``httpx.AsyncClient``.
"""

from __future__ import annotations

import asyncio
import json
import random
from dataclasses import asdict
from typing import Any, Dict, List, Optional, Tuple
from urllib.parse import quote, urljoin

try:
    import httpx
except ImportError as exc:  # pragma: no cover - import-time guard
    raise ImportError(
        "AsyncQuidnugClient requires httpx. Install with "
        "`pip install 'quidnug[async]'` or `pip install httpx`."
    ) from exc

from quidnug.client import (
    _CONFLICT_CODES,
    _UNAVAILABLE_CODES,
    _dc,
    _domain_fingerprint_from_wire,
    _event_from_wire,
    _guardian_set_from_wire,
    _identity_from_wire,
    _nonce_snapshot_from_wire,
    _strip_none,
    _title_from_wire,
    _trust_edge_from_wire,
    _trust_result_from_wire,
)
from quidnug.crypto import Quid, canonical_bytes
from quidnug.wire import (
    EventTx as _EventTx,
    IdentityTx as _IdentityTx,
    OwnershipStake as _WireOwnershipStake,
    TitleTx as _TitleTx,
    TrustTx as _TrustTx,
    sign_wire as _sign_wire,
)
from quidnug.errors import (
    ConflictError,
    NodeError,
    UnavailableError,
    ValidationError,
)
from quidnug.types import (
    AnchorGossipMessage,
    DomainFingerprint,
    Event,
    ForkBlock,
    GuardianRecoveryCommit,
    GuardianRecoveryInit,
    GuardianRecoveryVeto,
    GuardianResignation,
    GuardianSet,
    GuardianSetUpdate,
    IdentityRecord,
    NodeAdvertisementParams,
    NonceSnapshot,
    OwnershipStake,
    Title,
    TrustEdge,
    TrustResult,
)


class AsyncQuidnugClient:
    """Async counterpart to ``QuidnugClient``.

    All methods accept the same keyword arguments and return the
    same types as their sync siblings. The only difference: you
    must ``await`` them, and the client is a context manager
    (``async with AsyncQuidnugClient(...) as c:``).
    """

    def __init__(
        self,
        base_url: str,
        *,
        timeout: float = 30.0,
        max_retries: int = 3,
        retry_base_delay: float = 1.0,
        client: Optional[httpx.AsyncClient] = None,
        auth_header: Optional[str] = None,
    ) -> None:
        if not base_url:
            raise ValidationError("base_url is required")
        self.base_url = base_url.rstrip("/")
        self.api_base = self.base_url + "/api"
        self.timeout = timeout
        self.max_retries = max_retries
        self.retry_base_delay = retry_base_delay
        self._auth_header = auth_header
        self._client = client
        self._owns_client = client is None

    async def __aenter__(self) -> "AsyncQuidnugClient":
        if self._client is None:
            self._client = httpx.AsyncClient(timeout=self.timeout)
        return self

    async def __aexit__(self, exc_type, exc, tb) -> None:
        if self._owns_client and self._client is not None:
            await self._client.aclose()
            self._client = None

    # --- HTTP plumbing ---------------------------------------------------

    async def _request(
        self,
        method: str,
        path: str,
        *,
        params: Optional[Dict[str, Any]] = None,
        body: Any = None,
        retry: Optional[bool] = None,
    ) -> Dict[str, Any]:
        if self._client is None:
            self._client = httpx.AsyncClient(timeout=self.timeout)
            self._owns_client = True

        url = urljoin(self.api_base + "/", path.lstrip("/"))
        if retry is None:
            retry = method.upper() == "GET"

        headers = {"Accept": "application/json"}
        if body is not None:
            headers["Content-Type"] = "application/json"
        if self._auth_header:
            headers["Authorization"] = f"Bearer {self._auth_header}"

        data: Optional[str] = None
        if body is not None:
            data = json.dumps(body, default=_json_fallback)

        attempts = (self.max_retries + 1) if retry else 1
        last_err: Optional[Exception] = None
        for attempt in range(attempts):
            try:
                resp = await self._client.request(
                    method=method,
                    url=url,
                    params=params,
                    content=data,
                    headers=headers,
                )
            except httpx.HTTPError as exc:
                last_err = exc
                if attempt < attempts - 1:
                    await self._sleep_backoff(attempt, None)
                    continue
                raise NodeError(f"Network error on {method} {path}: {exc}") from exc

            if resp.status_code >= 500 or resp.status_code == 429:
                if attempt < attempts - 1:
                    await self._sleep_backoff(attempt, resp.headers.get("Retry-After"))
                    continue

            return self._parse_envelope(resp)

        raise last_err or NodeError(f"Retries exhausted on {method} {path}")

    async def _sleep_backoff(self, attempt: int, retry_after: Optional[str]) -> None:
        if retry_after:
            try:
                delay = float(retry_after)
                await asyncio.sleep(min(delay, 60.0))
                return
            except ValueError:
                pass
        delay = self.retry_base_delay * (2**attempt) + random.random() * 0.1
        await asyncio.sleep(min(delay, 60.0))

    def _parse_envelope(self, resp: httpx.Response) -> Dict[str, Any]:
        try:
            payload = resp.json()
        except ValueError as exc:
            raise NodeError(
                f"Non-JSON response (HTTP {resp.status_code}): {resp.text[:200]}",
                status_code=resp.status_code,
                response_body=resp.text,
            ) from exc

        if not isinstance(payload, dict):
            raise NodeError(
                "API response is not a JSON object",
                status_code=resp.status_code,
                response_body=resp.text,
            )

        if payload.get("success") is True:
            data = payload.get("data")
            return data if isinstance(data, dict) else {"value": data}

        err = payload.get("error") or {}
        code = err.get("code") or "UNKNOWN_ERROR"
        message = err.get("message") or f"HTTP {resp.status_code}"
        details = err.get("details") if isinstance(err.get("details"), dict) else None

        if resp.status_code == 503 or code in _UNAVAILABLE_CODES:
            raise UnavailableError(message, details=details or {"code": code})
        if code in _CONFLICT_CODES or resp.status_code == 409:
            raise ConflictError(message, details=details or {"code": code})
        if 400 <= resp.status_code < 500:
            raise ValidationError(message, details=details or {"code": code})
        raise NodeError(message, status_code=resp.status_code, response_body=resp.text)

    # --- Health ----------------------------------------------------------

    async def health(self) -> Dict[str, Any]:
        return await self._request("GET", "health")

    async def info(self) -> Dict[str, Any]:
        return await self._request("GET", "info")

    async def nodes(self, *, limit: Optional[int] = None, offset: Optional[int] = None) -> Dict[str, Any]:
        return await self._request("GET", "nodes",
                                   params=_strip_none({"limit": limit, "offset": offset}))

    # --- Identity --------------------------------------------------------

    async def register_identity(
        self,
        signer: Quid,
        *,
        subject_quid: Optional[str] = None,
        domain: str = "default",
        name: Optional[str] = None,
        description: Optional[str] = None,
        attributes: Optional[Dict[str, Any]] = None,
        home_domain: Optional[str] = None,
        update_nonce: int = 1,
    ) -> Dict[str, Any]:
        if not signer.has_private_key:
            raise ValidationError("signer must have a private key")
        subject = subject_quid or signer.id
        import time
        tx = _IdentityTx(
            trust_domain=domain,
            timestamp=int(time.time()),
            public_key=signer.public_key_hex,
            quid_id=subject,
            name=name or "",
            description=description or "",
            attributes=attributes,
            creator=signer.id,
            update_nonce=update_nonce,
            home_domain=home_domain or "",
        )
        wire = _sign_wire(tx, signer)
        return await self._request("POST", "transactions/identity", body=wire)

    async def get_identity(self, quid_id: str, *, domain: Optional[str] = None) -> Optional[IdentityRecord]:
        if not quid_id:
            raise ValidationError("quid_id is required")
        try:
            data = await self._request(
                "GET",
                f"identity/{quote(quid_id, safe='')}",
                params=_strip_none({"domain": domain}),
            )
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _identity_from_wire(data)

    # --- Trust -----------------------------------------------------------

    async def grant_trust(
        self,
        signer: Quid,
        *,
        trustee: str,
        level: float,
        domain: str = "default",
        nonce: int = 1,
        valid_until: Optional[int] = None,
        description: Optional[str] = None,
    ) -> Dict[str, Any]:
        if not signer.has_private_key:
            raise ValidationError("signer must have a private key")
        if not trustee:
            raise ValidationError("trustee is required")
        if not 0.0 <= level <= 1.0:
            raise ValidationError("level must be in [0.0, 1.0]")
        import time
        tx = _TrustTx(
            trust_domain=domain,
            timestamp=int(time.time()),
            public_key=signer.public_key_hex,
            truster=signer.id,
            trustee=trustee,
            trust_level=level,
            nonce=nonce,
            description=description or "",
            valid_until=valid_until or 0,
        )
        wire = _sign_wire(tx, signer)
        return await self._request("POST", "transactions/trust", body=wire)

    async def get_trust(
        self,
        observer: str,
        target: str,
        *,
        domain: str = "default",
        max_depth: Optional[int] = None,
    ) -> TrustResult:
        if not observer or not target:
            raise ValidationError("observer and target are required")
        params = _strip_none({"domain": domain, "maxDepth": max_depth})
        path = f"trust/{quote(observer, safe='')}/{quote(target, safe='')}"
        data = await self._request("GET", path, params=params)
        return _trust_result_from_wire(observer, target, domain, data)

    async def get_trust_edges(self, quid_id: str) -> List[TrustEdge]:
        data = await self._request("GET", f"trust/edges/{quote(quid_id, safe='')}")
        raw = data.get("edges") or data.get("data") or []
        if not isinstance(raw, list):
            return []
        return [_trust_edge_from_wire(e) for e in raw]

    # --- Title -----------------------------------------------------------

    async def register_title(
        self,
        signer: Quid,
        *,
        asset_id: str,
        owners: List[OwnershipStake],
        domain: str = "default",
        title_type: Optional[str] = None,
        prev_title_tx_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        if not signer.has_private_key:
            raise ValidationError("signer must have a private key")
        if not asset_id:
            raise ValidationError("asset_id is required")
        if not owners:
            raise ValidationError("owners is required")
        total = sum(s.percentage for s in owners)
        if abs(total - 1.0) < 0.001:
            norm = 1.0
        elif abs(total - 100.0) < 0.001:
            norm = 0.01
        else:
            raise ValidationError(
                f"ownership percentages must sum to 1.0 (or 100.0 for percent); got {total}"
            )
        import time
        wire_owners = [
            _WireOwnershipStake(
                owner_id=s.owner_id,
                percentage=s.percentage * norm,
                stake_type=getattr(s, "stake_type", "") or "",
            )
            for s in owners
        ]
        _ = prev_title_tx_id  # not in v1.0 wire schema
        tx = _TitleTx(
            trust_domain=domain,
            timestamp=int(time.time()),
            public_key=signer.public_key_hex,
            asset_id=asset_id,
            owners=wire_owners,
            signatures={},
            title_type=title_type or "",
        )
        wire = _sign_wire(tx, signer)
        return await self._request("POST", "transactions/title", body=wire)

    async def get_title(self, asset_id: str, *, domain: Optional[str] = None) -> Optional[Title]:
        try:
            data = await self._request(
                "GET",
                f"title/{quote(asset_id, safe='')}",
                params=_strip_none({"domain": domain}),
            )
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _title_from_wire(data)

    # --- Events ----------------------------------------------------------

    async def emit_event(
        self,
        signer: Quid,
        *,
        subject_id: str,
        subject_type: str,
        event_type: str,
        domain: str = "default",
        payload: Optional[Dict[str, Any]] = None,
        payload_cid: Optional[str] = None,
        sequence: Optional[int] = None,
    ) -> Dict[str, Any]:
        if not signer.has_private_key:
            raise ValidationError("signer must have a private key")
        if subject_type not in ("QUID", "TITLE"):
            raise ValidationError("subject_type must be 'QUID' or 'TITLE'")
        if not event_type:
            raise ValidationError("event_type is required")
        if (payload is None) == (payload_cid is None):
            raise ValidationError("exactly one of payload or payload_cid is required")

        if sequence is None:
            try:
                stream = await self.get_event_stream(subject_id, domain=domain)
                sequence = (stream.get("latestSequence", 0) + 1) if stream else 1
            except Exception:
                sequence = 1

        import time
        tx = _EventTx(
            trust_domain=domain,
            timestamp=int(time.time()),
            public_key=signer.public_key_hex,
            subject_id=subject_id,
            subject_type=subject_type,
            sequence=sequence,
            event_type=event_type,
            payload=payload,
            payload_cid=payload_cid or "",
        )
        wire = _sign_wire(tx, signer)
        return await self._request("POST", "events", body=wire)

    async def get_event_stream(self, subject_id: str, *, domain: Optional[str] = None) -> Optional[Dict[str, Any]]:
        try:
            return await self._request(
                "GET",
                f"streams/{quote(subject_id, safe='')}",
                params=_strip_none({"domain": domain}),
            )
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise

    async def get_stream_events(
        self,
        subject_id: str,
        *,
        domain: Optional[str] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
    ) -> Tuple[List[Event], Dict[str, Any]]:
        data = await self._request(
            "GET",
            f"streams/{quote(subject_id, safe='')}/events",
            params=_strip_none({"domain": domain, "limit": limit, "offset": offset}),
        )
        raw = data.get("data") or data.get("events") or []
        events = [_event_from_wire(e) for e in raw] if isinstance(raw, list) else []
        pagination = data.get("pagination") or {}
        return events, pagination

    # --- Guardians / gossip / bootstrap / fork-block (short form) --------
    #
    # The remaining endpoints mirror the sync client. They all go through
    # ``self._request`` so behavior is identical; we just delegate.

    async def submit_guardian_set_update(self, update: GuardianSetUpdate) -> Dict[str, Any]:
        return await self._request("POST", "guardian/set-update", body=_dc(update))

    async def submit_recovery_init(self, init: GuardianRecoveryInit) -> Dict[str, Any]:
        return await self._request("POST", "guardian/recovery/init", body=_dc(init))

    async def submit_recovery_veto(self, veto: GuardianRecoveryVeto) -> Dict[str, Any]:
        return await self._request("POST", "guardian/recovery/veto", body=_dc(veto))

    async def submit_recovery_commit(self, commit: GuardianRecoveryCommit) -> Dict[str, Any]:
        return await self._request("POST", "guardian/recovery/commit", body=_dc(commit))

    async def submit_guardian_resignation(self, resignation: GuardianResignation) -> Dict[str, Any]:
        return await self._request("POST", "guardian/resign", body=_dc(resignation))

    async def get_guardian_set(self, quid: str) -> Optional[GuardianSet]:
        try:
            data = await self._request("GET", f"guardian/set/{quote(quid, safe='')}")
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _guardian_set_from_wire(data)

    async def get_pending_recovery(self, quid: str) -> Optional[Dict[str, Any]]:
        try:
            return await self._request(
                "GET", f"guardian/pending-recovery/{quote(quid, safe='')}",
            )
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise

    async def get_guardian_resignations(self, quid: str) -> List[Dict[str, Any]]:
        data = await self._request(
            "GET", f"guardian/resignations/{quote(quid, safe='')}",
        )
        raw = data.get("data") or data.get("resignations") or []
        return raw if isinstance(raw, list) else []

    # --- Cross-domain gossip + fingerprints ------------------------------

    async def submit_domain_fingerprint(self, fp: DomainFingerprint) -> Dict[str, Any]:
        return await self._request("POST", "domain-fingerprints", body=_dc(fp))

    async def submit_anchor_gossip(self, message: AnchorGossipMessage) -> Dict[str, Any]:
        return await self._request("POST", "anchor-gossip", body=_dc(message))

    async def push_anchor(self, message: AnchorGossipMessage) -> Dict[str, Any]:
        return await self._request("POST", "gossip/push-anchor", body=_dc(message))

    async def push_fingerprint(self, fp: DomainFingerprint) -> Dict[str, Any]:
        return await self._request("POST", "gossip/push-fingerprint", body=_dc(fp))

    async def get_latest_domain_fingerprint(self, domain: str) -> Optional[DomainFingerprint]:
        try:
            data = await self._request("GET", f"domain-fingerprints/{quote(domain, safe='')}/latest")
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _domain_fingerprint_from_wire(data)

    # --- Bootstrap + nonce snapshots -------------------------------------

    async def submit_nonce_snapshot(self, snapshot: NonceSnapshot) -> Dict[str, Any]:
        return await self._request("POST", "nonce-snapshots", body=_dc(snapshot))

    async def get_latest_nonce_snapshot(self, domain: str) -> Optional[NonceSnapshot]:
        try:
            data = await self._request(
                "GET", f"nonce-snapshots/{quote(domain, safe='')}/latest",
            )
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _nonce_snapshot_from_wire(data)

    async def bootstrap_status(self) -> Dict[str, Any]:
        return await self._request("GET", "bootstrap/status")

    # --- Fork-block ------------------------------------------------------

    async def submit_fork_block(self, fb: ForkBlock) -> Dict[str, Any]:
        return await self._request("POST", "fork-block", body=_dc(fb))

    async def fork_block_status(self) -> Dict[str, Any]:
        return await self._request("GET", "fork-block/status")

    # --- Blocks + pending transactions -----------------------------------

    async def get_blocks(
        self, *, limit: Optional[int] = None, offset: Optional[int] = None,
    ) -> Dict[str, Any]:
        return await self._request(
            "GET", "blocks", params=_strip_none({"limit": limit, "offset": offset}),
        )

    async def get_pending_transactions(
        self, *, limit: Optional[int] = None, offset: Optional[int] = None,
    ) -> Dict[str, Any]:
        return await self._request(
            "GET", "transactions", params=_strip_none({"limit": limit, "offset": offset}),
        )

    # --- Domains ---------------------------------------------------------

    async def list_domains(self) -> Dict[str, Any]:
        return await self._request("GET", "domains")

    async def register_domain(self, domain: str, **attrs: Any) -> Dict[str, Any]:
        body = {"name": domain, **attrs}
        return await self._request("POST", "domains", body=body)

    async def ensure_domain(self, domain: str, **attrs: Any) -> Dict[str, Any]:
        """Register the domain if it doesn't already exist. Idempotent."""
        try:
            return await self.register_domain(domain, **attrs)
        except ValidationError as e:
            if "already exists" in str(e).lower():
                return {
                    "status": "success",
                    "domain": domain,
                    "message": "trust domain already exists",
                }
            raise

    # --- Wait helpers ----------------------------------------------------

    async def wait_for_identity(
        self,
        quid_id: str,
        *,
        domain: Optional[str] = None,
        timeout: float = 30.0,
        poll: float = 0.5,
    ) -> IdentityRecord:
        """Block until ``quid_id`` is visible in the committed
        identity registry, or raise ``TimeoutError``.

        Identity transactions live in the pending pool until the
        next block is sealed; code that immediately emits events
        referencing the new quid must wait for commit first.
        """
        import time as _time

        deadline = _time.monotonic() + timeout
        while _time.monotonic() < deadline:
            rec = await self.get_identity(quid_id, domain=domain)
            if rec is not None:
                return rec
            await asyncio.sleep(poll)
        raise TimeoutError(
            f"identity {quid_id} did not commit within {timeout}s",
        )

    async def wait_for_identities(
        self,
        quid_ids: List[str],
        *,
        domain: Optional[str] = None,
        timeout: float = 30.0,
        poll: float = 0.5,
    ) -> None:
        """Block until every quid_id in the list is committed.
        Shares a single deadline across all ids."""
        import time as _time

        deadline = _time.monotonic() + timeout
        for qid in quid_ids:
            remaining = max(0.0, deadline - _time.monotonic())
            if remaining <= 0:
                raise TimeoutError(
                    f"identities not all committed within {timeout}s "
                    f"(blocked on {qid})",
                )
            await self.wait_for_identity(
                qid, domain=domain, timeout=remaining, poll=poll,
            )

    async def wait_for_title(
        self,
        asset_id: str,
        *,
        domain: Optional[str] = None,
        timeout: float = 30.0,
        poll: float = 0.5,
    ) -> Title:
        """Block until ``asset_id`` is visible in the committed
        title registry, or raise ``TimeoutError``."""
        import time as _time

        deadline = _time.monotonic() + timeout
        while _time.monotonic() < deadline:
            rec = await self.get_title(asset_id, domain=domain)
            if rec is not None:
                return rec
            await asyncio.sleep(poll)
        raise TimeoutError(
            f"title {asset_id} did not commit within {timeout}s",
        )

    # --- Raw passthrough -------------------------------------------------

    async def raw_get(self, path: str) -> bytes:
        """GET ``path`` (without ``/api`` prefix; the client adds it)
        and return the raw response body bytes. Mirrors the sync
        ``QuidnugClient.raw_get``."""
        if self._client is None:
            self._client = httpx.AsyncClient(timeout=self.timeout)
            self._owns_client = True
        url = urljoin(self.api_base + "/", path.lstrip("/"))
        headers = {"Accept": "application/json"}
        if self._auth_header:
            headers["Authorization"] = f"Bearer {self._auth_header}"
        resp = await self._client.get(url, headers=headers)
        body = resp.content
        if resp.status_code < 200 or resp.status_code >= 300:
            raise NodeError(
                f"raw_get {path}: HTTP {resp.status_code}",
                status_code=resp.status_code,
                response_body=resp.text,
            )
        return body

    # --- QDP-0014: Node advertisement + discovery ------------------------

    async def publish_node_advertisement(
        self, signer: Quid, params: NodeAdvertisementParams,
    ) -> Dict[str, Any]:
        """POST /api/node-advertisements — async counterpart to
        ``QuidnugClient.publish_node_advertisement``.

        .. note::

           Stub: requires the QDP-0014 wire dataclass + signing path
           to land in ``quidnug.wire``. See the sync method's docstring.
        """
        # TODO(QDP-0014): mirror sync implementation once
        # quidnug.wire.NodeAdvertisementTx lands.
        raise NotImplementedError(
            "publish_node_advertisement is not yet implemented in the "
            "Python SDK; requires the QDP-0014 wire struct + signing "
            "path to land in quidnug.wire. Track parity with the Go "
            "SDK's Client.PublishNodeAdvertisement."
        )

    async def discover_domain(self, domain: str) -> Dict[str, Any]:
        """GET /api/v2/discovery/domain/{domain}."""
        if not domain:
            raise ValidationError("domain is required")
        return await self._request(
            "GET", f"v2/discovery/domain/{quote(domain, safe='')}",
        )

    async def discover_node(self, quid: str) -> Dict[str, Any]:
        """GET /api/v2/discovery/node/{quid}."""
        if not quid:
            raise ValidationError("quid is required")
        return await self._request(
            "GET", f"v2/discovery/node/{quote(quid, safe='')}",
        )

    async def discover_operator(self, operator_quid: str) -> Dict[str, Any]:
        """GET /api/v2/discovery/operator/{operator_quid}."""
        if not operator_quid:
            raise ValidationError("operator_quid is required")
        return await self._request(
            "GET", f"v2/discovery/operator/{quote(operator_quid, safe='')}",
        )

    async def discover_quids(
        self,
        *,
        domain: str,
        since: Optional[int] = None,
        sort: Optional[str] = None,
        observer: Optional[str] = None,
        event_type: Optional[str] = None,
        min_trust_weight: Optional[float] = None,
        exclude_quids: Optional[List[str]] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
    ) -> Dict[str, Any]:
        """GET /api/v2/discovery/quids — per-domain quid index."""
        if not domain:
            raise ValidationError("domain is required")
        params: Dict[str, Any] = {"domain": domain}
        if since is not None and since > 0:
            params["since"] = since
        if sort:
            params["sort"] = sort
        if observer:
            params["observer"] = observer
        if event_type:
            params["eventType"] = event_type
        if min_trust_weight is not None and min_trust_weight > 0:
            params["min-trust-weight"] = min_trust_weight
        if exclude_quids:
            params["excludeQuid"] = ",".join(exclude_quids)
        if limit is not None and limit > 0:
            params["limit"] = limit
        if offset is not None and offset > 0:
            params["offset"] = offset
        return await self._request("GET", "v2/discovery/quids", params=params)

    async def discover_trusted_quids(
        self,
        domain: str,
        *,
        min_trust: Optional[float] = None,
        limit: Optional[int] = None,
    ) -> Dict[str, Any]:
        """GET /api/v2/discovery/trusted-quids."""
        if not domain:
            raise ValidationError("domain is required")
        params: Dict[str, Any] = {"domain": domain}
        if min_trust is not None and min_trust > 0:
            params["min-trust"] = min_trust
        if limit is not None and limit > 0:
            params["limit"] = limit
        return await self._request(
            "GET", "v2/discovery/trusted-quids", params=params,
        )


def _json_fallback(obj: Any) -> Any:
    if hasattr(obj, "__dataclass_fields__"):
        return asdict(obj)
    if isinstance(obj, bytes):
        return obj.hex()
    raise TypeError(f"Type {type(obj).__name__} not JSON serializable")


__all__ = ["AsyncQuidnugClient"]
