//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{
    DomainFingerprint, Event, GuardianSet, IdentityRecord, NonceSnapshot, Title, TrustEdge,
    TrustResult,
};
use crate::wire::{IdentityTx, TrustTx};
use reqwest::{Client as HttpClient, StatusCode};
use serde::de::DeserializeOwned;
use serde_json::Value;
use std::time::Duration;

/// Async Quidnug HTTP client.
#[derive(Clone, Debug)]
pub struct Client {
    http: HttpClient,
    api_base: String,
    timeout: Duration,
}

/// Parameters for [`Client::grant_trust`].
#[derive(Debug, Clone)]
pub struct TrustParams<'a> {
    /// Trustee quid ID.
    pub trustee: &'a str,
    /// Trust level in [0, 1].
    pub level: f64,
    /// Trust domain.
    pub domain: &'a str,
    /// Monotonic nonce.
    pub nonce: i64,
}

impl Client {
    /// Construct a new client against `base_url` (e.g. `http://localhost:8080`).
    pub fn new(base_url: &str) -> Result<Self> {
        let trimmed = base_url.trim_end_matches('/');
        let api_base = format!("{trimmed}/api");
        let http = HttpClient::builder()
            .timeout(Duration::from_secs(30))
            .build()?;
        Ok(Self {
            http,
            api_base,
            timeout: Duration::from_secs(30),
        })
    }

    /// Health check (GET /api/health).
    pub async fn health(&self) -> Result<Value> {
        self.get("health").await
    }

    /// Info (GET /api/info).
    pub async fn info(&self) -> Result<Value> {
        self.get("info").await
    }

