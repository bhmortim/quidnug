"""Tests for the QuidnugClient HTTP surface.

Uses responses library patterns via `requests_mock`-style fixtures. To
keep the dependency surface minimal, we use a small custom mock adapter
instead of requiring ``responses`` or ``requests_mock`` as a test dep.
"""

from __future__ import annotations

import json
from typing import Any, Dict, List, Optional, Tuple
from unittest.mock import MagicMock

import pytest
import requests

from quidnug import (
    ConflictError,
    NodeError,
    Quid,
    QuidnugClient,
    UnavailableError,
    ValidationError,
)
from quidnug.types import OwnershipStake


class _FakeResponse:
    def __init__(
        self,
        *,
        status_code: int = 200,
        payload: Any = None,
        text: Optional[str] = None,
        headers: Optional[Dict[str, str]] = None,
    ) -> None:
        self.status_code = status_code
        self._payload = payload
        self._text = text if text is not None else json.dumps(payload if payload is not None else {})
        self.headers = headers or {}
        self.content = self._text.encode("utf-8")

    def json(self) -> Any:
        if self._payload is None:
            raise ValueError("no json")
        return self._payload

    @property
    def text(self) -> str:
        return self._text


class _FakeSession:
    def __init__(self) -> None:
        self.calls: List[Tuple[str, str, Dict[str, Any]]] = []
        self._responses: List[_FakeResponse] = []

    def queue(self, resp: _FakeResponse) -> None:
        self._responses.append(resp)

    def request(self, method: str, url: str, **kw: Any) -> _FakeResponse:
        self.calls.append((method.upper(), url, kw))
        if not self._responses:
            raise AssertionError(f"No response queued for {method} {url}")
        return self._responses.pop(0)

    def post(self, url: str, **kw: Any) -> _FakeResponse:
        return self.request("POST", url, **kw)

    def get(self, url: str, **kw: Any) -> _FakeResponse:
        return self.request("GET", url, **kw)


@pytest.fixture
def session() -> _FakeSession:
    return _FakeSession()


@pytest.fixture
def client(session: _FakeSession) -> QuidnugClient:
    return QuidnugClient("http://node.local:8080", session=session, max_retries=0, retry_base_delay=0.0)


# --- Envelope parsing -----------------------------------------------------


def test_success_envelope_unwraps_data(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {"foo": "bar"}}))
    assert client.health() == {"foo": "bar"}


