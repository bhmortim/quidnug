//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{Event, EventStream, IdentityRecord, OwnershipStake, Title, TrustEdge, TrustResult};
use crate::wire::{EventTx, IdentityTx, TitleTx, TrustTx, WireOwnershipStake};
use reqwest::{Client as HttpClient, StatusCode};
use serde::de::DeserializeOwned;
use serde_json::Value;
use std::collections::BTreeMap;
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

    // --- Nodes / blocks / pending tx ----------------------------------

    /// List known peers (GET /api/nodes).
    pub async fn nodes(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        self.get(&with_pagination("nodes", limit, offset)).await
    }

    /// Recent committed blocks (GET /api/blocks).
    pub async fn get_blocks(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        self.get(&with_pagination("blocks", limit, offset)).await
    }

    /// Tentative blocks for a domain (GET /api/blocks/tentative/{domain}).
    pub async fn get_tentative_blocks(&self, domain: &str) -> Result<Value> {
        self.get(&format!("blocks/tentative/{}", urlencoding(domain)))
            .await
    }

    /// Pending (mempool) transactions (GET /api/transactions).
    pub async fn get_pending_transactions(
        &self,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        self.get(&with_pagination("transactions", limit, offset))
            .await
    }

    // --- Title --------------------------------------------------------

    /// Submit a TITLE transaction.
    ///
    /// `owners` percentages may be on either the fraction scale (sum ≈
    /// 1.0) or the percent scale (sum ≈ 100.0); they are normalized to
    /// fraction for the wire (the server's invariant is sum == 1.0).
    pub async fn register_title(
        &self,
        signer: &Quid,
        asset_id: &str,
        owners: &[OwnershipStake],
        domain: &str,
        title_type: Option<&str>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if asset_id.is_empty() {
            return Err(Error::validation("asset_id is required"));
        }
        let normalized = OwnershipStake::normalize_percentages(owners)?;
        let wire_owners: Vec<WireOwnershipStake> = normalized
            .iter()
            .map(|s| WireOwnershipStake {
                owner_id: &s.owner_id,
                percentage: s.percentage,
                stake_type: s.stake_type.as_deref().unwrap_or(""),
            })
            .collect();
        let mut tx = TitleTx {
            id: String::new(),
            tx_type: "TITLE",
            trust_domain: domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            asset_id,
            owners: wire_owners,
            previous_owners: Vec::new(),
            signatures: BTreeMap::new(),
            expiry_date: 0,
            title_type: title_type.unwrap_or(""),
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/title", &body).await
    }

    // --- Trust queries / registries -----------------------------------

    /// Structured relational trust query (POST /api/trust/query).
    pub async fn query_relational_trust(
        &self,
        observer: &str,
        target: &str,
        domain: &str,
        max_depth: u32,
    ) -> Result<TrustResult> {
        let body = serde_json::json!({
            "observer": observer,
            "target": target,
            "domain": domain,
            "maxDepth": max_depth,
        });
        let raw = self.post("trust/query", &body).await?;
        serde_json::from_value(raw).map_err(Error::from)
    }

    /// Paginated trust-edge listing (GET /api/registry/trust).
    pub async fn query_trust_registry(
        &self,
        truster: Option<&str>,
        trustee: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let mut q = QueryBuilder::new("registry/trust");
        q.opt_str("truster", truster);
        q.opt_str("trustee", trustee);
        q.opt_u32("limit", limit);
        q.opt_u32("offset", offset);
        self.get(&q.finish()).await
    }

    /// Paginated identity-registry dump or single lookup
    /// (GET /api/registry/identity).
    pub async fn query_identity_registry(
        &self,
        quid_id: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let mut q = QueryBuilder::new("registry/identity");
        q.opt_str("quid_id", quid_id);
        q.opt_u32("limit", limit);
        q.opt_u32("offset", offset);
        self.get(&q.finish()).await
    }

    /// Paginated title-registry dump or single lookup
    /// (GET /api/registry/title).
    pub async fn query_title_registry(
        &self,
        asset_id: Option<&str>,
        owner_id: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let mut q = QueryBuilder::new("registry/title");
        q.opt_str("asset_id", asset_id);
        q.opt_str("owner_id", owner_id);
        q.opt_u32("limit", limit);
        q.opt_u32("offset", offset);
        self.get(&q.finish()).await
    }

    // --- Domains ------------------------------------------------------

    /// List all known trust domains (GET /api/domains).
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    /// Query a domain by type+param (GET /api/domains/{name}/query).
    ///
    /// The query is forwarded across the hierarchy if the domain isn't
    /// local to this node.
    pub async fn query_domain(
        &self,
        name: &str,
        query_type: &str,
        param: &str,
    ) -> Result<Value> {
        let path = format!(
            "domains/{}/query?type={}&param={}",
            urlencoding(name),
            urlencoding(query_type),
            urlencoding(param),
        );
        self.get(&path).await
    }

    /// Returns the domains this node currently manages
    /// (GET /api/node/domains).
    pub async fn get_node_domains(&self) -> Result<Value> {
        self.get("node/domains").await
    }

    /// Replace the set of domains this node manages
    /// (POST /api/node/domains).
    pub async fn update_node_domains(&self, domains: &[&str]) -> Result<Value> {
        let body = serde_json::json!({ "managedDomains": domains });
        self.post("node/domains", &body).await
    }

    /// Deliver a node-to-node domain gossip message
    /// (POST /api/gossip/domains).
    pub async fn receive_domain_gossip(&self, message: &Value) -> Result<Value> {
        self.post("gossip/domains", message).await
    }

    // --- Events -------------------------------------------------------

    /// Submit an EVENT transaction. Signer must own `subject_id`.
    ///
    /// Exactly one of `payload` (inline JSON) or `payload_cid` (IPFS
    /// reference) must be supplied. If `sequence` is `None`, the
    /// client fetches the current stream's `latest_sequence` and uses
    /// `latest_sequence + 1` — falling back to `1` if no stream
    /// exists yet.
    #[allow(clippy::too_many_arguments)]
    pub async fn emit_event(
        &self,
        signer: &Quid,
        subject_id: &str,
        subject_type: &str,
        event_type: &str,
        domain: &str,
        payload: Option<Value>,
        payload_cid: Option<&str>,
        sequence: Option<i64>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if subject_type != "QUID" && subject_type != "TITLE" {
            return Err(Error::validation("subject_type must be 'QUID' or 'TITLE'"));
        }
        if event_type.is_empty() {
            return Err(Error::validation("event_type is required"));
        }
        if payload.is_some() == payload_cid.is_some() {
            return Err(Error::validation(
                "exactly one of payload or payload_cid is required",
            ));
        }

        let seq = match sequence {
            Some(s) => s,
            None => match self.get_event_stream(subject_id, domain).await {
                Ok(Some(s)) => s.latest_sequence + 1,
                _ => 1,
            },
        };

        let mut tx = EventTx {
            id: String::new(),
            tx_type: "EVENT",
            trust_domain: domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            subject_id,
            subject_type,
            sequence: seq,
            event_type,
            payload,
            payload_cid: payload_cid.unwrap_or(""),
            previous_event_id: "",
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("events", &body).await
    }

    /// Stream metadata or `None` on 404 (GET /api/streams/{subject}).
    pub async fn get_event_stream(
        &self,
        subject_id: &str,
        domain: &str,
    ) -> Result<Option<EventStream>> {
        let mut path = format!("streams/{}", urlencoding(subject_id));
        if !domain.is_empty() {
            path.push_str(&format!("?domain={}", urlencoding(domain)));
        }
        match self.get_typed::<EventStream>(&path).await {
            Ok(s) => Ok(Some(s)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Read events from a stream
    /// (GET /api/streams/{subject}/events). Returns `(events,
    /// pagination)` where the pagination map is whatever the node
    /// emits (typically `limit`/`offset`/`total`).
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        domain: &str,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<(Vec<Event>, Value)> {
        let mut q = QueryBuilder::new(&format!("streams/{}/events", urlencoding(subject_id)));
        if !domain.is_empty() {
            q.opt_str("domain", Some(domain));
        }
        q.opt_u32("limit", limit);
        q.opt_u32("offset", offset);
        let raw = self.get(&q.finish()).await?;
        let events: Vec<Event> = match raw.get("data").or_else(|| raw.get("events")) {
            Some(arr) => serde_json::from_value(arr.clone()).unwrap_or_default(),
            None => Vec::new(),
        };
        let pagination = raw.get("pagination").cloned().unwrap_or(Value::Null);
        Ok((events, pagination))
    }

    // --- IPFS ---------------------------------------------------------

    /// Pin raw bytes and return the CID (POST /api/ipfs/pin).
    pub async fn ipfs_pin(&self, content: &[u8]) -> Result<String> {
        let url = format!("{}/ipfs/pin", self.api_base);
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .header("Content-Type", "application/octet-stream")
            .body(content.to_vec())
            .send()
            .await?;
        let data = parse_envelope(resp).await?;
        let cid = data
            .get("cid")
            .or_else(|| data.get("value"))
            .and_then(|v| v.as_str())
            .ok_or_else(|| Error::UnexpectedResponse("IPFS pin response missing cid".into()))?;
        Ok(cid.to_string())
    }

    /// Fetch raw bytes by CID (GET /api/ipfs/{cid}).
    pub async fn ipfs_get(&self, cid: &str) -> Result<Vec<u8>> {
        let url = format!("{}/ipfs/{}", self.api_base, urlencoding(cid));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        if status.is_client_error() || status.is_server_error() {
            let body = resp.text().await.unwrap_or_default();
            return Err(Error::Node {
                status: status.as_u16(),
                message: truncate(&body, 200),
            });
        }
        Ok(resp.bytes().await?.to_vec())
    }

    // --- Quid management ---------------------------------------------

    /// Server-side keypair generation (POST /api/quids). Returns the
    /// envelope's `data` field with `quidId`, `publicKey`, `created`.
    ///
    /// The server holds no private material on the caller's behalf —
    /// this endpoint is for environments without local crypto.
    pub async fn create_quid(&self) -> Result<Value> {
        let body = serde_json::json!({});
        self.post("quids", &body).await
    }

    // --- Metrics ------------------------------------------------------

    /// Prometheus metrics in text-exposition format (GET /metrics).
    ///
    /// Note: this hits `/metrics` directly, not `/api/metrics`.
    pub async fn metrics(&self) -> Result<String> {
        let base = self.api_base.trim_end_matches("/api");
        let url = format!("{base}/metrics");
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        let body = resp.text().await?;
        if status.is_client_error() || status.is_server_error() {
            return Err(Error::Node {
                status: status.as_u16(),
                message: truncate(&body, 200),
            });
        }
        Ok(body)
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

fn with_pagination(base: &str, limit: Option<u32>, offset: Option<u32>) -> String {
    let mut q = QueryBuilder::new(base);
    q.opt_u32("limit", limit);
    q.opt_u32("offset", offset);
    q.finish()
}

/// Builds a path with `?k=v&k=v` query params from optional values,
/// skipping anything not set. Used so paginated GETs match the
/// Python client's `_strip_none` behavior.
struct QueryBuilder {
    out: String,
    has_q: bool,
}

impl QueryBuilder {
    fn new(base: &str) -> Self {
        Self {
            out: base.to_string(),
            has_q: false,
        }
    }

    fn opt_str(&mut self, key: &str, value: Option<&str>) {
        if let Some(v) = value {
            self.push(key, &urlencoding(v));
        }
    }

    fn opt_u32(&mut self, key: &str, value: Option<u32>) {
        if let Some(v) = value {
            self.push(key, &v.to_string());
        }
    }

    fn push(&mut self, key: &str, value: &str) {
        if self.has_q {
            self.out.push('&');
        } else {
            self.out.push('?');
            self.has_q = true;
        }
        self.out.push_str(key);
        self.out.push('=');
        self.out.push_str(value);
    }

    fn finish(self) -> String {
        self.out
    }
}
