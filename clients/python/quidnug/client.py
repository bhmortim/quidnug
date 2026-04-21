"""HTTP client covering the full Quidnug protocol surface.

One class: ``QuidnugClient``. All methods map one-for-one to a node's
JSON API (defined in ``internal/core/handlers*.go`` of the Go reference
implementation). Responses are returned as typed dataclasses where the
server emits a known type; otherwise as the raw JSON-decoded dict so
the SDK never silently drops fields from protocol-level upgrades.

Error handling
--------------
All SDK-raised errors derive from ``QuidnugError`` (see ``errors.py``).

- ``ValidationError`` — local precondition failures (missing key,
  percentages not summing to 100, subject_type not in {QUID,TITLE}).
- ``ConflictError`` — 409 and select 400s carrying ``error.code`` of
  NONCE_REPLAY / GUARDIAN_SET_MISMATCH / QUORUM_NOT_MET.
- ``UnavailableError`` — 503 and feature-gated activation errors.
- ``NodeError`` — everything else: transport, 5xx, unexpected shape.

The client retries idempotent GETs and transient 5xx/429 with
exponential backoff + jitter. Non-idempotent writes are sent once.
"""

from __future__ import annotations

import json
import logging
import random
import time
from dataclasses import asdict
from typing import Any, Dict, List, Optional, Tuple, Union
from urllib.parse import quote, urljoin

import requests

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
    QuidnugError,
    UnavailableError,
    ValidationError,
)
from quidnug.types import (
    Anchor,
    AnchorGossipMessage,
    Block,
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
    NonceSnapshot,
    OwnershipStake,
    Title,
    TrustEdge,
    TrustResult,
)

_LOG = logging.getLogger(__name__)

# Server-side error codes that should raise ConflictError rather than
# generic NodeError. Sourced from the Go error taxonomy.
_CONFLICT_CODES = frozenset(
    {
        "NONCE_REPLAY",
        "GUARDIAN_SET_MISMATCH",
        "QUORUM_NOT_MET",
        "VETOED",
        "INVALID_SIGNATURE",
        "FORK_ALREADY_ACTIVE",
        "DUPLICATE",
        "ALREADY_EXISTS",
        "INVALID_STATE_TRANSITION",
    }
)
_UNAVAILABLE_CODES = frozenset({"FEATURE_NOT_ACTIVE", "NOT_READY", "BOOTSTRAPPING"})


# --- Helpers ---------------------------------------------------------------


