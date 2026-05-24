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

    // ------------------------------------------------------------------
    // Health / info / peers / nodes / quids
    // ------------------------------------------------------------------

    /// GET /api/nodes — list known peer nodes.
    pub async fn nodes(&self) -> Result<Value> {
        self.get("nodes").await
    }

    /// GET /api/peers — peer scoreboard snapshot.
    pub async fn peers(&self) -> Result<Value> {
        self.get("peers").await
    }

    /// GET /api/peers/{nodeQuid} — single peer's record, or `None` if absent.
    pub async fn get_peer(&self, node_quid: &str) -> Result<Option<Value>> {
        if node_quid.is_empty() {
            return Err(Error::validation("node_quid is required"));
        }
        match self.get(&format!("peers/{}", urlencoding(node_quid))).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") || m.contains("PEER_NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" || code == "PEER_NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// POST /api/quids — server-side keygen (trusted mode only).
    pub async fn generate_quid(&self, metadata: Option<Value>) -> Result<Value> {
        let body = serde_json::json!({ "metadata": metadata.unwrap_or_else(|| serde_json::json!({})) });
        self.post("quids", &body).await
    }

    // ------------------------------------------------------------------
    // Blocks + transactions
    // ------------------------------------------------------------------

    /// GET /api/blocks — paginated block list.
    pub async fn get_blocks(&self) -> Result<Value> {
        self.get("blocks").await
    }

    /// GET /api/blocks/tentative/{domain}.
    pub async fn get_tentative_blocks(&self, domain: &str) -> Result<Value> {
        if domain.is_empty() {
            return Err(Error::validation("domain is required"));
        }
        self.get(&format!("blocks/tentative/{}", urlencoding(domain))).await
    }

    /// GET /api/transactions — pending mempool transactions.
    pub async fn get_pending_transactions(&self) -> Result<Value> {
        self.get("transactions").await
    }

    // ------------------------------------------------------------------
    // Title (POST + queries)
    // ------------------------------------------------------------------

    /// POST /api/transactions/title — register a title.
    pub async fn register_title(&self, body: &Value) -> Result<Value> {
        self.post("transactions/title", body).await
    }

    // ------------------------------------------------------------------
    // Events + streams
    // ------------------------------------------------------------------

    /// POST /api/events — submit a signed EVENT transaction.
    pub async fn emit_event(&self, event: &Value) -> Result<Value> {
        self.post("events", event).await
    }

    /// GET /api/streams/{subjectId} — stream metadata, or `None` if absent.
    pub async fn get_event_stream(&self, subject_id: &str) -> Result<Option<Value>> {
        if subject_id.is_empty() {
            return Err(Error::validation("subject_id is required"));
        }
        match self.get(&format!("streams/{}", urlencoding(subject_id))).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/streams/{subjectId}/events — events with optional pagination.
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        domain: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        if subject_id.is_empty() {
            return Err(Error::validation("subject_id is required"));
        }
        let mut path = format!("streams/{}/events", urlencoding(subject_id));
        let mut qs: Vec<String> = Vec::new();
        if let Some(d) = domain { qs.push(format!("domain={}", urlencoding(d))); }
        if let Some(l) = limit { qs.push(format!("limit={l}")); }
        if let Some(o) = offset { qs.push(format!("offset={o}")); }
        if !qs.is_empty() {
            path.push('?');
            path.push_str(&qs.join("&"));
        }
        self.get(&path).await
    }

    // ------------------------------------------------------------------
    // Domains
    // ------------------------------------------------------------------

    /// GET /api/domains — every domain this node knows about.
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    /// GET /api/domains/top — top-N most-active domains.
    pub async fn top_domains(&self) -> Result<Value> {
        self.get("domains/top").await
    }

    /// GET /api/domains/{name}/query — query a domain registry directly.
    ///
    /// `query_type` is one of "identity", "trust", "title"; `param`
    /// is the lookup key (for trust queries, "observer:target").
    pub async fn query_domain(&self, domain: &str, query_type: &str, param: &str) -> Result<Value> {
        if domain.is_empty() {
            return Err(Error::validation("domain is required"));
        }
        if !matches!(query_type, "identity" | "trust" | "title") {
            return Err(Error::validation(
                "query_type must be 'identity', 'trust', or 'title'",
            ));
        }
        self.get(&format!(
            "domains/{}/query?type={}&param={}",
            urlencoding(domain),
            urlencoding(query_type),
            urlencoding(param),
        ))
        .await
    }

    /// GET /api/node/domains.
    pub async fn get_node_domains(&self) -> Result<Value> {
        self.get("node/domains").await
    }

    /// POST /api/node/domains — replace the node's managed-domains list.
    pub async fn update_node_domains(&self, domains: &[&str]) -> Result<Value> {
        let body = serde_json::json!({ "managedDomains": domains });
        self.post("node/domains", &body).await
    }

    /// POST /api/gossip/domains — push a domain-gossip message.
    pub async fn send_domain_gossip(&self, gossip: &Value) -> Result<Value> {
        self.post("gossip/domains", gossip).await
    }

    // ------------------------------------------------------------------
    // Registry queries
    // ------------------------------------------------------------------

    /// GET /api/registry/trust — paginated/filterable trust registry.
    pub async fn query_trust_registry(&self) -> Result<Value> {
        self.get("registry/trust").await
    }

    /// GET /api/registry/identity — paginated/filterable identity registry.
    pub async fn query_identity_registry(&self) -> Result<Value> {
        self.get("registry/identity").await
    }

    /// GET /api/registry/title — paginated/filterable title registry.
    pub async fn query_title_registry(&self) -> Result<Value> {
        self.get("registry/title").await
    }

    /// POST /api/trust/query — multi-quid relational trust query.
    pub async fn query_relational_trust(&self, query: &Value) -> Result<Value> {
        self.post("trust/query", query).await
    }

    // ------------------------------------------------------------------
    // IPFS
    // ------------------------------------------------------------------

    /// POST /api/ipfs/pin — pin raw content; returns the CID.
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
        let env = parse_envelope(resp).await?;
        env.get("cid")
            .and_then(|v| v.as_str())
            .or_else(|| env.get("value").and_then(|v| v.as_str()))
            .map(|s| s.to_string())
            .ok_or_else(|| Error::Node {
                status: 0,
                message: "ipfs pin response missing cid".into(),
            })
    }

    /// GET /api/ipfs/{cid} — fetch the pinned bytes.
    pub async fn ipfs_get(&self, cid: &str) -> Result<Vec<u8>> {
        if cid.is_empty() {
            return Err(Error::validation("cid is required"));
        }
        let url = format!("{}/ipfs/{}", self.api_base, urlencoding(cid));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        if status.is_client_error() || status.is_server_error() {
            return Err(Error::Node {
                status: status.as_u16(),
                message: format!("ipfs get failed (HTTP {})", status.as_u16()),
            });
        }
        Ok(resp.bytes().await?.to_vec())
    }

    // ------------------------------------------------------------------
    // Node advertisements (QDP-0014)
    // ------------------------------------------------------------------

    /// POST /api/node-advertisements — publish a signed node advertisement.
    pub async fn create_node_advertisement(&self, advertisement: &Value) -> Result<Value> {
        self.post("node-advertisements", advertisement).await
    }

    // ------------------------------------------------------------------
    // Guardian sets + recovery (QDP-0002 / QDP-0006)
    // ------------------------------------------------------------------

    /// POST /api/guardian/set-update — install or rotate guardians.
    pub async fn submit_guardian_set_update(&self, update: &Value) -> Result<Value> {
        self.post("guardian/set-update", update).await
    }

    /// POST /api/guardian/recovery/init — start the M-of-N recovery delay.
    pub async fn submit_recovery_init(&self, init: &Value) -> Result<Value> {
        self.post("guardian/recovery/init", init).await
    }

    /// POST /api/guardian/recovery/veto — owner/guardian aborts recovery.
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

    /// GET /api/guardian/set/{quid} — current guardian set, or `None`.
    pub async fn get_guardian_set(&self, quid_id: &str) -> Result<Option<Value>> {
        match self.get(&format!("guardian/set/{}", urlencoding(quid_id))).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/pending-recovery/{quid}.
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

    /// GET /api/guardian/resignations/{quid}.
    pub async fn get_guardian_resignations(&self, quid_id: &str) -> Result<Value> {
        self.get(&format!("guardian/resignations/{}", urlencoding(quid_id))).await
    }

    // ------------------------------------------------------------------
    // Cross-domain gossip + fingerprints (QDP-0003 / QDP-0005)
    // ------------------------------------------------------------------

    /// POST /api/domain-fingerprints — publish a signed fingerprint.
    pub async fn submit_domain_fingerprint(&self, fp: &Value) -> Result<Value> {
        self.post("domain-fingerprints", fp).await
    }

    /// GET /api/domain-fingerprints/{domain}/latest.
    pub async fn get_latest_domain_fingerprint(&self, domain: &str) -> Result<Option<Value>> {
        match self
            .get(&format!("domain-fingerprints/{}/latest", urlencoding(domain)))
            .await
        {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// POST /api/anchor-gossip — deliver a cross-domain anchor message.
    pub async fn submit_anchor_gossip(&self, message: &Value) -> Result<Value> {
        self.post("anchor-gossip", message).await
    }

    /// POST /api/gossip/push-anchor — push-gossip variant (QDP-0005).
    pub async fn push_anchor(&self, message: &Value) -> Result<Value> {
        self.post("gossip/push-anchor", message).await
    }

    /// POST /api/gossip/push-fingerprint — push-gossip variant (QDP-0005).
    pub async fn push_fingerprint(&self, fp: &Value) -> Result<Value> {
        self.post("gossip/push-fingerprint", fp).await
    }

    // ------------------------------------------------------------------
    // Bootstrap + nonce snapshots (QDP-0008)
    // ------------------------------------------------------------------

    /// POST /api/nonce-snapshots — publish a K-of-K bootstrap snapshot.
    pub async fn submit_nonce_snapshot(&self, snapshot: &Value) -> Result<Value> {
        self.post("nonce-snapshots", snapshot).await
    }

    /// GET /api/nonce-snapshots/{domain}/latest.
    pub async fn get_latest_nonce_snapshot(&self, domain: &str) -> Result<Option<Value>> {
        match self
            .get(&format!("nonce-snapshots/{}/latest", urlencoding(domain)))
            .await
        {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/bootstrap/status.
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get("bootstrap/status").await
    }

    // ------------------------------------------------------------------
    // Fork-block (QDP-0009)
    // ------------------------------------------------------------------

    /// POST /api/fork-block — submit a signed fork-activation block.
    pub async fn submit_fork_block(&self, fb: &Value) -> Result<Value> {
        self.post("fork-block", fb).await
    }

    /// GET /api/fork-block/status — activation status across features.
    pub async fn fork_block_status(&self) -> Result<Value> {
        self.get("fork-block/status").await
    }

    // ------------------------------------------------------------------
    // Moderation (QDP-0015)
    // ------------------------------------------------------------------

    /// POST /api/moderation/actions — submit a signed moderation action.
    pub async fn create_moderation_action(&self, action: &Value) -> Result<Value> {
        self.post("moderation/actions", action).await
    }

    /// GET /api/moderation/actions/{targetType}/{targetId}.
    pub async fn get_moderation_actions(
        &self,
        target_type: &str,
        target_id: &str,
    ) -> Result<Value> {
        if target_type.is_empty() || target_id.is_empty() {
            return Err(Error::validation("target_type and target_id are required"));
        }
        self.get(&format!(
            "moderation/actions/{}/{}",
            urlencoding(target_type),
            urlencoding(target_id),
        ))
        .await
    }

    // ------------------------------------------------------------------
    // Audit log (QDP-0018)
    // ------------------------------------------------------------------

    /// GET /api/audit/head — operator's current audit head.
    pub async fn audit_head(&self) -> Result<Value> {
        self.get("audit/head").await
    }

    /// GET /api/audit/entries — entries after a cursor.
    pub async fn audit_entries(&self, since: Option<i64>, limit: Option<u32>) -> Result<Value> {
        let mut path = String::from("audit/entries");
        let mut qs: Vec<String> = Vec::new();
        if let Some(s) = since { qs.push(format!("since={s}")); }
        if let Some(l) = limit { qs.push(format!("limit={l}")); }
        if !qs.is_empty() {
            path.push('?');
            path.push_str(&qs.join("&"));
        }
        self.get(&path).await
    }

    /// GET /api/audit/entry/{sequence} — one entry by sequence, or `None` on 404.
    pub async fn audit_entry(&self, sequence: i64) -> Result<Option<Value>> {
        if sequence < 0 {
            return Err(Error::validation("sequence must be non-negative"));
        }
        match self.get(&format!("audit/entry/{sequence}")).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    // ------------------------------------------------------------------
    // Privacy (QDP-0017)
    // ------------------------------------------------------------------

    /// POST /api/privacy/dsr — submit a Data Subject Request.
    pub async fn create_dsr(&self, request: &Value) -> Result<Value> {
        self.post("privacy/dsr", request).await
    }

    /// GET /api/privacy/dsr/{requestTxId} — status of a DSR or `None` on 404.
    pub async fn get_dsr_status(&self, request_tx_id: &str) -> Result<Option<Value>> {
        if request_tx_id.is_empty() {
            return Err(Error::validation("request_tx_id is required"));
        }
        match self.get(&format!("privacy/dsr/{}", urlencoding(request_tx_id))).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// POST /api/privacy/consent/grants — record an opt-in.
    pub async fn create_consent_grant(&self, grant: &Value) -> Result<Value> {
        self.post("privacy/consent/grants", grant).await
    }

    /// POST /api/privacy/consent/withdraws — revoke a prior grant.
    pub async fn create_consent_withdraw(&self, withdraw: &Value) -> Result<Value> {
        self.post("privacy/consent/withdraws", withdraw).await
    }

    /// GET /api/privacy/consent/history?subject={quid}.
    pub async fn get_consent_history(&self, subject_quid: &str) -> Result<Value> {
        if subject_quid.is_empty() {
            return Err(Error::validation("subject_quid is required"));
        }
        self.get(&format!(
            "privacy/consent/history?subject={}",
            urlencoding(subject_quid),
        ))
        .await
    }

    /// POST /api/privacy/restrictions — narrow allowed processing.
    pub async fn create_processing_restriction(&self, restriction: &Value) -> Result<Value> {
        self.post("privacy/restrictions", restriction).await
    }

    /// GET /api/privacy/restrictions/{subjectQuid}.
    pub async fn get_restrictions_for_subject(&self, subject_quid: &str) -> Result<Value> {
        if subject_quid.is_empty() {
            return Err(Error::validation("subject_quid is required"));
        }
        self.get(&format!("privacy/restrictions/{}", urlencoding(subject_quid))).await
    }

    /// POST /api/privacy/compliance — operator's compliance attestation.
    pub async fn create_dsr_compliance(&self, compliance: &Value) -> Result<Value> {
        self.post("privacy/compliance", compliance).await
    }

    // ------------------------------------------------------------------
    // Discovery (QDP-0014)
    // ------------------------------------------------------------------

    /// GET /api/v2/discovery/domain/{name}.
    pub async fn discover_domain(&self, name: &str) -> Result<Value> {
        if name.is_empty() {
            return Err(Error::validation("name is required"));
        }
        self.get(&format!("v2/discovery/domain/{}", urlencoding(name))).await
    }

    /// GET /api/v2/discovery/node/{quid}.
    pub async fn discover_node(&self, quid: &str) -> Result<Value> {
        if quid.is_empty() {
            return Err(Error::validation("quid is required"));
        }
        self.get(&format!("v2/discovery/node/{}", urlencoding(quid))).await
    }

    /// GET /api/v2/discovery/operator/{quid}.
    pub async fn discover_operator(&self, quid: &str) -> Result<Value> {
        if quid.is_empty() {
            return Err(Error::validation("quid is required"));
        }
        self.get(&format!("v2/discovery/operator/{}", urlencoding(quid))).await
    }

    /// GET /api/v2/discovery/quids.
    pub async fn discover_quids(&self) -> Result<Value> {
        self.get("v2/discovery/quids").await
    }

    /// GET /api/v2/discovery/trusted-quids.
    pub async fn discover_trusted_quids(&self) -> Result<Value> {
        self.get("v2/discovery/trusted-quids").await
    }

    // ------------------------------------------------------------------
    // DNS attestation (QDP-0023)
    // ------------------------------------------------------------------

    /// POST /api/v2/dns/claim — declare intent to attest a DNS domain.
    pub async fn submit_dns_claim(&self, claim: &Value) -> Result<Value> {
        self.post("v2/dns/claim", claim).await
    }

    /// POST /api/v2/dns/challenge — root's challenge back to the claimant.
    pub async fn submit_dns_challenge(&self, challenge: &Value) -> Result<Value> {
        self.post("v2/dns/challenge", challenge).await
    }

    /// POST /api/v2/dns/attestation — root signs the verified claim.
    pub async fn submit_dns_attestation(&self, attestation: &Value) -> Result<Value> {
        self.post("v2/dns/attestation", attestation).await
    }

    /// POST /api/v2/dns/renewal — extend an attestation's validity.
    pub async fn submit_dns_renewal(&self, renewal: &Value) -> Result<Value> {
        self.post("v2/dns/renewal", renewal).await
    }

    /// POST /api/v2/dns/revocation — revoke a DNS attestation.
    pub async fn submit_dns_revocation(&self, revocation: &Value) -> Result<Value> {
        self.post("v2/dns/revocation", revocation).await
    }

    /// POST /api/v2/dns/delegate — delegate DNS authority to another quid.
    pub async fn submit_authority_delegate(&self, delegate: &Value) -> Result<Value> {
        self.post("v2/dns/delegate", delegate).await
    }

    /// POST /api/v2/dns/delegate-revocation — revoke a delegation.
    pub async fn submit_authority_delegate_revocation(
        &self,
        revocation: &Value,
    ) -> Result<Value> {
        self.post("v2/dns/delegate-revocation", revocation).await
    }

    /// GET /api/v2/dns/attestations/{domain} — all attestations for a domain.
    pub async fn get_dns_attestations(&self, domain: &str) -> Result<Value> {
        if domain.is_empty() {
            return Err(Error::validation("domain is required"));
        }
        self.get(&format!("v2/dns/attestations/{}", urlencoding(domain))).await
    }

    /// GET /api/v2/dns/attestations/{domain}/weighted — trust-weighted view.
    pub async fn get_dns_attestations_weighted(&self, domain: &str) -> Result<Value> {
        if domain.is_empty() {
            return Err(Error::validation("domain is required"));
        }
        self.get(&format!(
            "v2/dns/attestations/{}/weighted",
            urlencoding(domain),
        ))
        .await
    }

    /// GET /api/v2/dns/resolve/{domain}/{recordType} — resolve a record.
    pub async fn resolve_dns_record(&self, domain: &str, record_type: &str) -> Result<Value> {
        if domain.is_empty() || record_type.is_empty() {
            return Err(Error::validation("domain and record_type are required"));
        }
        self.get(&format!(
            "v2/dns/resolve/{}/{}",
            urlencoding(domain),
            urlencoding(record_type),
        ))
        .await
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
            if code == "NOT_FOUND" || code == "PEER_NOT_FOUND" {
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
