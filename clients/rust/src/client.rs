//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{IdentityRecord, OwnershipStake, Title, TrustEdge, TrustResult};
use crate::wire::{IdentityTx, TitleTx, TrustTx};
use reqwest::{Client as HttpClient, StatusCode};
use serde::de::DeserializeOwned;
use serde_json::Value;
use std::collections::HashMap;
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

/// Parameters for [`Client::register_title`].
#[derive(Debug, Clone)]
pub struct TitleParams<'a> {
    /// Asset quid ID.
    pub asset_id: &'a str,
    /// Trust domain.
    pub domain: &'a str,
    /// Owners (percentages must sum to ~100).
    pub owners: Vec<OwnershipStake>,
    /// Optional title type discriminator.
    pub title_type: &'a str,
    /// Optional expiry (unix seconds; 0 = none).
    pub expiry_date: i64,
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

    /// Submit a signed TITLE transaction.
    ///
    /// v1.0 conformant: typed `TitleTx` wire struct + IEEE-1363
    /// signature + server-compatible ID derivation.
    pub async fn register_title<'a>(&self, signer: &Quid, p: TitleParams<'a>) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if p.owners.is_empty() {
            return Err(Error::validation("owners must be non-empty"));
        }
        let total: f64 = p.owners.iter().map(|o| o.percentage).sum();
        if (total - 100.0).abs() > 0.001 {
            return Err(Error::validation(format!(
                "ownership percentages must sum to 100 (got {})",
                total
            )));
        }
        let mut tx = TitleTx {
            id: String::new(),
            tx_type: "TITLE",
            trust_domain: p.domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            asset_id: p.asset_id,
            owners: p.owners,
            previous_owners: Vec::new(),
            signatures: HashMap::new(),
            expiry_date: p.expiry_date,
            title_type: p.title_type,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/title", &body).await
    }

    // ==================================================================
    // v1/v2 endpoints under /api/<path> (added in v0.3)
    // ==================================================================

    // --- Events -------------------------------------------------------

    /// Emit a signed event transaction (POST /api/events).
    pub async fn emit_event(&self, tx: &Value) -> Result<Value> {
        self.post("events", tx).await
    }

    /// Fetch the latest stream metadata for `subject_id`, or `None` on 404.
    pub async fn get_event_stream(&self, subject_id: &str) -> Result<Option<Value>> {
        self.get_or_null(&format!("streams/{}", urlencoding(subject_id)))
            .await
    }

    /// Page through stream events for `subject_id`.
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let qs = build_qs(&[
            ("limit", limit.map(|v| v.to_string())),
            ("offset", offset.map(|v| v.to_string())),
        ]);
        self.get(&format!("streams/{}/events{}", urlencoding(subject_id), qs))
            .await
    }

    // --- IPFS ---------------------------------------------------------

    /// Pin raw bytes to the node's IPFS gateway (POST /api/ipfs/pin).
    pub async fn ipfs_pin(&self, content: &[u8]) -> Result<Value> {
        let url = format!("{}/ipfs/pin", self.api_base);
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .body(content.to_vec())
            .send()
            .await?;
        parse_envelope(resp).await
    }

    /// Fetch raw bytes by CID (GET /api/ipfs/{cid}).
    pub async fn ipfs_get(&self, cid: &str) -> Result<Vec<u8>> {
        let url = format!("{}/ipfs/{}", self.api_base, urlencoding(cid));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        let bytes = resp.bytes().await?;
        if status.is_success() {
            return Ok(bytes.to_vec());
        }
        // Best-effort error envelope decoding.
        let env: Value = serde_json::from_slice(&bytes).unwrap_or(Value::Null);
        Err(envelope_to_error(status, &env))
    }

    // --- Registry queries --------------------------------------------

    /// Query the trust registry (GET /api/registry/trust).
    pub async fn query_trust_registry(&self, query: &[(&str, &str)]) -> Result<Value> {
        self.get(&format!("registry/trust{}", build_qs_pairs(query))).await
    }

    /// Query the identity registry (GET /api/registry/identity).
    pub async fn query_identity_registry(&self, query: &[(&str, &str)]) -> Result<Value> {
        self.get(&format!("registry/identity{}", build_qs_pairs(query)))
            .await
    }

    /// Query the title registry (GET /api/registry/title).
    pub async fn query_title_registry(&self, query: &[(&str, &str)]) -> Result<Value> {
        self.get(&format!("registry/title{}", build_qs_pairs(query))).await
    }

    // --- Blocks + pending -------------------------------------------

    /// List committed blocks (GET /api/blocks).
    pub async fn get_blocks(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let qs = build_qs(&[
            ("limit", limit.map(|v| v.to_string())),
            ("offset", offset.map(|v| v.to_string())),
        ]);
        self.get(&format!("blocks{}", qs)).await
    }

    /// Tentative (pre-commit) blocks for a domain.
    pub async fn get_tentative_blocks(&self, domain: &str) -> Result<Value> {
        self.get(&format!("blocks/tentative/{}", urlencoding(domain)))
            .await
    }

    /// Pending transaction pool snapshot.
    pub async fn get_pending_transactions(
        &self,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let qs = build_qs(&[
            ("limit", limit.map(|v| v.to_string())),
            ("offset", offset.map(|v| v.to_string())),
        ]);
        self.get(&format!("transactions{}", qs)).await
    }

    // --- Domains / nodes ---------------------------------------------

    /// List known trust domains (GET /api/domains).
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    /// List known nodes (GET /api/nodes).
    pub async fn nodes(&self) -> Result<Value> {
        self.get("nodes").await
    }

    /// Domains this node serves (GET /api/node/domains).
    pub async fn get_node_domains(&self) -> Result<Value> {
        self.get("node/domains").await
    }

    /// Update the domains this node serves (POST /api/node/domains).
    pub async fn update_node_domains(&self, domains: &[String]) -> Result<Value> {
        let body = serde_json::json!({ "domains": domains });
        self.post("node/domains", &body).await
    }

    // ==================================================================
    // v3 surface — peers, advertisements, moderation, audit, privacy
    // (all under /api/<path>)
    // ==================================================================

    // --- Peers (QDP-0011) --------------------------------------------

    /// Peer scoreboard (GET /api/peers).
    pub async fn get_peers(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let qs = build_qs(&[
            ("limit", limit.map(|v| v.to_string())),
            ("offset", offset.map(|v| v.to_string())),
        ]);
        self.get(&format!("peers{}", qs)).await
    }

    /// Per-peer breakdown, or `None` on 404.
    pub async fn get_peer(&self, node_quid: &str) -> Result<Option<Value>> {
        self.get_or_null(&format!("peers/{}", urlencoding(node_quid)))
            .await
    }

    // --- Node advertisements -----------------------------------------

    /// Submit a node advertisement (POST /api/node-advertisements).
    pub async fn submit_node_advertisement(&self, ad: &Value) -> Result<Value> {
        self.post("node-advertisements", ad).await
    }

    // --- Domain extras ------------------------------------------------

    /// Top domains by activity (GET /api/domains/top).
    pub async fn get_top_domains(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let qs = build_qs(&[
            ("limit", limit.map(|v| v.to_string())),
            ("offset", offset.map(|v| v.to_string())),
        ]);
        self.get(&format!("domains/top{}", qs)).await
    }

    /// Submit a domain gossip message (POST /api/gossip/domains).
    pub async fn submit_domain_gossip(&self, msg: &Value) -> Result<Value> {
        self.post("gossip/domains", msg).await
    }

    // --- Moderation (QDP-0015) ---------------------------------------

    /// Submit a moderation action (POST /api/moderation/actions).
    pub async fn submit_moderation_action(&self, action: &Value) -> Result<Value> {
        self.post("moderation/actions", action).await
    }

    /// Get moderation actions for a target.
    pub async fn get_moderation_actions(
        &self,
        target_type: &str,
        target_id: &str,
    ) -> Result<Value> {
        self.get(&format!(
            "moderation/actions/{}/{}",
            urlencoding(target_type),
            urlencoding(target_id)
        ))
        .await
    }

    // --- Audit (QDP-0018) --------------------------------------------

    /// Operator audit log head (GET /api/audit/head).
    pub async fn get_audit_head(&self) -> Result<Value> {
        self.get("audit/head").await
    }

    /// Operator audit entries (GET /api/audit/entries).
    pub async fn get_audit_entries(
        &self,
        since: Option<i64>,
        limit: Option<u32>,
    ) -> Result<Value> {
        let qs = build_qs(&[
            ("since", since.map(|v| v.to_string())),
            ("limit", limit.map(|v| v.to_string())),
        ]);
        self.get(&format!("audit/entries{}", qs)).await
    }

    /// Single audit entry by sequence, or `None` on 404.
    pub async fn get_audit_entry(&self, sequence: u64) -> Result<Option<Value>> {
        self.get_or_null(&format!("audit/entry/{}", sequence)).await
    }

    // --- Privacy / DSR (QDP-0017) ------------------------------------

    /// Submit a DSR request (POST /api/privacy/dsr).
    pub async fn submit_dsr(&self, request: &Value) -> Result<Value> {
        self.post("privacy/dsr", request).await
    }

    /// Get DSR status, or `None` on 404.
    pub async fn get_dsr_status(&self, request_tx_id: &str) -> Result<Option<Value>> {
        self.get_or_null(&format!("privacy/dsr/{}", urlencoding(request_tx_id)))
            .await
    }

    /// Record a consent grant (POST /api/privacy/consent/grants).
    pub async fn grant_consent(&self, grant: &Value) -> Result<Value> {
        self.post("privacy/consent/grants", grant).await
    }

    /// Withdraw consent (POST /api/privacy/consent/withdraws).
    pub async fn withdraw_consent(&self, withdraw: &Value) -> Result<Value> {
        self.post("privacy/consent/withdraws", withdraw).await
    }

    /// Consent history (GET /api/privacy/consent/history).
    pub async fn get_consent_history(
        &self,
        limit: Option<u32>,
        offset: Option<u32>,
        subject_quid: Option<&str>,
    ) -> Result<Value> {
        let qs = build_qs(&[
            ("limit", limit.map(|v| v.to_string())),
            ("offset", offset.map(|v| v.to_string())),
            ("subjectQuid", subject_quid.map(|s| s.to_string())),
        ]);
        self.get(&format!("privacy/consent/history{}", qs)).await
    }

    /// Create a processing restriction (POST /api/privacy/restrictions).
    pub async fn create_processing_restriction(&self, restriction: &Value) -> Result<Value> {
        self.post("privacy/restrictions", restriction).await
    }

    /// List processing restrictions for a subject.
    pub async fn get_processing_restrictions(&self, subject_quid: &str) -> Result<Value> {
        self.get(&format!(
            "privacy/restrictions/{}",
            urlencoding(subject_quid)
        ))
        .await
    }

    /// Submit a DSR compliance attestation (POST /api/privacy/compliance).
    pub async fn submit_dsr_compliance(&self, compliance: &Value) -> Result<Value> {
        self.post("privacy/compliance", compliance).await
    }

    // ==================================================================
    // v2 surface — guardian, gossip, bootstrap, fork-block, discovery, DNS
    // (all under /api/v2/<path>)
    // ==================================================================

    // --- Guardian (QDP-0002) -----------------------------------------

    /// Install or rotate a guardian set (POST /api/v2/guardian/set-update).
    pub async fn submit_guardian_set_update(&self, update: &Value) -> Result<Value> {
        self.post_v2("guardian/set-update", update).await
    }

    /// Initiate guardian recovery (POST /api/v2/guardian/recovery/init).
    pub async fn submit_recovery_init(&self, init: &Value) -> Result<Value> {
        self.post_v2("guardian/recovery/init", init).await
    }

    /// Veto a pending recovery (POST /api/v2/guardian/recovery/veto).
    pub async fn submit_recovery_veto(&self, veto: &Value) -> Result<Value> {
        self.post_v2("guardian/recovery/veto", veto).await
    }

    /// Commit a recovery after the delay (POST /api/v2/guardian/recovery/commit).
    pub async fn submit_recovery_commit(&self, commit: &Value) -> Result<Value> {
        self.post_v2("guardian/recovery/commit", commit).await
    }

    /// Submit a guardian resignation (POST /api/v2/guardian/resign).
    pub async fn submit_guardian_resignation(&self, resignation: &Value) -> Result<Value> {
        self.post_v2("guardian/resign", resignation).await
    }

    /// Fetch the active guardian set for a quid, or `None` on 404.
    pub async fn get_guardian_set(&self, quid: &str) -> Result<Option<Value>> {
        self.get_v2_or_null(&format!("guardian/set/{}", urlencoding(quid)))
            .await
    }

    /// Fetch any pending recovery for a quid, or `None` on 404.
    pub async fn get_pending_recovery(&self, quid: &str) -> Result<Option<Value>> {
        self.get_v2_or_null(&format!("guardian/pending-recovery/{}", urlencoding(quid)))
            .await
    }

    /// Fetch guardian resignations for a quid.
    pub async fn get_guardian_resignations(&self, quid: &str) -> Result<Value> {
        self.get_v2(&format!("guardian/resignations/{}", urlencoding(quid)))
            .await
    }

    // --- Cross-domain gossip (QDP-0003 / QDP-0005) -------------------

    /// Submit a domain fingerprint (POST /api/v2/domain-fingerprints).
    pub async fn submit_domain_fingerprint(&self, fp: &Value) -> Result<Value> {
        self.post_v2("domain-fingerprints", fp).await
    }

    /// Latest fingerprint for a domain, or `None` on 404.
    pub async fn get_latest_domain_fingerprint(&self, domain: &str) -> Result<Option<Value>> {
        self.get_v2_or_null(&format!(
            "domain-fingerprints/{}/latest",
            urlencoding(domain)
        ))
        .await
    }

    /// Submit an anchor gossip message (POST /api/v2/anchor-gossip).
    pub async fn submit_anchor_gossip(&self, msg: &Value) -> Result<Value> {
        self.post_v2("anchor-gossip", msg).await
    }

    /// Push an anchor to peers (POST /api/v2/gossip/push-anchor).
    pub async fn push_anchor(&self, msg: &Value) -> Result<Value> {
        self.post_v2("gossip/push-anchor", msg).await
    }

    /// Push a fingerprint to peers (POST /api/v2/gossip/push-fingerprint).
    pub async fn push_fingerprint(&self, fp: &Value) -> Result<Value> {
        self.post_v2("gossip/push-fingerprint", fp).await
    }

    // --- Bootstrap (QDP-0008) ----------------------------------------

    /// Submit a nonce snapshot (POST /api/v2/nonce-snapshots).
    pub async fn submit_nonce_snapshot(&self, snapshot: &Value) -> Result<Value> {
        self.post_v2("nonce-snapshots", snapshot).await
    }

    /// Latest nonce snapshot for a domain, or `None` on 404.
    pub async fn get_latest_nonce_snapshot(&self, domain: &str) -> Result<Option<Value>> {
        self.get_v2_or_null(&format!("nonce-snapshots/{}/latest", urlencoding(domain)))
            .await
    }

    /// Bootstrap status (GET /api/v2/bootstrap/status).
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get_v2("bootstrap/status").await
    }

    // --- Fork-block (QDP-0009) ---------------------------------------

    /// Submit a fork-activation block (POST /api/v2/fork-block).
    pub async fn submit_fork_block(&self, fb: &Value) -> Result<Value> {
        self.post_v2("fork-block", fb).await
    }

    /// Fork-block status (GET /api/v2/fork-block/status).
    pub async fn fork_block_status(&self) -> Result<Value> {
        self.get_v2("fork-block/status").await
    }

    // --- Discovery (QDP-0014) ----------------------------------------

    /// Lookup a discovery domain record, or `None` on 404.
    pub async fn get_discovery_domain(&self, name: &str) -> Result<Option<Value>> {
        self.get_v2_or_null(&format!("discovery/domain/{}", urlencoding(name)))
            .await
    }

    /// Lookup a discovery node record, or `None` on 404.
    pub async fn get_discovery_node(&self, quid: &str) -> Result<Option<Value>> {
        self.get_v2_or_null(&format!("discovery/node/{}", urlencoding(quid)))
            .await
    }

    /// Lookup a discovery operator record, or `None` on 404.
    pub async fn get_discovery_operator(&self, quid: &str) -> Result<Option<Value>> {
        self.get_v2_or_null(&format!("discovery/operator/{}", urlencoding(quid)))
            .await
    }

    /// Search discovery quids.
    pub async fn discovery_quids(&self, query: &[(&str, &str)]) -> Result<Value> {
        self.get_v2(&format!("discovery/quids{}", build_qs_pairs(query)))
            .await
    }

    /// Search trust-anchored discovery quids.
    pub async fn discovery_trusted_quids(&self, query: &[(&str, &str)]) -> Result<Value> {
        self.get_v2(&format!(
            "discovery/trusted-quids{}",
            build_qs_pairs(query)
        ))
        .await
    }

    // --- DNS attestation (QDP-0023) ---------------------------------

    /// Submit a DNS claim (POST /api/v2/dns/claim).
    pub async fn submit_dns_claim(&self, claim: &Value) -> Result<Value> {
        self.post_v2("dns/claim", claim).await
    }

    /// Submit a DNS challenge (POST /api/v2/dns/challenge).
    pub async fn submit_dns_challenge(&self, challenge: &Value) -> Result<Value> {
        self.post_v2("dns/challenge", challenge).await
    }

    /// Submit a DNS attestation (POST /api/v2/dns/attestation).
    pub async fn submit_dns_attestation(&self, attestation: &Value) -> Result<Value> {
        self.post_v2("dns/attestation", attestation).await
    }

    /// Submit a DNS attestation renewal (POST /api/v2/dns/renewal).
    pub async fn submit_dns_renewal(&self, renewal: &Value) -> Result<Value> {
        self.post_v2("dns/renewal", renewal).await
    }

    /// Submit a DNS attestation revocation (POST /api/v2/dns/revocation).
    pub async fn submit_dns_revocation(&self, revocation: &Value) -> Result<Value> {
        self.post_v2("dns/revocation", revocation).await
    }

    /// Submit a DNS delegate (POST /api/v2/dns/delegate).
    pub async fn submit_dns_delegate(&self, delegate: &Value) -> Result<Value> {
        self.post_v2("dns/delegate", delegate).await
    }

    /// Revoke a DNS delegate (POST /api/v2/dns/delegate-revocation).
    pub async fn submit_dns_delegate_revocation(&self, revocation: &Value) -> Result<Value> {
        self.post_v2("dns/delegate-revocation", revocation).await
    }

    /// List DNS attestations for a domain.
    pub async fn get_dns_attestations(&self, domain: &str) -> Result<Value> {
        self.get_v2(&format!("dns/attestations/{}", urlencoding(domain)))
            .await
    }

    /// List weighted DNS attestations for a domain.
    pub async fn get_dns_attestations_weighted(&self, domain: &str) -> Result<Value> {
        self.get_v2(&format!(
            "dns/attestations/{}/weighted",
            urlencoding(domain)
        ))
        .await
    }

    /// Resolve a DNS record via the protocol-layer attestation set.
    pub async fn resolve_dns(&self, domain: &str, record_type: &str) -> Result<Value> {
        self.get_v2(&format!(
            "dns/resolve/{}/{}",
            urlencoding(domain),
            urlencoding(record_type)
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

    async fn get_v2(&self, path: &str) -> Result<Value> {
        let url = format!("{}/v2/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        parse_envelope(resp).await
    }

    async fn post_v2(&self, path: &str, body: &Value) -> Result<Value> {
        let url = format!("{}/v2/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .json(body)
            .send()
            .await?;
        parse_envelope(resp).await
    }

    /// Shared `Result<Option<Value>>` adapter: maps NOT_FOUND to `None`,
    /// propagates everything else.
    async fn get_or_null(&self, path: &str) -> Result<Option<Value>> {
        match self.get(path).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    async fn get_v2_or_null(&self, path: &str) -> Result<Option<Value>> {
        match self.get_v2(path).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
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

/// Build a `?k=v&k=v` query string from an iterable of optional values.
/// Skips `None` entries. Returns the empty string when nothing's present.
fn build_qs(params: &[(&str, Option<String>)]) -> String {
    let mut parts: Vec<String> = Vec::with_capacity(params.len());
    for (k, v) in params {
        if let Some(val) = v {
            parts.push(format!("{}={}", urlencoding(k), urlencoding(val)));
        }
    }
    if parts.is_empty() {
        String::new()
    } else {
        format!("?{}", parts.join("&"))
    }
}

/// Build a `?k=v&k=v` query string from already-present pairs.
fn build_qs_pairs(params: &[(&str, &str)]) -> String {
    if params.is_empty() {
        return String::new();
    }
    let parts: Vec<String> = params
        .iter()
        .map(|(k, v)| format!("{}={}", urlencoding(k), urlencoding(v)))
        .collect();
    format!("?{}", parts.join("&"))
}