def _dc(obj: Any) -> Any:
    """Recursively convert dataclasses and enums to plain dicts."""
    if hasattr(obj, "__dataclass_fields__"):
        return {k: _dc(v) for k, v in asdict(obj).items()}
    if isinstance(obj, dict):
        return {k: _dc(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [_dc(v) for v in obj]
    return obj


def _strip_none(d: Dict[str, Any]) -> Dict[str, Any]:
    """Drop None values (matches Go omitempty semantics for pointer fields)."""
    return {k: v for k, v in d.items() if v is not None}


# --- Client ----------------------------------------------------------------


class QuidnugClient:
    """Thin, strongly-typed HTTP client for a Quidnug node.

    Example
    -------
    .. code-block:: python

        from quidnug import QuidnugClient, Quid

        client = QuidnugClient("http://localhost:8080")
        alice = Quid.generate()
        bob = Quid.generate()

        client.register_identity(alice, name="Alice")
        client.register_identity(bob, name="Bob")
        client.grant_trust(alice, trustee=bob.id, level=0.9, domain="contractors.home")

        tr = client.get_trust(alice.id, bob.id, domain="contractors.home")
        print(tr.trust_level, tr.path)

    Parameters
    ----------
    base_url:
        Base node URL. ``/api`` is automatically appended to every path.
    timeout:
        Per-request timeout in seconds. Default 30. Applies to connect + read.
    max_retries:
        Retries for transient failures (5xx, 429, connect error). Default 3.
    retry_base_delay:
        Initial backoff in seconds. Doubled each retry with ±100ms jitter.
        Default 1.0.
    session:
        Optional pre-configured ``requests.Session`` (for connection
        pooling, custom adapters, or TLS client certs).
    auth_header:
        Optional Bearer token value. Sent as ``Authorization: Bearer <x>``.
    """

    def __init__(
        self,
        base_url: str,
        *,
        timeout: float = 30.0,
        max_retries: int = 3,
        retry_base_delay: float = 1.0,
        session: Optional[requests.Session] = None,
        auth_header: Optional[str] = None,
    ) -> None:
        if not base_url:
            raise ValidationError("base_url is required")
        self.base_url = base_url.rstrip("/")
        self.api_base = self.base_url + "/api"
        self.timeout = timeout
        self.max_retries = max_retries
        self.retry_base_delay = retry_base_delay
        self._session = session or requests.Session()
        self._auth_header = auth_header

    # --- Request plumbing --------------------------------------------------

    def _request(
        self,
        method: str,
        path: str,
        *,
        params: Optional[Dict[str, Any]] = None,
        body: Any = None,
        retry: Optional[bool] = None,
    ) -> Dict[str, Any]:
        """Issue an HTTP request with retry/backoff. Returns the ``data``
        field of the standard ``{success,data,error}`` envelope.

        ``retry`` defaults to True for GETs, False for POSTs (POSTs may
        be non-idempotent and we prefer surface transport errors
        immediately so the caller can decide whether to replay).
        """
        url = urljoin(self.api_base + "/", path.lstrip("/"))
        if retry is None:
            retry = method.upper() == "GET"

        headers = {"Accept": "application/json"}
        if body is not None:
            headers["Content-Type"] = "application/json"
        if self._auth_header:
            headers["Authorization"] = f"Bearer {self._auth_header}"

        last_err: Optional[Exception] = None
        attempts = (self.max_retries + 1) if retry else 1
        for attempt in range(attempts):
            try:
                resp = self._session.request(
                    method=method,
                    url=url,
                    params=params,
                    data=json.dumps(body, default=_json_fallback) if body is not None else None,
                    headers=headers,
                    timeout=self.timeout,
                )
            except requests.RequestException as exc:
                last_err = exc
                if attempt < attempts - 1:
                    self._sleep_backoff(attempt, None)
                    continue
                raise NodeError(
                    f"Network error calling {method} {path}: {exc}",
                ) from exc

            # Retry on 5xx or 429 while we still have attempts left.
            if (resp.status_code >= 500 or resp.status_code == 429) and attempt < attempts - 1:
                last_err = _make_transport_error(method, path, resp)
                self._sleep_backoff(attempt, resp.headers.get("Retry-After"))
                continue

            # Whether terminal 5xx/429 or success: parse the envelope so
            # server-provided error codes (BOOTSTRAPPING, NOT_READY,
            # FEATURE_NOT_ACTIVE) are mapped to UnavailableError rather
            # than being masked by the generic transport-error path.
            return self._parse_envelope(resp)

        # Exhausted retries with no exception raised — unreachable, but
        # makes mypy happy.
        raise last_err or NodeError(f"Retries exhausted on {method} {path}")

    def _sleep_backoff(self, attempt: int, retry_after: Optional[str]) -> None:
        if retry_after:
            try:
                delay = float(retry_after)
                time.sleep(min(delay, 60.0))
                return
            except ValueError:
                pass
        delay = self.retry_base_delay * (2**attempt) + random.random() * 0.1
        time.sleep(min(delay, 60.0))

    def _parse_envelope(self, resp: requests.Response) -> Dict[str, Any]:
        """Validate and unwrap the standard ``{success, data, error}`` envelope."""
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

        # Failure envelope
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
        raise NodeError(
            message,
            status_code=resp.status_code,
            response_body=resp.text,
        )

    # --- Health, info, nodes -----------------------------------------------

    def health(self) -> Dict[str, Any]:
        """GET /api/health — is the node up and reachable?"""
        return self._request("GET", "health")

    def info(self) -> Dict[str, Any]:
        """GET /api/info — node identity, version, features, domains."""
        return self._request("GET", "info")

    def nodes(self, *, limit: Optional[int] = None, offset: Optional[int] = None) -> Dict[str, Any]:
        """GET /api/nodes — list known peers."""
        return self._request("GET", "nodes", params=_strip_none({"limit": limit, "offset": offset}))

    # --- Identity ----------------------------------------------------------

    def register_identity(
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
        """Submit an IDENTITY transaction naming ``subject_quid`` (defaults
        to the signer). The signer is also the definer.

        Returns the server's transaction receipt (``tx_id``, ``sequence``).
        """
        if not signer.has_private_key:
            raise ValidationError("signer must have a private key")
        subject = subject_quid or signer.id
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
        return self._request("POST", "transactions/identity", body=wire)

    def get_identity(self, quid_id: str, *, domain: Optional[str] = None) -> Optional[IdentityRecord]:
        """GET /api/identity/{quid} — returns ``None`` on 404."""
        if not quid_id:
            raise ValidationError("quid_id is required")
        try:
            data = self._request(
                "GET",
                f"identity/{quote(quid_id, safe='')}",
                params=_strip_none({"domain": domain}),
            )
        except ValidationError as exc:
            # Go returns 404 -> our ValidationError. Normalize to None.
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _identity_from_wire(data)

    def query_identity_registry(
        self,
        *,
        quid_id: Optional[str] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
    ) -> Dict[str, Any]:
        """GET /api/registry/identity — paginated dump / single lookup."""
        params = _strip_none({"quid_id": quid_id, "limit": limit, "offset": offset})
        return self._request("GET", "registry/identity", params=params)

    # --- Trust -------------------------------------------------------------

    def grant_trust(
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
        """Submit a TRUST transaction. Signer becomes the truster."""
        if not signer.has_private_key:
            raise ValidationError("signer must have a private key")
        if not trustee:
            raise ValidationError("trustee is required")
        if not 0.0 <= level <= 1.0:
            raise ValidationError("level must be in [0.0, 1.0]")
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
        return self._request("POST", "transactions/trust", body=wire)

    def get_trust(
        self,
        observer: str,
        target: str,
        *,
        domain: str = "default",
        max_depth: Optional[int] = None,
    ) -> TrustResult:
        """GET /api/trust/{observer}/{target}?domain=... — compute relational trust."""
        if not observer or not target:
            raise ValidationError("observer and target are required")
        params = _strip_none({"domain": domain, "maxDepth": max_depth})
        path = f"trust/{quote(observer, safe='')}/{quote(target, safe='')}"
        data = self._request("GET", path, params=params)
        return _trust_result_from_wire(observer, target, domain, data)

    def query_relational_trust(
        self,
        *,
        observer: str,
        target: str,
        domain: str = "default",
        max_depth: int = 5,
    ) -> TrustResult:
        """POST /api/trust/query — structured relational trust query."""
        body = {"observer": observer, "target": target, "domain": domain, "maxDepth": max_depth}
        data = self._request("POST", "trust/query", body=body)
        return _trust_result_from_wire(observer, target, domain, data)

    def get_trust_edges(self, quid_id: str) -> List[TrustEdge]:
        """GET /api/trust/edges/{quid} — direct outbound trust edges."""
        data = self._request("GET", f"trust/edges/{quote(quid_id, safe='')}")
        raw = data.get("edges") or data.get("data") or []
        if not isinstance(raw, list):
            return []
        return [_trust_edge_from_wire(e) for e in raw]

    def query_trust_registry(
        self,
        *,
        truster: Optional[str] = None,
        trustee: Optional[str] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
    ) -> Dict[str, Any]:
        """GET /api/registry/trust — paginated trust-edge listing."""
        params = _strip_none(
            {"truster": truster, "trustee": trustee, "limit": limit, "offset": offset}
        )
        return self._request("GET", "registry/trust", params=params)

    # --- Title / ownership -------------------------------------------------

    def register_title(
        self,
        signer: Quid,
        *,
        asset_id: str,
        owners: List[OwnershipStake],
        domain: str = "default",
        title_type: Optional[str] = None,
        prev_title_tx_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Submit a TITLE transaction.

        ``owners`` percentages may be provided on either the
        1.0 (fraction) or 100.0 (percent) scale; values are
        normalized to fraction for the wire (the server's
        invariant is sum == 1.0)."""
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
        wire_owners = [
            _WireOwnershipStake(
                owner_id=s.owner_id,
                percentage=s.percentage * norm,
                stake_type=getattr(s, "stake_type", "") or "",
            )
            for s in owners
        ]
        # prev_title_tx_id is accepted for backward-compat but not
        # present in the v1.0 wire schema (transfers use
        # previous_owners). Silently ignored for now.
        _ = prev_title_tx_id
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
        return self._request("POST", "transactions/title", body=wire)

    def get_title(self, asset_id: str, *, domain: Optional[str] = None) -> Optional[Title]:
        """GET /api/title/{asset} — returns ``None`` on 404."""
        try:
            data = self._request(
                "GET",
                f"title/{quote(asset_id, safe='')}",
                params=_strip_none({"domain": domain}),
            )
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _title_from_wire(data)

    def query_title_registry(
        self,
        *,
        asset_id: Optional[str] = None,
        owner_id: Optional[str] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
    ) -> Dict[str, Any]:
        params = _strip_none(
            {"asset_id": asset_id, "owner_id": owner_id, "limit": limit, "offset": offset}
        )
        return self._request("GET", "registry/title", params=params)

    # --- Events ------------------------------------------------------------

    def emit_event(
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
        """Submit an EVENT transaction. Signer must own ``subject_id``.

        Either ``payload`` (inline) or ``payload_cid`` (IPFS-resolved)
        must be provided, but not both.
        """
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
                stream = self.get_event_stream(subject_id, domain=domain)
                sequence = (stream.get("latestSequence", 0) + 1) if stream else 1
            except QuidnugError:
                sequence = 1

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
        return self._request("POST", "events", body=wire)

    def get_event_stream(self, subject_id: str, *, domain: Optional[str] = None) -> Optional[Dict[str, Any]]:
        """GET /api/streams/{subject} — stream metadata or None on 404."""
        try:
            return self._request(
                "GET",
                f"streams/{quote(subject_id, safe='')}",
                params=_strip_none({"domain": domain}),
            )
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise

    def get_stream_events(
        self,
        subject_id: str,
        *,
        domain: Optional[str] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
    ) -> Tuple[List[Event], Dict[str, Any]]:
        """GET /api/streams/{subject}/events — returns (events, pagination)."""
        data = self._request(
            "GET",
            f"streams/{quote(subject_id, safe='')}/events",
            params=_strip_none({"domain": domain, "limit": limit, "offset": offset}),
        )
        raw = data.get("data") or data.get("events") or []
        events = [_event_from_wire(e) for e in raw] if isinstance(raw, list) else []
        pagination = data.get("pagination") or {}
        return events, pagination

    # --- IPFS / large-payload storage --------------------------------------

    def ipfs_pin(self, content: Union[str, bytes]) -> str:
        """POST /api/ipfs/pin — returns the CID of pinned content."""
        headers = {"Content-Type": "application/octet-stream"}
        if isinstance(content, str):
            body = content.encode("utf-8")
        else:
            body = content
        resp = self._session.post(
            urljoin(self.api_base + "/", "ipfs/pin"),
            data=body,
            headers=headers,
            timeout=self.timeout,
        )
        data = self._parse_envelope(resp)
        cid = data.get("cid") or data.get("value")
        if not cid:
            raise NodeError("IPFS pin response missing cid")
        return cid

    def ipfs_get(self, cid: str) -> bytes:
        """GET /api/ipfs/{cid} — fetch raw bytes."""
        resp = self._session.get(
            urljoin(self.api_base + "/", f"ipfs/{quote(cid, safe='')}"),
            timeout=self.timeout,
        )
        if resp.status_code >= 400:
            raise NodeError(
                f"IPFS fetch failed (HTTP {resp.status_code})",
                status_code=resp.status_code,
                response_body=resp.text,
            )
        return resp.content

    # --- Guardian sets + recovery (QDP-0002 / QDP-0006) --------------------

    def submit_guardian_set_update(self, update: GuardianSetUpdate) -> Dict[str, Any]:
        """POST /api/guardian/set-update — install or rotate guardians."""
        return self._request("POST", "guardian/set-update", body=_dc(update))

    def submit_recovery_init(self, init: GuardianRecoveryInit) -> Dict[str, Any]:
        """POST /api/guardian/recovery/init — start the M-of-N recovery delay."""
        return self._request("POST", "guardian/recovery/init", body=_dc(init))

    def submit_recovery_veto(self, veto: GuardianRecoveryVeto) -> Dict[str, Any]:
        """POST /api/guardian/recovery/veto — owner or guardian aborts recovery."""
        return self._request("POST", "guardian/recovery/veto", body=_dc(veto))

    def submit_recovery_commit(self, commit: GuardianRecoveryCommit) -> Dict[str, Any]:
        """POST /api/guardian/recovery/commit — finalize the delayed recovery."""
        return self._request("POST", "guardian/recovery/commit", body=_dc(commit))

    def submit_guardian_resignation(self, resignation: GuardianResignation) -> Dict[str, Any]:
        """POST /api/guardian/resign — guardian leaves the set."""
        return self._request("POST", "guardian/resign", body=_dc(resignation))

    def get_guardian_set(self, quid: str) -> Optional[GuardianSet]:
        """GET /api/guardian/set/{quid} — current guardian set or None."""
        try:
            data = self._request("GET", f"guardian/set/{quote(quid, safe='')}")
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _guardian_set_from_wire(data)

    def get_pending_recovery(self, quid: str) -> Optional[Dict[str, Any]]:
        """GET /api/guardian/pending-recovery/{quid}."""
        try:
            return self._request("GET", f"guardian/pending-recovery/{quote(quid, safe='')}")
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise

    def get_guardian_resignations(self, quid: str) -> List[Dict[str, Any]]:
        """GET /api/guardian/resignations/{quid}."""
        data = self._request("GET", f"guardian/resignations/{quote(quid, safe='')}")
        raw = data.get("data") or data.get("resignations") or []
        return raw if isinstance(raw, list) else []

    # --- Cross-domain gossip + fingerprints (QDP-0003 / QDP-0005) ----------

    def submit_domain_fingerprint(self, fp: DomainFingerprint) -> Dict[str, Any]:
        """POST /api/domain-fingerprints — publish a signed fingerprint."""
        return self._request("POST", "domain-fingerprints", body=_dc(fp))

    def get_latest_domain_fingerprint(self, domain: str) -> Optional[DomainFingerprint]:
        """GET /api/domain-fingerprints/{domain}/latest."""
        try:
            data = self._request("GET", f"domain-fingerprints/{quote(domain, safe='')}/latest")
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _domain_fingerprint_from_wire(data)

    def submit_anchor_gossip(self, message: AnchorGossipMessage) -> Dict[str, Any]:
        """POST /api/anchor-gossip — deliver a cross-domain anchor message.

        Idempotent: the node returns 200 with ``data.duplicate=True`` on
        re-receipt of an already-accepted message_id.
        """
        return self._request("POST", "anchor-gossip", body=_dc(message))

    def push_anchor(self, message: AnchorGossipMessage) -> Dict[str, Any]:
        """POST /api/gossip/push-anchor — push gossip variant (QDP-0005)."""
        return self._request("POST", "gossip/push-anchor", body=_dc(message))

    def push_fingerprint(self, fp: DomainFingerprint) -> Dict[str, Any]:
        """POST /api/gossip/push-fingerprint — push gossip variant (QDP-0005)."""
        return self._request("POST", "gossip/push-fingerprint", body=_dc(fp))

    # --- Bootstrap + nonce snapshots (QDP-0008) ----------------------------

    def submit_nonce_snapshot(self, snapshot: NonceSnapshot) -> Dict[str, Any]:
        """POST /api/nonce-snapshots — publish a K-of-K bootstrap snapshot."""
        return self._request("POST", "nonce-snapshots", body=_dc(snapshot))

    def get_latest_nonce_snapshot(self, domain: str) -> Optional[NonceSnapshot]:
        """GET /api/nonce-snapshots/{domain}/latest."""
        try:
            data = self._request("GET", f"nonce-snapshots/{quote(domain, safe='')}/latest")
        except ValidationError as exc:
            if (exc.details or {}).get("code") == "NOT_FOUND":
                return None
            raise
        return _nonce_snapshot_from_wire(data)

    def bootstrap_status(self) -> Dict[str, Any]:
        """GET /api/bootstrap/status."""
        return self._request("GET", "bootstrap/status")

    # --- Fork-block (QDP-0009) ---------------------------------------------

    def submit_fork_block(self, fb: ForkBlock) -> Dict[str, Any]:
        """POST /api/fork-block — submit a signed fork-activation block."""
        return self._request("POST", "fork-block", body=_dc(fb))

    def fork_block_status(self) -> Dict[str, Any]:
        """GET /api/fork-block/status — activation status across features."""
        return self._request("GET", "fork-block/status")

    # --- Blocks + transactions ---------------------------------------------

    def get_blocks(self, *, limit: Optional[int] = None, offset: Optional[int] = None) -> Dict[str, Any]:
        return self._request("GET", "blocks", params=_strip_none({"limit": limit, "offset": offset}))

    def get_tentative_blocks(self, domain: str) -> Dict[str, Any]:
        return self._request("GET", f"blocks/tentative/{quote(domain, safe='')}")

    def get_pending_transactions(
        self, *, limit: Optional[int] = None, offset: Optional[int] = None
    ) -> Dict[str, Any]:
        return self._request("GET", "transactions", params=_strip_none({"limit": limit, "offset": offset}))

    # --- Domains -----------------------------------------------------------

    def list_domains(self) -> Dict[str, Any]:
        return self._request("GET", "domains")

    def register_domain(self, domain: str, **attrs: Any) -> Dict[str, Any]:
        body = {"name": domain, **attrs}
        return self._request("POST", "domains", body=body)

    def ensure_domain(self, domain: str, **attrs: Any) -> Dict[str, Any]:
        """Register the domain if it doesn't already exist. Idempotent.

        Convenience wrapper around ``register_domain`` for demos
        and bootstrap scripts that don't care whether the domain
        was just created or already present.
        """
        try:
            return self.register_domain(domain, **attrs)
        except ValidationError as e:
            if "already exists" in str(e).lower():
                return {"status": "success", "domain": domain,
                         "message": "trust domain already exists"}
            raise

    def wait_for_identity(
        self, quid_id: str, *, timeout: float = 30.0, poll: float = 0.5,
    ) -> "IdentityRecord":
        """Block until the identity with ``quid_id`` is visible in
        the committed registry, or raise TimeoutError.

        A just-submitted identity transaction lives in the pending
        pool until the next block is sealed. Code that immediately
        emits events or title transactions referencing the new
        quid must wait for commit first. Demos and bootstrap
        scripts use this to avoid racing the block producer.
        """
        import time as _time
        deadline = _time.time() + timeout
        while _time.time() < deadline:
            rec = self.get_identity(quid_id)
            if rec is not None:
                return rec
            _time.sleep(poll)
        raise TimeoutError(
            f"identity {quid_id} did not commit within {timeout}s"
        )

    def wait_for_identities(
        self, quid_ids: List[str], *, timeout: float = 30.0, poll: float = 0.5,
    ) -> None:
        """Block until every quid_id in the list is committed.
        Shares a single deadline across all ids."""
        import time as _time
        deadline = _time.time() + timeout
        for qid in quid_ids:
            remaining = max(0.0, deadline - _time.time())
            if remaining <= 0:
                raise TimeoutError(
                    f"identities not all committed within {timeout}s "
                    f"(blocked on {qid})"
                )
            self.wait_for_identity(qid, timeout=remaining, poll=poll)

    def wait_for_title(
        self, asset_id: str, *, timeout: float = 30.0, poll: float = 0.5,
    ) -> "Title":
        """Block until the title with ``asset_id`` is visible in
        the committed registry, or raise TimeoutError.

        A just-submitted title transaction lives in the pending
        pool until the next block is sealed. Code that immediately
        emits events referencing the title subject must wait for
        commit first.
        """
        import time as _time
        deadline = _time.time() + timeout
        while _time.time() < deadline:
            rec = self.get_title(asset_id)
            if rec is not None:
                return rec
            _time.sleep(poll)
        raise TimeoutError(
            f"title {asset_id} did not commit within {timeout}s"
        )

    def get_node_domains(self) -> Dict[str, Any]:
        return self._request("GET", "node/domains")

    def update_node_domains(self, domains: List[str]) -> Dict[str, Any]:
        return self._request("POST", "node/domains", body={"managedDomains": domains})


# --- Wire -> dataclass decoders -------------------------------------------
#
# Kept at module scope so users can reuse them when they hold a raw
# envelope from some other source (e.g. a WebSocket event, a replay log).


def _identity_from_wire(d: Dict[str, Any]) -> IdentityRecord:
    return IdentityRecord(
        quid_id=d.get("quidId") or d.get("quid_id") or "",
        creator=d.get("creator") or d.get("definerQuid") or "",
        update_nonce=int(d.get("updateNonce", d.get("update_nonce", 0))),
        signature=d.get("signature", ""),
        name=d.get("name"),
        description=d.get("description"),
        attributes=d.get("attributes") or {},
        home_domain=d.get("homeDomain") or d.get("home_domain"),
        public_key=d.get("publicKey") or d.get("public_key"),
    )


def _title_from_wire(d: Dict[str, Any]) -> Title:
    owners_raw = d.get("ownershipMap") or d.get("owners") or []
    owners = [
        OwnershipStake(
            owner_id=s.get("ownerId") or s.get("owner_id") or s.get("owner", ""),
            percentage=float(s.get("percentage", 0.0)),
            stake_type=s.get("stakeType") or s.get("stake_type"),
        )
        for s in owners_raw
    ]
    return Title(
        asset_id=d.get("assetId") or d.get("asset_id") or d.get("assetQuid") or "",
        domain=d.get("domain") or d.get("trustDomain") or "",
        title_type=d.get("titleType") or d.get("title_type") or "",
        owners=owners,
        attributes=d.get("attributes") or {},
        creator=d.get("issuerQuid") or d.get("creator", ""),
        signatures=d.get("transferSigs") or d.get("signatures") or {},
    )


def _event_from_wire(d: Dict[str, Any]) -> Event:
    return Event(
        subject_id=d.get("subjectId") or d.get("subject_id") or "",
        subject_type=d.get("subjectType") or d.get("subject_type") or "QUID",
        event_type=d.get("eventType") or d.get("event_type") or "",
        payload=d.get("payload") or {},
        payload_cid=d.get("payloadCid") or d.get("payload_cid"),
        timestamp=d.get("timestamp"),
        sequence=d.get("sequence"),
        creator=d.get("creator", "") or d.get("signerQuid", ""),
        signature=d.get("signature", ""),
    )


def _trust_edge_from_wire(d: Dict[str, Any]) -> TrustEdge:
    return TrustEdge(
        truster=d.get("truster", ""),
        trustee=d.get("trustee", ""),
        trust_level=float(d.get("trustLevel", d.get("trust_level", 0.0))),
        domain=d.get("domain") or d.get("trustDomain") or "",
        nonce=int(d.get("nonce", 0)),
        signature=d.get("signature", ""),
        valid_until=d.get("validUntil") or d.get("valid_until"),
        description=d.get("description"),
        attributes=d.get("attributes") or {},
    )


def _trust_result_from_wire(
    observer: str, target: str, domain: str, d: Dict[str, Any]
) -> TrustResult:
    return TrustResult(
        observer=d.get("observer", observer),
        target=d.get("target", target),
        trust_level=float(d.get("trustLevel", d.get("trust_level", 0.0))),
        path=list(d.get("trustPath") or d.get("path") or []),
        path_depth=int(d.get("pathDepth", d.get("path_depth", 0))),
        domain=d.get("domain", domain),
    )


def _guardian_set_from_wire(d: Dict[str, Any]) -> GuardianSet:
    from quidnug.types import GuardianRef

    guardians_raw = d.get("guardians") or []
    guardians = [
        GuardianRef(
            quid=g.get("quid", ""),
            weight=int(g.get("weight", 1)),
            epoch=int(g.get("epoch", 0)),
            added_at_block=g.get("addedAtBlock") or g.get("added_at_block"),
        )
        for g in guardians_raw
    ]
    return GuardianSet(
        subject_quid=d.get("subjectQuid") or d.get("subject_quid") or "",
        guardians=guardians,
        threshold=int(d.get("threshold", 0)),
        recovery_delay_seconds=int(d.get("recoveryDelaySeconds") or d.get("recovery_delay_seconds") or 0),
        require_guardian_rotation=bool(
            d.get("requireGuardianRotation") or d.get("require_guardian_rotation") or False
        ),
        updated_at_block=d.get("updatedAtBlock") or d.get("updated_at_block"),
    )


def _domain_fingerprint_from_wire(d: Dict[str, Any]) -> DomainFingerprint:
    return DomainFingerprint(
        domain=d.get("domain", ""),
        block_height=int(d.get("blockHeight", d.get("block_height", 0))),
        block_hash=d.get("blockHash") or d.get("block_hash") or "",
        producer_quid=d.get("producerQuid") or d.get("producer_quid") or "",
        timestamp=int(d.get("timestamp", 0)),
        signature=d.get("signature", ""),
        schema_version=int(d.get("schemaVersion", d.get("schema_version", 1))),
    )


def _nonce_snapshot_from_wire(d: Dict[str, Any]) -> NonceSnapshot:
    from quidnug.types import NonceSnapshotEntry

    entries_raw = d.get("entries") or []
    entries = [
        NonceSnapshotEntry(
            quid=e.get("quid", ""),
            epoch=int(e.get("epoch", 0)),
            max_nonce=int(e.get("maxNonce", e.get("max_nonce", 0))),
        )
        for e in entries_raw
    ]
    return NonceSnapshot(
        block_height=int(d.get("blockHeight", d.get("block_height", 0))),
        block_hash=d.get("blockHash") or d.get("block_hash") or "",
        timestamp=int(d.get("timestamp", 0)),
        trust_domain=d.get("trustDomain") or d.get("trust_domain") or "",
        entries=entries,
        producer_quid=d.get("producerQuid") or d.get("producer_quid") or "",
        signature=d.get("signature", ""),
        schema_version=int(d.get("schemaVersion", d.get("schema_version", 1))),
    )


# --- Misc helpers ----------------------------------------------------------


def _make_transport_error(method: str, path: str, resp: requests.Response) -> NodeError:
    return NodeError(
        f"{method} {path} → HTTP {resp.status_code}",
        status_code=resp.status_code,
        response_body=resp.text,
    )


def _json_fallback(obj: Any) -> Any:
    if hasattr(obj, "__dataclass_fields__"):
        return asdict(obj)
    if isinstance(obj, bytes):
        return obj.hex()
    raise TypeError(f"Type {type(obj).__name__} not JSON serializable")


__all__ = ["QuidnugClient"]