def test_success_envelope_scalar_data_wrapped(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": 42}))
    assert client.health() == {"value": 42}


def test_error_envelope_with_409_raises_conflict(session, client):
    session.queue(
        _FakeResponse(
            status_code=409,
            payload={
                "success": False,
                "error": {"code": "NONCE_REPLAY", "message": "replay detected"},
            },
        )
    )
    with pytest.raises(ConflictError) as exc:
        client.health()
    assert "replay" in exc.value.message.lower()


def test_error_envelope_with_503_raises_unavailable(session, client):
    session.queue(
        _FakeResponse(
            status_code=503,
            payload={"success": False, "error": {"code": "BOOTSTRAPPING", "message": "not ready"}},
        )
    )
    with pytest.raises(UnavailableError):
        client.health()


def test_feature_not_active_raises_unavailable(session, client):
    session.queue(
        _FakeResponse(
            status_code=400,
            payload={
                "success": False,
                "error": {"code": "FEATURE_NOT_ACTIVE", "message": "fork gate closed"},
            },
        )
    )
    with pytest.raises(UnavailableError):
        client.health()


def test_5xx_is_node_error(session, client):
    session.queue(
        _FakeResponse(
            status_code=500,
            payload={"success": False, "error": {"code": "INTERNAL", "message": "oops"}},
        )
    )
    with pytest.raises(NodeError) as exc:
        client.health()
    assert exc.value.status_code == 500


def test_non_json_response_raises_node_error(session, client):
    session.queue(_FakeResponse(status_code=500, text="<html>500</html>", payload=None))
    with pytest.raises(NodeError):
        client.health()


def test_network_error_wraps_as_node_error(client: QuidnugClient):
    sess = MagicMock()
    sess.request.side_effect = requests.ConnectionError("DNS failure")
    client._session = sess
    with pytest.raises(NodeError) as exc:
        client.health()
    assert "Network error" in exc.value.message


# --- Validation happy-paths -----------------------------------------------


def test_grant_trust_requires_private_key(client):
    pub_only = Quid.from_public_hex(Quid.generate().public_key_hex)
    with pytest.raises(ValidationError):
        client.grant_trust(pub_only, trustee="abc", level=0.5)


def test_grant_trust_rejects_level_out_of_range(client):
    q = Quid.generate()
    with pytest.raises(ValidationError):
        client.grant_trust(q, trustee="abc", level=1.5)
    with pytest.raises(ValidationError):
        client.grant_trust(q, trustee="abc", level=-0.1)


def test_register_title_validates_percentage_sum(client):
    q = Quid.generate()
    bad_owners = [OwnershipStake(owner_id="a", percentage=50.0), OwnershipStake(owner_id="b", percentage=49.0)]
    with pytest.raises(ValidationError):
        client.register_title(q, asset_id="asset-1", owners=bad_owners)


def test_emit_event_rejects_both_payload_and_cid(client):
    q = Quid.generate()
    with pytest.raises(ValidationError):
        client.emit_event(
            q,
            subject_id="s",
            subject_type="QUID",
            event_type="note",
            payload={"x": 1},
            payload_cid="Qm...",
        )


def test_emit_event_rejects_bad_subject_type(client):
    q = Quid.generate()
    with pytest.raises(ValidationError):
        client.emit_event(
            q, subject_id="s", subject_type="NOPE", event_type="note", payload={"x": 1}
        )


# --- Endpoint routing is correct ------------------------------------------


def test_grant_trust_posts_to_transactions_trust(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {"txId": "abc"}}))
    q = Quid.generate()
    client.grant_trust(q, trustee="bob", level=0.8, domain="contractors.home")
    method, url, kw = session.calls[0]
    assert method == "POST"
    assert url.endswith("/api/transactions/trust")
    body = json.loads(kw["data"])
    assert body["type"] == "TRUST"
    assert body["truster"] == q.id
    assert body["trustee"] == "bob"
    assert body["trustLevel"] == 0.8
    assert body["trustDomain"] == "contractors.home"
    assert "signature" in body


def test_get_trust_uses_path_segments(session, client):
    session.queue(
        _FakeResponse(
            payload={
                "success": True,
                "data": {"trustLevel": 0.72, "trustPath": ["a", "b", "c"], "pathDepth": 2},
            }
        )
    )
    result = client.get_trust("alice", "carol", domain="contractors.home", max_depth=3)
    method, url, kw = session.calls[0]
    assert method == "GET"
    assert "/api/trust/alice/carol" in url
    assert kw["params"]["domain"] == "contractors.home"
    assert kw["params"]["maxDepth"] == 3
    assert result.trust_level == 0.72
    assert result.path == ["a", "b", "c"]
    assert result.path_depth == 2


def test_get_identity_returns_none_on_not_found(session, client):
    session.queue(
        _FakeResponse(
            status_code=404,
            payload={"success": False, "error": {"code": "NOT_FOUND", "message": "absent"}},
        )
    )
    assert client.get_identity("missing") is None


def test_retry_after_transient_5xx(session):
    client = QuidnugClient("http://n.local", session=session, max_retries=2, retry_base_delay=0.001)
    # Two 503s then success
    session.queue(
        _FakeResponse(status_code=503, payload={"success": False, "error": {"code": "NOT_READY"}})
    )
    session.queue(
        _FakeResponse(status_code=503, payload={"success": False, "error": {"code": "NOT_READY"}})
    )
    session.queue(_FakeResponse(payload={"success": True, "data": {"ok": True}}))
    result = client.health()
    assert result == {"ok": True}
    assert len(session.calls) == 3


def test_does_not_retry_4xx_client_errors(session):
    client = QuidnugClient("http://n.local", session=session, max_retries=3, retry_base_delay=0.001)
    session.queue(
        _FakeResponse(
            status_code=400,
            payload={"success": False, "error": {"code": "BAD_REQUEST", "message": "nope"}},
        )
    )
    with pytest.raises(ValidationError):
        client.health()
    assert len(session.calls) == 1  # no retry