    /// Register an identity for `signer`, optionally with `name` and `home_domain`.
    ///
    /// v1.0 conformant: builds a typed `IdentityTx` wire struct
    /// whose field order matches `core.IdentityTransaction`,
    /// derives the tx ID using the same seed the server uses,
    /// signs with IEEE-1363, submits.
    pub async fn register_identity(
        &self,
        signer: &Quid,
        name: &str,
        home_domain: &str,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        let mut tx = IdentityTx {
            id: String::new(),
            tx_type: "IDENTITY",
            trust_domain: "default",
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            quid_id: signer.id(),
            name,
            description: "",
            attributes: None,
            creator: signer.id(),
            update_nonce: 1,
            home_domain,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/identity", &body).await
    }

    /// Fetch an identity record or `None` on 404.
    pub async fn get_identity(
        &self,
        quid_id: &str,
        domain: &str,
    ) -> Result<Option<IdentityRecord>> {
        let mut path = format!("identity/{}", urlencoding(quid_id));
        if !domain.is_empty() {
            path.push_str(&format!("?domain={}", urlencoding(domain)));
        }
        match self.get_typed::<IdentityRecord>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Submit a signed TRUST transaction.
    ///
    /// v1.0 conformant: typed `TrustTx` wire struct + IEEE-1363
    /// signature + server-compatible ID derivation.
    pub async fn grant_trust<'a>(&self, signer: &Quid, p: TrustParams<'a>) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if !(0.0..=1.0).contains(&p.level) {
            return Err(Error::validation("level must be in [0, 1]"));
        }
        let mut tx = TrustTx {
            id: String::new(),
            tx_type: "TRUST",
            trust_domain: p.domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            truster: signer.id(),
            trustee: p.trustee,
            trust_level: p.level,
            nonce: p.nonce,
            description: "",
            valid_until: 0,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/trust", &body).await
    }

    /// Query relational trust.
    pub async fn get_trust(
        &self,
        observer: &str,
        target: &str,
        domain: &str,
        max_depth: u32,
    ) -> Result<TrustResult> {
        let path = format!(
            "trust/{}/{}?domain={}&maxDepth={}",
            urlencoding(observer),
            urlencoding(target),
            urlencoding(domain),
            max_depth
        );
        self.get_typed(&path).await
    }

    /// Fetch a title or `None` on 404.
    pub async fn get_title(&self, asset_id: &str, domain: &str) -> Result<Option<Title>> {
        let mut path = format!("title/{}", urlencoding(asset_id));
        if !domain.is_empty() {
            path.push_str(&format!("?domain={}", urlencoding(domain)));
        }
        match self.get_typed::<Title>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Get a quid's direct outbound trust edges.
    // --- Domain helpers -----------------------------------------------
    //
    // Every non-default trust domain must be registered with the
    // node before any identity / trust / title / event tx in that
    // domain will be accepted. `ensure_domain` is the idempotent
    // bootstrap helper to call once during app startup.

    /// Register a new trust domain. Fails with an "already exists"
    /// error if the domain is already known; see [`ensure_domain`]
    /// for an idempotent wrapper.
    pub async fn register_domain(&self, domain: &str) -> Result<Value> {
        let body = serde_json::json!({ "name": domain });
        self.post("domains", &body).await
    }

    /// Idempotent domain registration. Returns normally on both
    /// fresh-register and already-exists; propagates any other
    /// error.
    pub async fn ensure_domain(&self, domain: &str) -> Result<Value> {
        match self.register_domain(domain).await {
            Ok(v) => Ok(v),
            Err(e) => {
                let msg = format!("{}", e).to_lowercase();
                if msg.contains("already exists") {
                    Ok(serde_json::json!({
                        "status": "success",
                        "domain": domain,
                        "message": "trust domain already exists",
                    }))
                } else {
                    Err(e)
                }
            }
        }
    }

    // --- Commit-wait helpers ------------------------------------------
    //
    // Identity and title transactions live in the node's pending
    // pool until the next block is sealed. Code that immediately
    // emits events or title transactions referencing the new quid
    // must wait for commit first; these helpers poll until the
    // record is visible in the committed registry.

    /// Block until the identity with `quid_id` is visible in the
    /// committed registry, or return `Error::Timeout` when the
    /// caller-supplied deadline expires.
    pub async fn wait_for_identity(
        &self,
        quid_id: &str,
        domain: &str,
        timeout: std::time::Duration,
        poll_interval: std::time::Duration,
    ) -> Result<IdentityRecord> {
        let deadline = std::time::Instant::now() + timeout;
        loop {
            if let Some(rec) = self.get_identity(quid_id, domain).await? {
                return Ok(rec);
            }
            if std::time::Instant::now() >= deadline {
                return Err(Error::validation(format!(
                    "identity {} did not commit within {:?}",
                    quid_id, timeout
                )));
            }
            tokio::time::sleep(poll_interval).await;
        }
    }

    /// Block until every listed quid_id is committed, sharing one
    /// total deadline across the whole batch.
    pub async fn wait_for_identities(
        &self,
        quid_ids: &[&str],
        domain: &str,
        timeout: std::time::Duration,
        poll_interval: std::time::Duration,
    ) -> Result<()> {
        let deadline = std::time::Instant::now() + timeout;
        for qid in quid_ids {
            let remaining = deadline.saturating_duration_since(std::time::Instant::now());
            if remaining.is_zero() {
                return Err(Error::validation(format!(
                    "identities not all committed within {:?} (blocked on {})",
                    timeout, qid
                )));
            }
            self.wait_for_identity(qid, domain, remaining, poll_interval)
                .await?;
        }
        Ok(())
    }

    /// Block until the title with `asset_id` is visible in the
    /// committed registry. Analogous to
    /// [`wait_for_identity`][Self::wait_for_identity]; required
    /// before emitting events on a freshly-registered title.
    pub async fn wait_for_title(
        &self,
        asset_id: &str,
        domain: &str,
        timeout: std::time::Duration,
        poll_interval: std::time::Duration,
    ) -> Result<Title> {
        let deadline = std::time::Instant::now() + timeout;
        loop {
            if let Some(t) = self.get_title(asset_id, domain).await? {
                return Ok(t);
            }
            if std::time::Instant::now() >= deadline {
                return Err(Error::validation(format!(
                    "title {} did not commit within {:?}",
                    asset_id, timeout
                )));
            }
            tokio::time::sleep(poll_interval).await;
        }
    }

    /// Returns the outbound trust edges (subject → object) anchored
    /// at `quid_id`. The node may envelope the result either at the
    /// canonical `edges` field or inside the generic `data` field
    /// of the response envelope; both shapes are tolerated for
    /// forward compatibility with intermediate node releases.
    pub async fn get_trust_edges(&self, quid_id: &str) -> Result<Vec<TrustEdge>> {
        #[derive(serde::Deserialize)]
        struct Wrap {
            #[serde(default)]
            edges: Vec<TrustEdge>,
            #[serde(default)]
            data: Vec<TrustEdge>,
        }
        let w: Wrap = self
            .get_typed(&format!("trust/edges/{}", urlencoding(quid_id)))
            .await?;
        if !w.edges.is_empty() {
            Ok(w.edges)
        } else {
            Ok(w.data)
        }
    }

    // --- Node / blocks / registry queries -----------------------------

    /// List known nodes (GET /api/nodes).
    pub async fn nodes(&self) -> Result<Value> {
        self.get("nodes").await
    }

    /// List blocks (GET /api/blocks).
    pub async fn get_blocks(&self) -> Result<Value> {
        self.get("blocks").await
    }

    /// Tentative blocks for a domain (GET /api/blocks/tentative/{domain}).
    pub async fn get_tentative_blocks(&self, domain: &str) -> Result<Value> {
        self.get(&format!("blocks/tentative/{}", urlencoding(domain)))
            .await
    }

    /// Pending transaction pool (GET /api/transactions).
    pub async fn get_pending_transactions(&self) -> Result<Value> {
        self.get("transactions").await
    }

    /// Trust registry snapshot (GET /api/registry/trust).
    pub async fn query_trust_registry(&self) -> Result<Value> {
        self.get("registry/trust").await
    }

    /// Identity registry snapshot (GET /api/registry/identity).
    pub async fn query_identity_registry(&self) -> Result<Value> {
        self.get("registry/identity").await
    }

    /// Title registry snapshot (GET /api/registry/title).
    pub async fn query_title_registry(&self) -> Result<Value> {
        self.get("registry/title").await
    }

    /// POST /api/trust/query — bulk relational trust query.
    ///
    /// Body should match the server's `TrustQuery` envelope (observer,
    /// target, domain, maxDepth). Returns a [`TrustResult`].
    pub async fn query_relational_trust(&self, query: &Value) -> Result<TrustResult> {
        let raw = self.post("trust/query", query).await?;
        serde_json::from_value(raw).map_err(Error::from)
    }

    // --- Domain management --------------------------------------------

    /// List registered trust domains (GET /api/domains).
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    /// Domains this node manages (GET /api/node/domains).
    pub async fn get_node_domains(&self) -> Result<Value> {
        self.get("node/domains").await
    }

    /// Replace this node's managed-domains list (POST /api/node/domains).
    pub async fn update_node_domains(&self, body: &Value) -> Result<Value> {
        self.post("node/domains", body).await
    }

    // --- Events --------------------------------------------------------

    /// Submit a fully-signed `EVENT` transaction (POST /api/events).
    ///
    /// The caller is responsible for assembling and signing the event
    /// envelope; this method handles transport only. For a typed
    /// event-emit helper, see the higher-level builder under
    /// `clients/python/quidnug/client.py::emit_event`.
    pub async fn emit_event(&self, event: &Value) -> Result<Value> {
        self.post("events", event).await
    }

    /// Get a subject's event-stream header (GET /api/streams/{subjectId}).
    pub async fn get_event_stream(&self, subject_id: &str) -> Result<Option<Value>> {
        match self.get(&format!("streams/{}", urlencoding(subject_id))).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Get events from a subject's stream (GET /api/streams/{subjectId}/events).
    ///
    /// `limit` / `offset` are forwarded as query parameters when non-zero.
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        limit: u32,
        offset: u32,
    ) -> Result<Vec<Event>> {
        let mut path = format!("streams/{}/events", urlencoding(subject_id));
        let mut q = Vec::new();
        if limit > 0 {
            q.push(format!("limit={}", limit));
        }
        if offset > 0 {
            q.push(format!("offset={}", offset));
        }
        if !q.is_empty() {
            path.push('?');
            path.push_str(&q.join("&"));
        }
        #[derive(serde::Deserialize)]
        struct Wrap {
            #[serde(default)]
            events: Vec<Event>,
            #[serde(default)]
            data: Vec<Event>,
        }
        let w: Wrap = self.get_typed(&path).await?;
        if !w.events.is_empty() {
            Ok(w.events)
        } else {
            Ok(w.data)
        }
    }

    // --- IPFS ----------------------------------------------------------

    /// Pin a payload to the node's IPFS backend (POST /api/ipfs/pin).
    /// Returns the IPFS CID in the response.
    pub async fn ipfs_pin(&self, body: &Value) -> Result<Value> {
        self.post("ipfs/pin", body).await
    }

    /// Fetch an IPFS-pinned payload by CID (GET /api/ipfs/{cid}).
    pub async fn ipfs_get(&self, cid: &str) -> Result<Value> {
        self.get(&format!("ipfs/{}", urlencoding(cid))).await
    }

    // --- Guardian sets + recovery (QDP-0002, QDP-0006) ----------------
    //
    // These take pre-signed JSON envelopes — the caller assembles and
    // signs the envelope with the appropriate quorum, the SDK handles
    // transport + error translation only.

    /// Install or rotate a guardian set (POST /api/guardian/set-update).
    pub async fn submit_guardian_set_update(&self, update: &Value) -> Result<Value> {
        self.post("guardian/set-update", update).await
    }

    /// Initiate a guardian-quorum recovery (POST /api/guardian/recovery/init).
    pub async fn submit_recovery_init(&self, init: &Value) -> Result<Value> {
        self.post("guardian/recovery/init", init).await
    }

    /// Veto an in-flight recovery during the time-lock window
    /// (POST /api/guardian/recovery/veto).
    pub async fn submit_recovery_veto(&self, veto: &Value) -> Result<Value> {
        self.post("guardian/recovery/veto", veto).await
    }

    /// Commit a recovery after the time-lock elapses
    /// (POST /api/guardian/recovery/commit).
    pub async fn submit_recovery_commit(&self, commit: &Value) -> Result<Value> {
        self.post("guardian/recovery/commit", commit).await
    }

    /// Guardian withdraws consent (POST /api/guardian/resign).
    pub async fn submit_guardian_resignation(&self, resignation: &Value) -> Result<Value> {
        self.post("guardian/resign", resignation).await
    }

    /// Fetch the current guardian set or `None` on 404.
    pub async fn get_guardian_set(&self, quid_id: &str) -> Result<Option<GuardianSet>> {
        match self
            .get_typed::<GuardianSet>(&format!("guardian/set/{}", urlencoding(quid_id)))
            .await
        {
            Ok(g) => Ok(Some(g)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Fetch a pending recovery (GET /api/guardian/pending-recovery/{quid}).
    /// Returns `None` when no recovery is in flight.
    pub async fn get_pending_recovery(&self, quid_id: &str) -> Result<Option<Value>> {
        match self
            .get(&format!("guardian/pending-recovery/{}", urlencoding(quid_id)))
            .await
        {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Fetch all guardian resignations for a subject
    /// (GET /api/guardian/resignations/{quid}).
    pub async fn get_guardian_resignations(&self, quid_id: &str) -> Result<Vec<Value>> {
        let raw = self
            .get(&format!("guardian/resignations/{}", urlencoding(quid_id)))
            .await?;
        if let Some(arr) = raw.get("resignations").and_then(|v| v.as_array()) {
            return Ok(arr.clone());
        }
        if let Some(arr) = raw.as_array() {
            return Ok(arr.clone());
        }
        Ok(Vec::new())
    }

    // --- Cross-domain gossip (QDP-0003, QDP-0005) ---------------------

    /// Submit a domain fingerprint (POST /api/domain-fingerprints).
    pub async fn submit_domain_fingerprint(&self, fingerprint: &Value) -> Result<Value> {
        self.post("domain-fingerprints", fingerprint).await
    }

    /// Latest domain fingerprint for `domain`
    /// (GET /api/domain-fingerprints/{domain}/latest).
    pub async fn get_latest_domain_fingerprint(
        &self,
        domain: &str,
    ) -> Result<Option<DomainFingerprint>> {
        match self
            .get_typed::<DomainFingerprint>(&format!(
                "domain-fingerprints/{}/latest",
                urlencoding(domain)
            ))
            .await
        {
            Ok(d) => Ok(Some(d)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Submit anchor gossip (POST /api/anchor-gossip).
    pub async fn submit_anchor_gossip(&self, message: &Value) -> Result<Value> {
        self.post("anchor-gossip", message).await
    }

    /// Push an anchor to a subscribed peer (POST /api/gossip/push-anchor).
    pub async fn push_anchor(&self, message: &Value) -> Result<Value> {
        self.post("gossip/push-anchor", message).await
    }

    /// Push a domain fingerprint to a peer
    /// (POST /api/gossip/push-fingerprint).
    pub async fn push_fingerprint(&self, fingerprint: &Value) -> Result<Value> {
        self.post("gossip/push-fingerprint", fingerprint).await
    }

    // --- K-of-K bootstrap (QDP-0008) ----------------------------------

    /// Submit a nonce snapshot (POST /api/nonce-snapshots).
    pub async fn submit_nonce_snapshot(&self, snapshot: &Value) -> Result<Value> {
        self.post("nonce-snapshots", snapshot).await
    }

    /// Latest nonce snapshot for a domain
    /// (GET /api/nonce-snapshots/{domain}/latest).
    pub async fn get_latest_nonce_snapshot(&self, domain: &str) -> Result<Option<NonceSnapshot>> {
        match self
            .get_typed::<NonceSnapshot>(&format!(
                "nonce-snapshots/{}/latest",
                urlencoding(domain)
            ))
            .await
        {
            Ok(s) => Ok(Some(s)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Bootstrap progress for the local node (GET /api/bootstrap/status).
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get("bootstrap/status").await
    }

    // --- Fork-block (QDP-0009) ----------------------------------------

    /// Submit a fork-block activation (POST /api/fork-block).
    pub async fn submit_fork_block(&self, fork_block: &Value) -> Result<Value> {
        self.post("fork-block", fork_block).await
    }

    /// Pending + active fork-block summary (GET /api/fork-block/status).
    pub async fn fork_block_status(&self) -> Result<Value> {
        self.get("fork-block/status").await
    }

    // --- Plumbing ------------------------------------------------------

    async fn get(&self, path: &str) -> Result<Value> {
        let url = format!("{}/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        parse_envelope(resp).await
    }

    async fn get_typed<T: DeserializeOwned>(&self, path: &str) -> Result<T> {
        let url = format!("{}/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        parse_envelope_typed(resp).await
    }

    async fn post(&self, path: &str, body: &Value) -> Result<Value> {
        let url = format!("{}/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .json(body)
            .send()
            .await?;
        parse_envelope(resp).await
    }
}

async fn parse_envelope(resp: reqwest::Response) -> Result<Value> {
    let status = resp.status();
    let body = resp.text().await?;
    let env: Value = match serde_json::from_str(&body) {
        Ok(v) => v,
        Err(_) => {
            return Err(Error::Node {
                status: status.as_u16(),
                message: format!("non-JSON response: {}", truncate(&body, 200)),
            });
        }
    };
    if env.get("success").and_then(|v| v.as_bool()).unwrap_or(false) {
        return Ok(env.get("data").cloned().unwrap_or(Value::Null));
    }
    Err(envelope_to_error(status, &env))
}

async fn parse_envelope_typed<T: DeserializeOwned>(resp: reqwest::Response) -> Result<T> {
    let value = parse_envelope(resp).await?;
    serde_json::from_value(value).map_err(Error::from)
}

fn envelope_to_error(status: StatusCode, env: &Value) -> Error {
    let code = env
        .pointer("/error/code")
        .and_then(|v| v.as_str())
        .unwrap_or("UNKNOWN_ERROR")
        .to_string();
    let message = env
        .pointer("/error/message")
        .and_then(|v| v.as_str())
        .unwrap_or("")
        .to_string();
    match status.as_u16() {
        503 => Error::Unavailable { code, message },
        409 => Error::Conflict { code, message },
        s if (400..500).contains(&s) => {
            if code == "NOT_FOUND" {
                Error::Validation(format!("{}: {}", code, message))
            } else if matches!(
                code.as_str(),
                "FEATURE_NOT_ACTIVE" | "NOT_READY" | "BOOTSTRAPPING"
            ) {
                Error::Unavailable { code, message }
            } else if is_conflict_code(&code) {
                Error::Conflict { code, message }
            } else {
                Error::Validation(message)
            }
        }
        s => Error::Node {
            status: s,
            message,
        },
    }
}

fn is_conflict_code(code: &str) -> bool {
    matches!(
        code,
        "NONCE_REPLAY"
            | "GUARDIAN_SET_MISMATCH"
            | "QUORUM_NOT_MET"
            | "VETOED"
            | "INVALID_SIGNATURE"
            | "FORK_ALREADY_ACTIVE"
            | "DUPLICATE"
            | "ALREADY_EXISTS"
            | "INVALID_STATE_TRANSITION"
    )
}

fn now_secs() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .unwrap_or(0)
}

fn urlencoding(s: &str) -> String {
    // Minimal percent-encoding for path/query segments. Avoids pulling
    // in a whole crate just for this.
    let mut out = String::with_capacity(s.len());
    for b in s.bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(b as char)
            }
            _ => out.push_str(&format!("%{:02X}", b)),
        }
    }
    out
}

fn truncate(s: &str, n: usize) -> String {
    if s.len() <= n {
        s.to_string()
    } else {
        format!("{}...", &s[..n])
    }
}
