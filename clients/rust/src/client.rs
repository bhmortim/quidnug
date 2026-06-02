//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{
    AnchorGossipMessage, DiscoverQuidsParams, DomainFingerprint, Event, EventParams, ForkBlock,
    GuardianRecoveryCommit, GuardianRecoveryInit, GuardianRecoveryVeto, GuardianResignation,
    GuardianSet, GuardianSetUpdate, IdentityRecord, NodeAdvertisementParams, NonceSnapshot,
    Pagination, Title, TitleParams, TrustEdge, TrustResult,
};
use crate::wire::{EventTx, IdentityTx, NodeAdvertisementTx, OwnershipStakeWire, TitleTx, TrustTx};
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

    // --- Listing / raw ------------------------------------------------

    /// List known peer nodes. Pagination via optional `limit` / `offset`.
    pub async fn nodes(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let path = format!("nodes{}", pagination_query(limit, offset));
        self.get(&path).await
    }

    /// Raw GET against an arbitrary path (relative to `/api`). Returns
    /// the unparsed response body so callers can inspect the full
    /// JSON envelope or non-success status codes.
    pub async fn raw_get(&self, path: &str) -> Result<Vec<u8>> {
        let url = format!("{}/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        let bytes = resp.bytes().await?;
        if status.is_success() {
            Ok(bytes.to_vec())
        } else {
            Err(Error::Node {
                status: status.as_u16(),
                message: format!(
                    "raw_get status {}: {}",
                    status.as_u16(),
                    truncate(&String::from_utf8_lossy(&bytes), 200)
                ),
            })
        }
    }

    // --- Discovery (QDP-0014) ----------------------------------------

    /// Discover the current consortium / endpoints / tip for `domain`.
    pub async fn discover_domain(&self, domain: &str) -> Result<Value> {
        if domain.is_empty() {
            return Err(Error::validation("domain is required"));
        }
        let path = format!("v2/discovery/domain/{}", urlencoding(domain));
        self.get(&path).await
    }

    /// Discover the raw signed advertisement for a node `quid`.
    pub async fn discover_node(&self, quid: &str) -> Result<Value> {
        if quid.is_empty() {
            return Err(Error::validation("quid is required"));
        }
        let path = format!("v2/discovery/node/{}", urlencoding(quid));
        self.get(&path).await
    }

    /// List all advertisements for a given operator quid.
    pub async fn discover_operator(&self, operator_quid: &str) -> Result<Value> {
        if operator_quid.is_empty() {
            return Err(Error::validation("operatorQuid is required"));
        }
        let path = format!("v2/discovery/operator/{}", urlencoding(operator_quid));
        self.get(&path).await
    }

    /// Query the per-domain quid index.
    pub async fn discover_quids(&self, p: DiscoverQuidsParams) -> Result<Value> {
        if p.domain.is_empty() {
            return Err(Error::validation("domain is required"));
        }
        let mut q: Vec<(String, String)> = Vec::new();
        q.push(("domain".into(), p.domain.clone()));
        if p.since > 0 {
            q.push(("since".into(), p.since.to_string()));
        }
        if !p.sort.is_empty() {
            q.push(("sort".into(), p.sort.clone()));
        }
        if !p.observer.is_empty() {
            q.push(("observer".into(), p.observer.clone()));
        }
        if !p.event_type.is_empty() {
            q.push(("eventType".into(), p.event_type.clone()));
        }
        if p.min_trust_weight > 0.0 {
            q.push(("min-trust-weight".into(), format_float(p.min_trust_weight)));
        }
        if !p.exclude_quids.is_empty() {
            q.push(("excludeQuid".into(), p.exclude_quids.join(",")));
        }
        if p.limit > 0 {
            q.push(("limit".into(), p.limit.to_string()));
        }
        if p.offset > 0 {
            q.push(("offset".into(), p.offset.to_string()));
        }
        let path = format!("v2/discovery/quids{}", encode_query(&q));
        self.get(&path).await
    }

    /// List quids the consortium directly TRUSTs above a threshold.
    pub async fn discover_trusted_quids(
        &self,
        domain: &str,
        min_trust: Option<f64>,
        limit: Option<u32>,
    ) -> Result<Value> {
        if domain.is_empty() {
            return Err(Error::validation("domain is required"));
        }
        let mut q: Vec<(String, String)> = Vec::new();
        q.push(("domain".into(), domain.to_string()));
        if let Some(mt) = min_trust {
            if mt > 0.0 {
                q.push(("min-trust".into(), format_float(mt)));
            }
        }
        if let Some(l) = limit {
            if l > 0 {
                q.push(("limit".into(), l.to_string()));
            }
        }
        let path = format!("v2/discovery/trusted-quids{}", encode_query(&q));
        self.get(&path).await
    }

    // --- Event streams -----------------------------------------------

    /// Return stream metadata, or `None` on 404.
    pub async fn get_event_stream(
        &self,
        subject_id: &str,
        domain: Option<&str>,
    ) -> Result<Option<Value>> {
        let mut path = format!("streams/{}", urlencoding(subject_id));
        if let Some(d) = domain {
            if !d.is_empty() {
                path.push_str(&format!("?domain={}", urlencoding(d)));
            }
        }
        match self.get(&path).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Page through events on a subject's stream, returning the rows
    /// and the pagination envelope.
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        domain: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<(Vec<Event>, Pagination)> {
        let mut q: Vec<(String, String)> = Vec::new();
        if let Some(d) = domain {
            if !d.is_empty() {
                q.push(("domain".into(), d.to_string()));
            }
        }
        if let Some(l) = limit {
            if l > 0 {
                q.push(("limit".into(), l.to_string()));
            }
        }
        if let Some(o) = offset {
            if o > 0 {
                q.push(("offset".into(), o.to_string()));
            }
        }
        let path = format!(
            "streams/{}/events{}",
            urlencoding(subject_id),
            encode_query(&q)
        );
        #[derive(serde::Deserialize, Default)]
        struct Wrap {
            #[serde(default)]
            data: Vec<Event>,
            #[serde(default)]
            events: Vec<Event>,
            #[serde(default)]
            pagination: Pagination,
        }
        let w: Wrap = self.get_typed(&path).await?;
        let rows = if !w.data.is_empty() { w.data } else { w.events };
        Ok((rows, w.pagination))
    }

    // --- Guardians (QDP-0002, QDP-0006) ------------------------------

    /// Install or rotate a guardian set.
    pub async fn submit_guardian_set_update(&self, u: &GuardianSetUpdate) -> Result<Value> {
        let body = serde_json::to_value(u)?;
        self.post("guardian/set-update", &body).await
    }

    /// Start the delayed recovery flow.
    pub async fn submit_recovery_init(&self, i: &GuardianRecoveryInit) -> Result<Value> {
        let body = serde_json::to_value(i)?;
        self.post("guardian/recovery/init", &body).await
    }

    /// Abort an in-flight recovery.
    pub async fn submit_recovery_veto(&self, v: &GuardianRecoveryVeto) -> Result<Value> {
        let body = serde_json::to_value(v)?;
        self.post("guardian/recovery/veto", &body).await
    }

    /// Finalize a recovery after the delay has elapsed.
    pub async fn submit_recovery_commit(&self, c: &GuardianRecoveryCommit) -> Result<Value> {
        let body = serde_json::to_value(c)?;
        self.post("guardian/recovery/commit", &body).await
    }

    /// Voluntary guardian resignation.
    pub async fn submit_guardian_resignation(&self, r: &GuardianResignation) -> Result<Value> {
        let body = serde_json::to_value(r)?;
        self.post("guardian/resign", &body).await
    }

    /// Return the current guardian set for `quid`, or `None` on 404.
    pub async fn get_guardian_set(&self, quid: &str) -> Result<Option<GuardianSet>> {
        let path = format!("guardian/set/{}", urlencoding(quid));
        match self.get_typed::<GuardianSet>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Return the pending-recovery record for `quid`, or `None` on 404.
    pub async fn get_pending_recovery(&self, quid: &str) -> Result<Option<Value>> {
        let path = format!("guardian/pending-recovery/{}", urlencoding(quid));
        match self.get(&path).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    // --- Gossip / fingerprints (QDP-0003, QDP-0005) ------------------

    /// Publish a signed domain fingerprint.
    pub async fn submit_domain_fingerprint(&self, fp: &DomainFingerprint) -> Result<Value> {
        let body = serde_json::to_value(fp)?;
        self.post("domain-fingerprints", &body).await
    }

    /// Return the newest known fingerprint for a domain, or `None` on 404.
    pub async fn get_latest_domain_fingerprint(
        &self,
        domain: &str,
    ) -> Result<Option<DomainFingerprint>> {
        let path = format!("domain-fingerprints/{}/latest", urlencoding(domain));
        match self.get_typed::<DomainFingerprint>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Submit a cross-domain anchor gossip message.
    pub async fn submit_anchor_gossip(&self, m: &AnchorGossipMessage) -> Result<Value> {
        let body = serde_json::to_value(m)?;
        self.post("anchor-gossip", &body).await
    }

    /// Push-mode variant of anchor gossip (QDP-0005).
    pub async fn push_anchor(&self, m: &AnchorGossipMessage) -> Result<Value> {
        let body = serde_json::to_value(m)?;
        self.post("gossip/push-anchor", &body).await
    }

    /// Push-mode variant of fingerprint gossip.
    pub async fn push_fingerprint(&self, fp: &DomainFingerprint) -> Result<Value> {
        let body = serde_json::to_value(fp)?;
        self.post("gossip/push-fingerprint", &body).await
    }

    // --- Bootstrap (QDP-0008) ----------------------------------------

    /// Publish a K-of-K bootstrap snapshot.
    pub async fn submit_nonce_snapshot(&self, s: &NonceSnapshot) -> Result<Value> {
        let body = serde_json::to_value(s)?;
        self.post("nonce-snapshots", &body).await
    }

    /// Return the most recent snapshot for a domain, or `None` on 404.
    pub async fn get_latest_nonce_snapshot(
        &self,
        domain: &str,
    ) -> Result<Option<NonceSnapshot>> {
        let path = format!("nonce-snapshots/{}/latest", urlencoding(domain));
        match self.get_typed::<NonceSnapshot>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Report whether the node is still catching up.
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get("bootstrap/status").await
    }

    // --- Fork-block (QDP-0009) ---------------------------------------

    /// Submit a signed fork-activation block.
    pub async fn submit_fork_block(&self, f: &ForkBlock) -> Result<Value> {
        let body = serde_json::to_value(f)?;
        self.post("fork-block", &body).await
    }

    /// Report fork activation status across features.
    pub async fn fork_block_status(&self) -> Result<Value> {
        self.get("fork-block/status").await
    }

    // --- Blocks, domains, transactions -------------------------------

    /// Return paginated blocks.
    pub async fn get_blocks(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let path = format!("blocks{}", pagination_query(limit, offset));
        self.get(&path).await
    }

    /// Return paginated pending (un-sealed) transactions.
    pub async fn get_pending_transactions(
        &self,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let path = format!("transactions{}", pagination_query(limit, offset));
        self.get(&path).await
    }

    /// Return all known domains on this node.
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    // --- Signed transactions (TITLE / EVENT / NODE_ADVERTISEMENT) ----

    /// Submit a signed TITLE transaction.
    ///
    /// Accepts owner percentages either summing to 1.0 (fraction) or
    /// 100.0 (percent); the latter is normalized to the wire's
    /// fractional convention before signing. The signed bytes use the
    /// same struct-declaration field order as `core.TitleTransaction`.
    pub async fn register_title<'a>(
        &self,
        signer: &Quid,
        p: TitleParams<'a>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if p.asset_id.is_empty() {
            return Err(Error::validation("assetID is required"));
        }
        if p.owners.is_empty() {
            return Err(Error::validation("owners is required"));
        }
        let total: f64 = p.owners.iter().map(|s| s.percentage).sum();
        let scale = if (0.999..=1.001).contains(&total) {
            1.0
        } else if (99.99..=100.01).contains(&total) {
            0.01
        } else {
            return Err(Error::validation(format!(
                "owner percentages must sum to 1.0 (or 100.0 for percent); got {total}"
            )));
        };
        let owners_wire: Vec<OwnershipStakeWire<'_>> = p
            .owners
            .iter()
            .map(|s| OwnershipStakeWire {
                owner_id: s.owner_id.as_str(),
                percentage: s.percentage * scale,
                stake_type: s.stake_type.as_deref().unwrap_or(""),
            })
            .collect();
        let domain = if p.domain.is_empty() { "default" } else { p.domain };
        let _ = p.prev_title_tx_id; // accepted but not on the wire
        let mut tx = TitleTx {
            id: String::new(),
            tx_type: "TITLE",
            trust_domain: domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            asset_id: p.asset_id,
            owners: owners_wire,
            previous_owners: Vec::new(),
            signatures: BTreeMap::new(),
            expiry_date: 0,
            title_type: p.title_type,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/title", &body).await
    }

    /// Submit a signed EVENT transaction.
    ///
    /// Exactly one of `p.payload` or `p.payload_cid` must be set.
    /// When `p.sequence == 0`, the SDK queries the subject's stream
    /// to compute `latest+1` for the caller.
    pub async fn emit_event<'a>(&self, signer: &Quid, p: EventParams<'a>) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if p.subject_type != "QUID" && p.subject_type != "TITLE" {
            return Err(Error::validation(
                r#"subjectType must be "QUID" or "TITLE""#,
            ));
        }
        if p.event_type.is_empty() {
            return Err(Error::validation("eventType is required"));
        }
        let has_payload = p.payload.is_some();
        let has_cid = !p.payload_cid.is_empty();
        if has_payload == has_cid {
            return Err(Error::validation(
                "exactly one of payload or payload_cid is required",
            ));
        }
        let domain = if p.domain.is_empty() { "default" } else { p.domain };
        let sequence = if p.sequence == 0 {
            let mut seq = 1_i64;
            if let Ok(Some(stream)) = self.get_event_stream(p.subject_id, Some(domain)).await {
                if let Some(latest) = stream.get("latestSequence").and_then(|v| v.as_i64()) {
                    seq = latest + 1;
                } else if let Some(latest) = stream.get("latestSequence").and_then(|v| v.as_f64())
                {
                    seq = latest as i64 + 1;
                }
            }
            seq
        } else {
            p.sequence
        };
        let payload_value = p
            .payload
            .as_ref()
            .map(|m| serde_json::to_value(m))
            .transpose()?;
        let mut tx = EventTx {
            id: String::new(),
            tx_type: "EVENT",
            trust_domain: domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            subject_id: p.subject_id,
            subject_type: p.subject_type,
            sequence,
            event_type: p.event_type,
            payload: payload_value,
            payload_cid: p.payload_cid,
            previous_event_id: "",
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("events", &body).await
    }

    /// Build, sign, and submit a QDP-0014 NodeAdvertisementTransaction.
    ///
    /// `signer` is the node's own keypair, so `NodeQuid = signer.id()`.
    /// The `OperatorQuid` must have a current direct TRUST edge
    /// (weight ≥ 0.5) to the node, otherwise the node rejects the
    /// submission.
    pub async fn publish_node_advertisement<'a>(
        &self,
        signer: &Quid,
        p: NodeAdvertisementParams<'a>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if p.operator_quid.is_empty() {
            return Err(Error::validation("operatorQuid is required"));
        }
        if p.endpoints.is_empty() {
            return Err(Error::validation("at least one endpoint is required"));
        }
        if p.advertisement_nonce <= 0 {
            return Err(Error::validation("advertisementNonce must be positive"));
        }
        if p.domain.is_empty() {
            return Err(Error::validation(
                "domain is required (typically operators.network.<your-domain>)",
            ));
        }
        let ttl_seconds = if p.ttl_seconds == 0 {
            6 * 3600
        } else {
            p.ttl_seconds
        };
        if ttl_seconds > 7 * 24 * 3600 {
            return Err(Error::validation("ttl must be <= 7 days"));
        }
        let proto = if p.protocol_version.is_empty() {
            "1.0"
        } else {
            p.protocol_version
        };
        let now_unix = now_secs();
        // Server stores expiresAt in UnixNano, matching the Go SDK.
        let expires_at = (now_unix + ttl_seconds).saturating_mul(1_000_000_000);

        // Pre-assign a random tx ID before signing so the server
        // doesn't re-hash and invalidate the signature.
        let mut id_raw = [0u8; 16];
        rand::Rng::fill(&mut rand::thread_rng(), &mut id_raw[..]);
        let id_hex = hex::encode(id_raw);

        let mut tx = NodeAdvertisementTx {
            id: id_hex,
            tx_type: "NODE_ADVERTISEMENT",
            trust_domain: p.domain,
            timestamp: now_unix,
            signature: String::new(),
            public_key: signer.public_key_hex(),
            node_quid: signer.id(),
            operator_quid: p.operator_quid,
            endpoints: &p.endpoints,
            supported_domains: &p.supported_domains,
            capabilities: &p.capabilities,
            protocol_version: proto,
            expires_at,
            advertisement_nonce: p.advertisement_nonce,
        };
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("node-advertisements", &body).await
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

/// Build a `?limit=&offset=` suffix, omitting unset values. Returns
/// the empty string when both are absent or zero.
fn pagination_query(limit: Option<u32>, offset: Option<u32>) -> String {
    let mut q: Vec<(String, String)> = Vec::new();
    if let Some(l) = limit {
        if l > 0 {
            q.push(("limit".into(), l.to_string()));
        }
    }
    if let Some(o) = offset {
        if o > 0 {
            q.push(("offset".into(), o.to_string()));
        }
    }
    encode_query(&q)
}

/// Encode `(key, value)` pairs into a `?k1=v1&k2=v2` suffix using
/// the same percent-encoding rules as path segments. Returns the
/// empty string for empty input.
fn encode_query(pairs: &[(String, String)]) -> String {
    if pairs.is_empty() {
        return String::new();
    }
    let mut out = String::from("?");
    for (i, (k, v)) in pairs.iter().enumerate() {
        if i > 0 {
            out.push('&');
        }
        out.push_str(&urlencoding(k));
        out.push('=');
        out.push_str(&urlencoding(v));
    }
    out
}

/// Format an `f64` the way Go's `strconv.FormatFloat(x, 'f', -1, 64)`
/// would: shortest decimal that round-trips, no exponent.
fn format_float(v: f64) -> String {
    // Rust's default `{}` for f64 is shortest-round-trip without
    // forced trailing `.0`, except it prefers scientific notation
    // for very small / very large values. For the parameter ranges
    // we send (trust thresholds in [0,1]), `{}` matches Go's `'f'`
    // behavior closely enough.
    let s = format!("{}", v);
    if s.contains('e') || s.contains('E') {
        // Fall back to fixed-decimal for pathological values.
        format!("{:.10}", v)
            .trim_end_matches('0')
            .trim_end_matches('.')
            .to_string()
    } else {
        s
    }
}