def test_post_is_not_retried_by_default(session):
    client = QuidnugClient("http://n.local", session=session, max_retries=3, retry_base_delay=0.001)
    session.queue(
        _FakeResponse(
            status_code=500,
            payload={"success": False, "error": {"code": "INTERNAL"}},
        )
    )
    q = Quid.generate()
    with pytest.raises(NodeError):
        client.grant_trust(q, trustee="x", level=0.5)
    assert len(session.calls) == 1


# --- Wire-format coverage for new v1 + v2 methods --------------------------


def test_peers_hits_peers_endpoint(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {"peers": [], "count": 0}}))
    assert client.peers() == {"peers": [], "count": 0}
    method, url, _ = session.calls[0]
    assert method == "GET"
    assert url.endswith("/api/peers")


def test_get_peer_returns_none_on_not_found(session, client):
    session.queue(
        _FakeResponse(
            status_code=404,
            payload={"success": False, "error": {"code": "PEER_NOT_FOUND"}},
        )
    )
    assert client.get_peer("nodex") is None


def test_generate_quid_posts_metadata(session, client):
    session.queue(
        _FakeResponse(
            payload={"success": True, "data": {"quidId": "deadbeef", "publicKey": "...", "created": 1}}
        )
    )
    client.generate_quid(metadata={"label": "alice"})
    method, url, kw = session.calls[0]
    assert method == "POST"
    assert url.endswith("/api/quids")
    body = json.loads(kw["data"])
    assert body == {"metadata": {"label": "alice"}}


def test_top_domains_hits_endpoint(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {"domains": []}}))
    client.top_domains()
    method, url, _ = session.calls[0]
    assert method == "GET"
    assert url.endswith("/api/domains/top")


def test_query_domain_sends_type_and_param(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {}}))
    client.query_domain("foo.com", query_type="trust", param="a:b")
    method, url, kw = session.calls[0]
    assert method == "GET"
    assert "/api/domains/foo.com/query" in url
    assert kw["params"] == {"type": "trust", "param": "a:b"}


def test_query_domain_rejects_bad_type(client):
    with pytest.raises(ValidationError):
        client.query_domain("foo.com", query_type="bogus", param="x")


def test_create_moderation_action_emits_camel_case(session, client):
    from quidnug.types import ModerationAction

    session.queue(_FakeResponse(payload={"success": True, "data": {"id": "tx1"}}))
    action = ModerationAction(
        moderator_quid="modquid",
        target_type="QUID",
        target_id="targetquid",
        scope="hide",
        reason_code="spam",
        nonce=1,
        do_not_federate=True,
    )
    client.create_moderation_action(action)
    method, url, kw = session.calls[0]
    assert method == "POST"
    assert url.endswith("/api/moderation/actions")
    body = json.loads(kw["data"])
    # snake_case dataclass field names must round-trip to camelCase JSON
    assert body["moderatorQuid"] == "modquid"
    assert body["targetType"] == "QUID"
    assert body["targetId"] == "targetquid"
    assert body["doNotFederate"] is True
    # ensure no snake_case keys leaked
    assert "moderator_quid" not in body


def test_get_moderation_actions_path(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {"actions": []}}))
    client.get_moderation_actions(target_type="QUID", target_id="abc")
    _, url, _ = session.calls[0]
    assert "/api/moderation/actions/QUID/abc" in url


def test_audit_head_decodes_envelope(session, client):
    session.queue(
        _FakeResponse(
            payload={
                "success": True,
                "data": {
                    "operatorQuid": "op",
                    "height": 42,
                    "headHash": "abcd",
                    "headSequence": 41,
                    "headTimestamp": 1000,
                },
            }
        )
    )
    head = client.audit_head()
    assert head.operator_quid == "op"
    assert head.height == 42
    assert head.head_hash == "abcd"


def test_audit_entries_decodes_list(session, client):
    session.queue(
        _FakeResponse(
            payload={
                "success": True,
                "data": {
                    "operatorQuid": "op",
                    "height": 2,
                    "entries": [
                        {
                            "sequence": 0,
                            "timestamp": 1,
                            "hash": "h0",
                            "prevHash": "0",
                            "operatorQuid": "op",
                            "eventType": "block_sealed",
                            "payload": {"height": 0},
                        }
                    ],
                },
            }
        )
    )
    entries, info = client.audit_entries(since=-1, limit=10)
    assert len(entries) == 1
    assert entries[0].sequence == 0
    assert entries[0].hash == "h0"
    assert info["height"] == 2


