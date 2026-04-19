//! Error taxonomy mirroring the Python and Go SDKs.

use thiserror::Error;

/// Result alias used throughout the SDK.
pub type Result<T> = std::result::Result<T, Error>;

/// All errors raised by the SDK. Consumers can match on each variant
/// or just bubble via `?`.
#[derive(Debug, Error)]
pub enum Error {
    /// Local precondition failed before any network call.
    #[error("validation: {0}")]
    Validation(String),

    /// Node logically rejected a transaction (nonce replay, quorum
    /// failure, etc.).
    #[error("conflict [{code}]: {message}")]
    Conflict {
        /// Server-provided error code (e.g. `NONCE_REPLAY`).
        code: String,
        /// Server-provided message.
        message: String,
    },

    /// Node reported unavailable (503) or feature-not-active.
    #[error("unavailable [{code}]: {message}")]
    Unavailable {
        /// Server-provided error code.
        code: String,
        /// Server-provided message.
        message: String,
    },

    /// Transport / HTTP error.
    #[error("node error (HTTP {status}): {message}")]
    Node {
        /// HTTP status code (0 for transport-level failures).
        status: u16,
        /// Description.
        message: String,
    },

    /// Crypto / key / signature failure.
    #[error("crypto: {0}")]
    Crypto(String),

    /// Unexpected JSON shape.
    #[error("unexpected response: {0}")]
    UnexpectedResponse(String),

    /// Underlying reqwest error.
    #[error("http transport: {0}")]
    Transport(#[from] reqwest::Error),

    /// JSON parse error.
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),

    /// URL parse error.
    #[error("url: {0}")]
    Url(#[from] url::ParseError),
}

impl Error {
    /// Convenience constructor for validation errors.
    pub fn validation(msg: impl Into<String>) -> Self {
        Error::Validation(msg.into())
    }

    /// Convenience constructor for crypto errors.
    pub fn crypto(msg: impl Into<String>) -> Self {
        Error::Crypto(msg.into())
    }
}
