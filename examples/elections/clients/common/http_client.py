"""
HTTP client wrapper around the Quidnug node's REST API.

Handles:
  - POSTing signed transactions (identity, trust, event)
  - GETing event streams and trust queries
  - Discovery (QDP-0014): finding which nodes serve a given
    election domain, resolving endpoints from
    `.well-known/quidnug-network.json`
  - Blind-issuance request (QDP-0021, mock if not yet
    implemented on the node)

The client is deliberately small; it's a reference, not a
production SDK. For production use @quidnug/client-python.
"""
from __future__ import annotations

import json
import logging
import time
from typing import Any, Optional

import requests

logger = logging.getLogger(__name__)


class HTTPClient:
    """Minimal Quidnug node HTTP client."""

    def __init__(self, base_url: str, timeout_seconds: int = 10):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout_seconds
        self.session = requests.Session()

    def _post(self, path: str, body: dict) -> dict:
        url = f"{self.base_url}{path}"
        resp = self.session.post(
            url,
            data=json.dumps(body, separators=(",", ":")),
            headers={"Content-Type": "application/json"},
            timeout=self.timeout,
        )
        try:
            j = resp.json()
        except Exception:
            raise RuntimeError(f"non-JSON response {resp.status_code}: {resp.text[:200]}")
        if not j.get("success"):
            raise RuntimeError(f"{resp.status_code}: {j.get('error', j)}")
        return j.get("data") or {}

    def _get(self, path: str, params: Optional[dict] = None) -> dict:
        url = f"{self.base_url}{path}"
        resp = self.session.get(url, params=params, timeout=self.timeout)
        try:
            j = resp.json()
        except Exception:
            raise RuntimeError(f"non-JSON response {resp.status_code}: {resp.text[:200]}")
        if not j.get("success"):
            raise RuntimeError(f"{resp.status_code}: {j.get('error', j)}")
        return j.get("data") or {}

    # ---------------- Transaction submission ----------------

    def submit_identity(self, signed_tx_dict: dict) -> dict:
        return self._post("/api/transactions/identity", signed_tx_dict)

    def submit_trust(self, signed_tx_dict: dict) -> dict:
        return self._post("/api/transactions/trust", signed_tx_dict)

    def submit_event(self, signed_tx_dict: dict) -> dict:
        return self._post("/api/events", signed_tx_dict)

    # ---------------- Queries ----------------

    def get_stream(self, subject_id: str, event_type: Optional[str] = None,
                   since: Optional[int] = None, limit: int = 1000) -> list:
        """List events on a quid's or title's stream, optionally
        filtered."""
        params = {"limit": limit}
        if event_type:
            params["eventType"] = event_type
        if since is not None:
            params["since"] = since
        data = self._get(f"/api/streams/{subject_id}/events", params)
        return data.get("data") or data.get("events") or []

    def get_trust(self, truster: str, trustee: str, domain: str,
                  max_depth: int = 5) -> float:
        data = self._get(
            f"/api/trust/{truster}/{trustee}",
            params={"domain": domain, "maxDepth": max_depth},
        )
        return float(data.get("trustLevel", 0.0))

    def get_identity(self, quid_id: str) -> Optional[dict]:
        try:
            return self._get(f"/api/identity/{quid_id}")
        except RuntimeError:
            return None

    # ---------------- Discovery (QDP-0014) ----------------

    def discover_domain(self, domain_name: str) -> dict:
        """QDP-0014 §6.1. Returns consortium + endpoint hints +
        block tip for a domain. Signed response."""
        return self._get(f"/api/v2/discovery/domain/{domain_name}")

    def discover_node(self, node_quid: str) -> dict:
        """QDP-0014 §6.2. Returns the node's current
        NODE_ADVERTISEMENT."""
        return self._get(f"/api/v2/discovery/node/{node_quid}")

    def discover_operator_nodes(self, operator_quid: str) -> list:
        """QDP-0014 §6.3. Returns all nodes attested by the
        operator."""
        data = self._get(f"/api/v2/discovery/operator/{operator_quid}")
        return data.get("nodes") or []

    def discover_quids_in_domain(self, domain: str, sort: str = "activity",
                                  limit: int = 100,
                                  observer: Optional[str] = None) -> list:
        """QDP-0014 §15.2. Per-domain quid index."""
        params = {"domain": domain, "sort": sort, "limit": limit}
        if observer:
            params["observer"] = observer
        data = self._get("/api/v2/discovery/quids", params)
        return data.get("quids") or []

    def get_well_known(self) -> dict:
        """Fetch the operator's `.well-known/quidnug-network.json`
        file, parse it, return the contents. Signature
        verification is the caller's responsibility."""
        url = f"{self.base_url}/.well-known/quidnug-network.json"
        resp = self.session.get(url, timeout=self.timeout)
        resp.raise_for_status()
        return resp.json()

    # ---------------- Blind issuance (QDP-0021) ----------------

    def request_ballot_signing(self, election_id: str, signed_request: dict) -> dict:
        """QDP-0021 §6.2. Sends the blinded-token request;
        receives a signed blinded token + the BALLOT_ISSUED event
        tx ID.

        MOCK: until QDP-0021 lands in the node, this endpoint
        is provided by a local Python mock server
        (setup_authority.py runs it). Once the real endpoint is
        live, no code change needed here — only the base URL
        changes."""
        return self._post(
            f"/api/v2/elections/{election_id}/ballot-request",
            signed_request,
        )

    # ---------------- Health + info ----------------

    def health(self) -> dict:
        return self._get("/api/health")

    def info(self) -> dict:
        return self._get("/api/info")