def test_audit_entry_returns_none_on_404(session, client):
    session.queue(
        _FakeResponse(
            status_code=404,
            payload={"success": False, "error": {"code": "NOT_FOUND"}},
        )
    )
    assert client.audit_entry(999) is None


def test_create_dsr_camel_cases(session, client):
    from quidnug.types import DataSubjectRequest

    session.queue(_FakeResponse(payload={"success": True, "data": {"id": "tx"}}))
    req = DataSubjectRequest(
        subject_quid="sub",
        request_type="ERASURE",
        nonce=1,
        contact_email="a@b",
        jurisdiction="EU",
    )
    client.create_dsr(req)
    body = json.loads(session.calls[0][2]["data"])
    assert body["subjectQuid"] == "sub"
    assert body["requestType"] == "ERASURE"
    assert body["contactEmail"] == "a@b"
    assert body["jurisdiction"] == "EU"


def test_create_consent_grant_emits_camel_case_list(session, client):
    from quidnug.types import ConsentGrant

    session.queue(_FakeResponse(payload={"success": True, "data": {"id": "tx"}}))
    grant = ConsentGrant(
        subject_quid="sub",
        controller_quid="ctrl",
        scope=["MARKETING", "ANALYTICS"],
        nonce=1,
        policy_url="https://example/policy",
        policy_hash="abc",
    )
    client.create_consent_grant(grant)
    body = json.loads(session.calls[0][2]["data"])
    assert body["scope"] == ["MARKETING", "ANALYTICS"]
    assert body["policyUrl"] == "https://example/policy"
    assert body["policyHash"] == "abc"


def test_get_consent_history_uses_subject_query_param(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {"entries": []}}))
    client.get_consent_history("subq")
    _, url, kw = session.calls[0]
    assert url.endswith("/api/privacy/consent/history")
    assert kw["params"] == {"subject": "subq"}


def test_get_restrictions_path_segment(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {"restrictedUses": []}}))
    client.get_restrictions_for_subject("subq")
    _, url, _ = session.calls[0]
    assert url.endswith("/api/privacy/restrictions/subq")


def test_discover_endpoints_use_v2_prefix(session, client):
    for _ in range(5):
        session.queue(_FakeResponse(payload={"success": True, "data": {}}))
    client.discover_domain("foo")
    client.discover_node("nodeq")
    client.discover_operator("opq")
    client.discover_quids()
    client.discover_trusted_quids()
    paths = [u for _, u, _ in session.calls]
    assert paths[0].endswith("/api/v2/discovery/domain/foo")
    assert paths[1].endswith("/api/v2/discovery/node/nodeq")
    assert paths[2].endswith("/api/v2/discovery/operator/opq")
    assert paths[3].endswith("/api/v2/discovery/quids")
    assert paths[4].endswith("/api/v2/discovery/trusted-quids")


def test_submit_dns_claim_camel_cases(session, client):
    from quidnug.types import DNSClaim

    session.queue(_FakeResponse(payload={"success": True, "data": {"id": "tx"}}))
    claim = DNSClaim(
        domain="example.com",
        owner_quid="own",
        root_quid="root",
        nonce=1,
        requested_valid_until=12345,
        payment_method="stripe",
        payment_reference="ref",
        contact_email="a@b",
    )
    client.submit_dns_claim(claim)
    _, url, kw = session.calls[0]
    assert url.endswith("/api/v2/dns/claim")
    body = json.loads(kw["data"])
    assert body["domain"] == "example.com"
    assert body["ownerQuid"] == "own"
    assert body["rootQuid"] == "root"
    assert body["requestedValidUntil"] == 12345
    assert body["paymentMethod"] == "stripe"


def test_resolve_dns_record_path(session, client):
    session.queue(_FakeResponse(payload={"success": True, "data": {}}))
    client.resolve_dns_record("example.com", "A")
    _, url, _ = session.calls[0]
    assert url.endswith("/api/v2/dns/resolve/example.com/A")
