"""Structured exception hierarchy for the Quidnug SDK.

All SDK-raised errors inherit from `QuidnugError`. Catch-all of
`QuidnugError` is safe — no other exceptions escape the SDK
intentionally (network-level `requests.RequestException` is
re-wrapped as `NodeError`).
"""

from __future__ import annotations

from typing import Any, Optional


class QuidnugError(Exception):
    """Base class for all SDK-raised errors."""

    def __init__(self, message: str, *, details: Optional[dict[str, Any]] = None) -> None:
        super().__init__(message)
        self.message = message
        self.details = details or {}

    def __repr__(self) -> str:
        return f"{type(self).__name__}({self.message!r}, details={self.details!r})"


class ValidationError(QuidnugError):
    """Raised when local validation catches a malformed request before sending.

    Raised before any network activity. Safe to retry with corrected input.
    """


class ConflictError(QuidnugError):
    """Raised when the node rejects a transaction for logical reasons.

    Examples: nonce replay, duplicate voter check-in, guardian-set-hash
    mismatch, quorum not met. HTTP 409 and some 400-class responses map
    here. Retrying without changes will not succeed.
    """


class UnavailableError(QuidnugError):
    """Raised when the node reports itself unavailable (HTTP 503) or the
    feature is gated behind a not-yet-activated flag."""


class NodeError(QuidnugError):
    """Raised for network / transport failures and unexpected HTTP errors.

    Wraps `requests.RequestException`, connect timeouts, 5xx responses.
    Retry logic in `QuidnugClient` already handles transient cases;
    this exception bubbles up only when retries are exhausted.
    """

    def __init__(
        self,
        message: str,
        *,
        status_code: Optional[int] = None,
        response_body: Optional[str] = None,
    ) -> None:
        details: dict[str, Any] = {}
        if status_code is not None:
            details["status_code"] = status_code
        if response_body is not None:
            details["response_body"] = response_body[:500]
        super().__init__(message, details=details)
        self.status_code = status_code
        self.response_body = response_body


class CryptoError(QuidnugError):
    """Raised for signature verification / key derivation failures."""


__all__ = [
    "QuidnugError",
    "ValidationError",
    "ConflictError",
    "UnavailableError",
    "NodeError",
    "CryptoError",
]
