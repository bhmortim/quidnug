//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{IdentityRecord, Title, TrustEdge, TrustResult};
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

    // --- Health / info / peers ----------------------------------------

    /// GET /api/nodes — list known peers.
    pub async fn nodes(&self) -> Result<Value> {
        self.get("nodes").await
    }

    /// GET /api/metrics — Prometheus exposition format (text, not JSON).
    pub async fn get_metrics(&self) -> Result<String> {
        let base = self.api_base.trim_end_matches("/api");
        let url = format!("{}/metrics", base);
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        if !resp.status().is_success() {
            return Err(Error::Node {
                status: resp.status().as_u16(),
                message: format!("metrics: HTTP {}", resp.status()),
            });
        }
        Ok(resp.text().await?)
    }

    // --- Blocks + transactions ----------------------------------------

    /// GET /api/blocks — recent blocks.
    pub async fn get_blocks(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let mut path = String::from("blocks");
        let mut q = String::new();
        if let Some(l) = limit { q.push_str(&format!("&limit={}", l)); }
        if let Some(o) = offset { q.push_str(&format!("&offset={}", o)); }
        if !q.is_empty() { path.push('?'); path.push_str(&q[1..]); }
        self.get(&path).await
    }

    /// GET /api/blocks/tentative/{domain} — proposed but uncommitted blocks.
    pub async fn get_tentative_blocks(&self, domain: &str) -> Result<Value> {
        self.get(&format!("blocks/tentative/{}", urlencoding(domain))).await
    }

    /// GET /api/transactions — pending transactions in the mempool.
    pub async fn get_transactions(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let mut path = String::from("transactions");
        let mut q = String::new();
        if let Some(l) = limit { q.push_str(&format!("&limit={}", l)); }
        if let Some(o) = offset { q.push_str(&format!("&offset={}", o)); }
        if !q.is_empty() { path.push('?'); path.push_str(&q[1..]); }
        self.get(&path).await
    }

    // --- Title registration -------------------------------------------

    /// POST /api/transactions/title — submit a signed TITLE transaction.
    ///
    /// `owners` percentages must sum to 100.0 (server invariant).
    pub async fn register_title(
        &self,
        signer: &Quid,
        asset_id: &str,
        owners: Vec<crate::types::OwnershipStake>,
        domain: &str,
        title_type: Option<&str>,
        prev_title_tx_id: Option<&str>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if asset_id.is_empty() {
            return Err(Error::validation("asset_id is required"));
        }
        if owners.is_empty() {
            return Err(Error::validation("owners is required"));
        }
        let total: f64 = owners.iter().map(|s| s.percentage).sum();
        if (total - 100.0).abs() > 0.001 {
            return Err(Error::validation(&format!(
                "owner percentages must sum to 100 (got {})", total
            )));
        }

        let mut tx = serde_json::json!({
            "type": "TITLE",
            "timestamp": now_secs(),
            "trustDomain": domain,
            "signerQuid": signer.id(),
            "issuerQuid": signer.id(),
            "assetQuid": asset_id,
            "ownershipMap": owners,
            "transferSigs": {},
        });
        if let Some(t) = title_type {
            tx["titleType"] = serde_json::json!(t);
        }
        if let Some(p) = prev_title_tx_id {
            tx["prevTitleTxID"] = serde_json::json!(p);
        }

        let signable = crate::canonical::canonical_bytes(&tx, &["signature", "txId"])?;
        tx["signature"] = serde_json::json!(signer.sign(&signable)?);
        self.post("transactions/title", &tx).await
    }

    // --- Events + streams ---------------------------------------------

    /// POST /api/events — submit a signed EVENT transaction.
    ///
    /// Exactly one of `payload` (inline JSON) or `payload_cid` (IPFS pin) must be provided.
    /// If `sequence` is 0, the client auto-fetches the next sequence from the stream.
    #[allow(clippy::too_many_arguments)]
    pub async fn emit_event(
        &self,
        signer: &Quid,
        subject_id: &str,
        subject_type: &str,
        event_type: &str,
        domain: &str,
        payload: Option<serde_json::Map<String, Value>>,
        payload_cid: Option<&str>,
        sequence: i64,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if subject_type != "QUID" && subject_type != "TITLE" {
            return Err(Error::validation("subjectType must be 'QUID' or 'TITLE'"));
        }
        if event_type.is_empty() {
            return Err(Error::validation("eventType is required"));
        }
        if payload.is_some() == payload_cid.is_some() {
            return Err(Error::validation("exactly one of payload or payloadCid is required"));
        }

        let seq = if sequence > 0 {
            sequence
        } else {
            match self.get_event_stream(subject_id, domain).await {
                Ok(Some(stream)) => stream
                    .get("latestSequence")
                    .and_then(|v| v.as_i64())
                    .unwrap_or(0)
                    + 1,
                _ => 1,
            }
        };

        let mut tx = serde_json::json!({
            "type": "EVENT",
            "timestamp": now_secs(),
            "trustDomain": domain,
            "subjectId": subject_id,
            "subjectType": subject_type,
            "eventType": event_type,
            "sequence": seq,
        });
        if let Some(p) = payload {
            tx["payload"] = Value::Object(p);
        }
        if let Some(c) = payload_cid {
            tx["payloadCid"] = serde_json::json!(c);
        }

        let signable = crate::canonical::canonical_bytes(&tx, &["signature", "txId", "publicKey"])?;
        tx["signature"] = serde_json::json!(signer.sign(&signable)?);
        tx["publicKey"] = serde_json::json!(signer.public_key_hex());
        self.post("events", &tx).await
    }

    /// GET /api/streams/{subjectId} — stream metadata (head sequence, etc).
    pub async fn get_event_stream(&self, subject_id: &str, domain: &str) -> Result<Option<Value>> {
        let mut path = format!("streams/{}", urlencoding(subject_id));
        if !domain.is_empty() {
            path.push_str(&format!("?domain={}", urlencoding(domain)));
        }
        match self.get(&path).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/streams/{subjectId}/events — paginated event list.
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        domain: &str,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Vec<crate::types::Event>> {
        let mut path = format!("streams/{}/events", urlencoding(subject_id));
        let mut q = String::new();
        if !domain.is_empty() { q.push_str(&format!("&domain={}", urlencoding(domain))); }
        if let Some(l) = limit { q.push_str(&format!("&limit={}", l)); }
        if let Some(o) = offset { q.push_str(&format!("&offset={}", o)); }
        if !q.is_empty() { path.push('?'); path.push_str(&q[1..]); }

        #[derive(serde::Deserialize)]
        struct Wrap {
            #[serde(default)]
            data: Vec<crate::types::Event>,
            #[serde(default)]
            events: Vec<crate::types::Event>,
        }
        let w: Wrap = self.get_typed(&path).await?;
        Ok(if !w.data.is_empty() { w.data } else { w.events })
    }

    // --- IPFS ---------------------------------------------------------

    /// POST /api/ipfs/pin — pin raw bytes, returns the CID.
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
        let v = parse_envelope(resp).await?;
        v.get("cid")
            .and_then(|c| c.as_str())
            .map(String::from)
            .ok_or_else(|| Error::Node {
                status: 200,
                message: "ipfs pin response missing cid".into(),
            })
    }

    /// GET /api/ipfs/{cid} — fetch raw bytes.
    pub async fn ipfs_get(&self, cid: &str) -> Result<Vec<u8>> {
        let url = format!("{}/ipfs/{}", self.api_base, urlencoding(cid));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        if !resp.status().is_success() {
            let status = resp.status().as_u16();
            let body = resp.text().await.unwrap_or_default();
            return Err(Error::Node {
                status,
                message: format!("ipfs get: HTTP {} — {}", status, truncate(&body, 200)),
            });
        }
        Ok(resp.bytes().await?.to_vec())
    }

    // --- Registry queries ---------------------------------------------

    /// GET /api/registry/trust — paginated trust-edge listing.
    pub async fn query_trust_registry(
        &self,
        truster: Option<&str>,
        trustee: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let mut path = String::from("registry/trust");
        let mut q = String::new();
        if let Some(t) = truster { q.push_str(&format!("&truster={}", urlencoding(t))); }
        if let Some(t) = trustee { q.push_str(&format!("&trustee={}", urlencoding(t))); }
        if let Some(l) = limit { q.push_str(&format!("&limit={}", l)); }
        if let Some(o) = offset { q.push_str(&format!("&offset={}", o)); }
        if !q.is_empty() { path.push('?'); path.push_str(&q[1..]); }
        self.get(&path).await
    }

    /// GET /api/registry/identity — paginated identity listing.
    pub async fn query_identity_registry(
        &self,
        quid_id: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let mut path = String::from("registry/identity");
        let mut q = String::new();
        if let Some(q_) = quid_id { q.push_str(&format!("&quidId={}", urlencoding(q_))); }
        if let Some(l) = limit { q.push_str(&format!("&limit={}", l)); }
        if let Some(o) = offset { q.push_str(&format!("&offset={}", o)); }
        if !q.is_empty() { path.push('?'); path.push_str(&q[1..]); }
        self.get(&path).await
    }

    /// GET /api/registry/title — paginated title listing.
    pub async fn query_title_registry(
        &self,
        asset_id: Option<&str>,
        owner: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let mut path = String::from("registry/title");
        let mut q = String::new();
        if let Some(a) = asset_id { q.push_str(&format!("&assetId={}", urlencoding(a))); }
        if let Some(o) = owner { q.push_str(&format!("&owner={}", urlencoding(o))); }
        if let Some(l) = limit { q.push_str(&format!("&limit={}", l)); }
        if let Some(o) = offset { q.push_str(&format!("&offset={}", o)); }
        if !q.is_empty() { path.push('?'); path.push_str(&q[1..]); }
        self.get(&path).await
    }

    /// POST /api/trust/query — structured relational trust query.
    pub async fn query_relational_trust(
        &self,
        observer: &str,
        target: &str,
        domain: &str,
        max_depth: u32,
    ) -> Result<crate::types::TrustResult> {
        let body = serde_json::json!({
            "observer": observer,
            "target": target,
            "domain": domain,
            "maxDepth": max_depth,
        });
        let v = self.post("trust/query", &body).await?;
        serde_json::from_value(v).map_err(Error::from)
    }

    // --- Domain management --------------------------------------------

    /// GET /api/domains — list all registered trust domains on this node.
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    /// GET /api/domains/{name}/query — domain-specific query (trust/identity/title).
    pub async fn query_domain(
        &self,
        name: &str,
        query_type: Option<&str>,
        param: Option<&str>,
    ) -> Result<Value> {
        let mut path = format!("domains/{}/query", urlencoding(name));
        let mut q = String::new();
        if let Some(t) = query_type { q.push_str(&format!("&type={}", urlencoding(t))); }
        if let Some(p) = param { q.push_str(&format!("&param={}", urlencoding(p))); }
        if !q.is_empty() { path.push('?'); path.push_str(&q[1..]); }
        self.get(&path).await
    }

    /// GET /api/node/domains — domains this node is managing.
    pub async fn get_node_domains(&self) -> Result<Value> {
        self.get("node/domains").await
    }

    /// POST /api/node/domains — update the list of domains this node manages.
    pub async fn update_node_domains(&self, domains: &[&str]) -> Result<Value> {
        let body = serde_json::json!({ "managedDomains": domains });
        self.post("node/domains", &body).await
    }

    // --- Guardian sets + recovery (QDP-0002) --------------------------

    /// POST /api/guardian/set-update — install or rotate guardians.
    pub async fn submit_guardian_set_update(&self, update: &Value) -> Result<Value> {
        self.post("guardian/set-update", update).await
    }

    /// POST /api/guardian/recovery/init — start the M-of-N recovery delay.
    pub async fn submit_recovery_init(&self, init: &Value) -> Result<Value> {
        self.post("guardian/recovery/init", init).await
    }

    /// POST /api/guardian/recovery/veto — owner/guardian aborts a recovery.
    pub async fn submit_recovery_veto(&self, veto: &Value) -> Result<Value> {
        self.post("guardian/recovery/veto", veto).await
    }

    /// POST /api/guardian/recovery/commit — finalize the delayed recovery.
    pub async fn submit_recovery_commit(&self, commit: &Value) -> Result<Value> {
        self.post("guardian/recovery/commit", commit).await
    }

    /// POST /api/guardian/resign — guardian leaves the set.
    pub async fn submit_guardian_resignation(&self, resignation: &Value) -> Result<Value> {
        self.post("guardian/resign", resignation).await
    }

    /// GET /api/guardian/set/{quid} — current guardian set or None.
    pub async fn get_guardian_set(&self, quid: &str) -> Result<Option<crate::types::GuardianSet>> {
        match self.get_typed::<crate::types::GuardianSet>(
            &format!("guardian/set/{}", urlencoding(quid))).await {
            Ok(s) => Ok(Some(s)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/pending-recovery/{quid}.
    pub async fn get_pending_recovery(&self, quid: &str) -> Result<Option<Value>> {
        match self.get(&format!("guardian/pending-recovery/{}", urlencoding(quid))).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/resignations/{quid}.
    pub async fn get_guardian_resignations(&self, quid: &str) -> Result<Vec<Value>> {
        #[derive(serde::Deserialize)]
        struct Wrap {
            #[serde(default)]
            data: Vec<Value>,
            #[serde(default)]
            resignations: Vec<Value>,
        }
        match self.get_typed::<Wrap>(&format!("guardian/resignations/{}", urlencoding(quid))).await {
            Ok(w) => Ok(if !w.data.is_empty() { w.data } else { w.resignations }),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(vec![]),
            Err(e) => Err(e),
        }
    }

    // --- Cross-domain gossip + fingerprints (QDP-0003 / QDP-0005) ----

    /// POST /api/domain-fingerprints — publish a signed fingerprint.
    pub async fn submit_domain_fingerprint(&self, fp: &crate::types::DomainFingerprint) -> Result<Value> {
        let body = serde_json::to_value(fp)?;
        self.post("domain-fingerprints", &body).await
    }

    /// GET /api/domain-fingerprints/{domain}/latest.
    pub async fn get_latest_domain_fingerprint(
        &self, domain: &str,
    ) -> Result<Option<crate::types::DomainFingerprint>> {
        match self.get_typed::<crate::types::DomainFingerprint>(
            &format!("domain-fingerprints/{}/latest", urlencoding(domain))).await {
            Ok(f) => Ok(Some(f)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// POST /api/anchor-gossip — deliver a cross-domain anchor message.
    pub async fn submit_anchor_gossip(&self, message: &Value) -> Result<Value> {
        self.post("anchor-gossip", message).await
    }

    /// POST /api/gossip/push-anchor — push gossip variant (QDP-0005).
    pub async fn push_anchor(&self, message: &Value) -> Result<Value> {
        self.post("gossip/push-anchor", message).await
    }

    /// POST /api/gossip/push-fingerprint — push gossip variant (QDP-0005).
    pub async fn push_fingerprint(&self, fp: &crate::types::DomainFingerprint) -> Result<Value> {
        let body = serde_json::to_value(fp)?;
        self.post("gossip/push-fingerprint", &body).await
    }

    // --- Bootstrap + nonce snapshots (QDP-0008) -----------------------

    /// POST /api/nonce-snapshots — publish a K-of-K bootstrap snapshot.
    pub async fn submit_nonce_snapshot(&self, snapshot: &crate::types::NonceSnapshot) -> Result<Value> {
        let body = serde_json::to_value(snapshot)?;
        self.post("nonce-snapshots", &body).await
    }

    /// GET /api/nonce-snapshots/{domain}/latest.
    pub async fn get_latest_nonce_snapshot(
        &self, domain: &str,
    ) -> Result<Option<crate::types::NonceSnapshot>> {
        match self.get_typed::<crate::types::NonceSnapshot>(
            &format!("nonce-snapshots/{}/latest", urlencoding(domain))).await {
            Ok(s) => Ok(Some(s)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/bootstrap/status.
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get("bootstrap/status").await
    }

    // --- Fork-block (QDP-0009) ----------------------------------------

    /// POST /api/fork-block — submit a signed fork-activation block.
    pub async fn submit_fork_block(&self, fb: &crate::types::ForkBlock) -> Result<Value> {
        let body = serde_json::to_value(fb)?;
        self.post("fork-block", &body).await
    }

    /// GET /api/fork-block/status — activation status across features.
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
