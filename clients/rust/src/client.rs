//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{
    DomainFingerprint, Event, GuardianSet, IdentityRecord, OwnershipStake, Title, TrustEdge,
    TrustResult,
};
use crate::wire::{EventTx, IdentityTx, OwnershipStakeWire, TitleTx, TrustTx};
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
    /// Asset quid ID being titled.
    pub asset_id: &'a str,
    /// Ownership stakes; percentages must sum to 1.0.
    pub owners: Vec<OwnershipStake>,
    /// Trust domain.
    pub domain: &'a str,
    /// Optional title-type discriminator.
    pub title_type: &'a str,
}

/// Parameters for [`Client::emit_event`].
#[derive(Debug, Clone)]
pub struct EventParams<'a> {
    /// Subject quid or title ID the event is anchored to.
    pub subject_id: &'a str,
    /// `"QUID"` or `"TITLE"`.
    pub subject_type: &'a str,
    /// Event-type discriminator.
    pub event_type: &'a str,
    /// Trust domain.
    pub domain: &'a str,
    /// Inline payload (mutually exclusive with `payload_cid`).
    pub payload: Option<serde_json::Value>,
    /// IPFS CID of payload (mutually exclusive with `payload`).
    pub payload_cid: &'a str,
    /// Sequence number; pass `0` to auto-fetch from the stream.
    pub sequence: i64,
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

    /// List known peers (GET /api/nodes).
    pub async fn nodes(&self, limit: u32, offset: u32) -> Result<Value> {
        let mut qs: Vec<String> = Vec::new();
        if limit > 0 { qs.push(format!("limit={limit}")); }
        if offset > 0 { qs.push(format!("offset={offset}")); }
        let path = if qs.is_empty() { "nodes".to_string() }
                   else { format!("nodes?{}", qs.join("&")) };
        self.get(&path).await
    }

    /// Paginated blocks (GET /api/blocks).
    pub async fn blocks(&self, limit: u32, offset: u32) -> Result<Value> {
        let mut qs: Vec<String> = Vec::new();
        if limit > 0 { qs.push(format!("limit={limit}")); }
        if offset > 0 { qs.push(format!("offset={offset}")); }
        let path = if qs.is_empty() { "blocks".to_string() }
                   else { format!("blocks?{}", qs.join("&")) };
        self.get(&path).await
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

    /// Structured relational-trust query (POST /api/trust/query).
    pub async fn query_relational_trust(
        &self,
        observer: &str,
        target: &str,
        domain: &str,
        max_depth: u32,
    ) -> Result<TrustResult> {
        if observer.is_empty() || target.is_empty() {
            return Err(Error::validation("observer and target are required"));
        }
        let mut body = serde_json::json!({
            "observer": observer,
            "target": target,
            "domain": if domain.is_empty() { "default" } else { domain },
        });
        if max_depth > 0 {
            body["maxDepth"] = serde_json::json!(max_depth);
        }
        let v = self.post("trust/query", &body).await?;
        serde_json::from_value(v).map_err(Error::from)
    }

    /// Paginated trust registry (GET /api/registry/trust).
    pub async fn query_trust_registry(
        &self,
        truster: &str,
        trustee: &str,
        limit: u32,
        offset: u32,
    ) -> Result<Value> {
        let mut qs: Vec<String> = Vec::new();
        if !truster.is_empty() { qs.push(format!("truster={}", urlencoding(truster))); }
        if !trustee.is_empty() { qs.push(format!("trustee={}", urlencoding(trustee))); }
        if limit > 0 { qs.push(format!("limit={limit}")); }
        if offset > 0 { qs.push(format!("offset={offset}")); }
        let path = if qs.is_empty() { "registry/trust".to_string() }
                   else { format!("registry/trust?{}", qs.join("&")) };
        self.get(&path).await
    }

    // -----------------------------------------------------------------
    // Title
    // -----------------------------------------------------------------

    /// Submit a signed TITLE transaction (POST /api/transactions/title).
    ///
    /// `owners` percentages may be provided on either the 1.0 (fraction)
    /// or 100.0 (percent) scale; values are normalized to fraction for
    /// the wire (the server's invariant is sum == 1.0).
    pub async fn register_title(&self, signer: &Quid, p: TitleParams<'_>) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if p.asset_id.is_empty() {
            return Err(Error::validation("asset_id is required"));
        }
        if p.owners.is_empty() {
            return Err(Error::validation("owners is required"));
        }
        let total: f64 = p.owners.iter().map(|o| o.percentage).sum();
        let norm: f64 = if (total - 1.0).abs() < 0.001 {
            1.0
        } else if (total - 100.0).abs() < 0.001 {
            0.01
        } else {
            return Err(Error::validation(format!(
                "ownership percentages must sum to 1.0 (or 100.0 for percent); got {total}"
            )));
        };
        let owners_wire: Vec<OwnershipStakeWire> = p
            .owners
            .iter()
            .map(|o| OwnershipStakeWire {
                owner_id: o.owner_id.as_str(),
                percentage: o.percentage * norm,
                stake_type: o.stake_type.as_deref().unwrap_or(""),
            })
            .collect();
        let mut tx = TitleTx {
            id: String::new(),
            tx_type: "TITLE",
            trust_domain: p.domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            asset_id: p.asset_id,
            owners: owners_wire,
            signatures: HashMap::new(),
            title_type: p.title_type,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/title", &body).await
    }

    // -----------------------------------------------------------------
    // Events
    // -----------------------------------------------------------------

    /// Submit a signed EVENT transaction (POST /api/events).
    pub async fn emit_event(&self, signer: &Quid, p: EventParams<'_>) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if p.subject_type != "QUID" && p.subject_type != "TITLE" {
            return Err(Error::validation("subject_type must be 'QUID' or 'TITLE'"));
        }
        if p.event_type.is_empty() {
            return Err(Error::validation("event_type is required"));
        }
        let has_payload = p.payload.is_some();
        let has_cid = !p.payload_cid.is_empty();
        if has_payload == has_cid {
            return Err(Error::validation(
                "exactly one of payload or payload_cid is required",
            ));
        }

        let mut sequence = p.sequence;
        if sequence == 0 {
            match self.get_event_stream(p.subject_id, p.domain).await {
                Ok(Some(stream)) => {
                    sequence = stream
                        .get("latestSequence")
                        .and_then(|v| v.as_i64())
                        .unwrap_or(0)
                        + 1;
                }
                _ => sequence = 1,
            }
        }

        let mut tx = EventTx {
            id: String::new(),
            tx_type: "EVENT",
            trust_domain: p.domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            subject_id: p.subject_id,
            subject_type: p.subject_type,
            sequence,
            event_type: p.event_type,
            payload: p.payload.clone(),
            payload_cid: p.payload_cid,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("events", &body).await
    }

    /// Stream metadata for a subject (GET /api/streams/{subjectId}).
    /// Returns `None` on 404.
    pub async fn get_event_stream(
        &self,
        subject_id: &str,
        domain: &str,
    ) -> Result<Option<Value>> {
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

    /// Paginated events on a subject's stream (GET /api/streams/{subjectId}/events).
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        domain: &str,
        limit: u32,
        offset: u32,
    ) -> Result<Vec<Event>> {
        let mut qs: Vec<String> = Vec::new();
        if !domain.is_empty() { qs.push(format!("domain={}", urlencoding(domain))); }
        if limit > 0 { qs.push(format!("limit={limit}")); }
        if offset > 0 { qs.push(format!("offset={offset}")); }
        let mut path = format!("streams/{}/events", urlencoding(subject_id));
        if !qs.is_empty() {
            path.push('?');
            path.push_str(&qs.join("&"));
        }
        let v = self.get(&path).await?;
        let arr = match &v {
            Value::Array(_) => v.clone(),
            Value::Object(map) => map
                .get("data")
                .or_else(|| map.get("events"))
                .cloned()
                .unwrap_or(Value::Array(Vec::new())),
            _ => Value::Array(Vec::new()),
        };
        serde_json::from_value(arr).map_err(Error::from)
    }

    // -----------------------------------------------------------------
    // Guardians (QDP-0002 / QDP-0006)
    // -----------------------------------------------------------------

    /// Install or rotate a guardian set (POST /api/guardian/set-update).
    pub async fn submit_guardian_set_update(&self, update: &Value) -> Result<Value> {
        self.post("guardian/set-update", update).await
    }

    /// Start delayed recovery (POST /api/guardian/recovery/init).
    pub async fn submit_recovery_init(&self, init: &Value) -> Result<Value> {
        self.post("guardian/recovery/init", init).await
    }

    /// Abort a pending recovery (POST /api/guardian/recovery/veto).
    pub async fn submit_recovery_veto(&self, veto: &Value) -> Result<Value> {
        self.post("guardian/recovery/veto", veto).await
    }

    /// Commit a recovery after delay (POST /api/guardian/recovery/commit).
    pub async fn submit_recovery_commit(&self, commit: &Value) -> Result<Value> {
        self.post("guardian/recovery/commit", commit).await
    }

    /// Guardian resignation (POST /api/guardian/resign).
    pub async fn submit_guardian_resignation(&self, resignation: &Value) -> Result<Value> {
        self.post("guardian/resign", resignation).await
    }

    /// Fetch current guardian set (GET /api/guardian/set/{quid}).
    /// Returns `None` on 404.
    pub async fn get_guardian_set(&self, quid_id: &str) -> Result<Option<GuardianSet>> {
        match self.get_typed::<GuardianSet>(&format!("guardian/set/{}", urlencoding(quid_id))).await {
            Ok(gs) => Ok(Some(gs)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Fetch pending recovery (if any) for a subject quid
    /// (GET /api/guardian/pending-recovery/{quid}). Returns `None` on 404.
    pub async fn get_pending_recovery(&self, quid_id: &str) -> Result<Option<Value>> {
        match self
            .get(&format!("guardian/pending-recovery/{}", urlencoding(quid_id)))
            .await
        {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// List guardian resignations for a subject quid
    /// (GET /api/guardian/resignations/{quid}).
    pub async fn get_guardian_resignations(&self, quid_id: &str) -> Result<Value> {
        self.get(&format!("guardian/resignations/{}", urlencoding(quid_id)))
            .await
    }

    // -----------------------------------------------------------------
    // Cross-domain gossip (QDP-0003)
    // -----------------------------------------------------------------

    /// Publish a signed domain fingerprint (POST /api/domain-fingerprints).
    pub async fn submit_domain_fingerprint(&self, fp: &Value) -> Result<Value> {
        self.post("domain-fingerprints", fp).await
    }

    /// Latest fingerprint for a domain (GET /api/domain-fingerprints/{domain}/latest).
    /// Returns `None` on 404.
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
            Ok(fp) => Ok(Some(fp)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Deliver cross-domain anchor gossip (POST /api/anchor-gossip).
    pub async fn submit_anchor_gossip(&self, message: &Value) -> Result<Value> {
        self.post("anchor-gossip", message).await
    }

    /// Push-gossip anchor variant (POST /api/gossip/push-anchor, QDP-0005).
    pub async fn push_anchor(&self, message: &Value) -> Result<Value> {
        self.post("gossip/push-anchor", message).await
    }

    /// Push-gossip fingerprint variant (POST /api/gossip/push-fingerprint, QDP-0005).
    pub async fn push_fingerprint(&self, fingerprint: &Value) -> Result<Value> {
        self.post("gossip/push-fingerprint", fingerprint).await
    }

    // -----------------------------------------------------------------
    // K-of-K bootstrap (QDP-0008)
    // -----------------------------------------------------------------

    /// Publish a K-of-K bootstrap snapshot (POST /api/nonce-snapshots).
    pub async fn submit_nonce_snapshot(&self, snapshot: &Value) -> Result<Value> {
        self.post("nonce-snapshots", snapshot).await
    }

    /// Latest snapshot for a domain (GET /api/nonce-snapshots/{domain}/latest).
    /// Returns `None` on 404.
    pub async fn get_latest_nonce_snapshot(&self, domain: &str) -> Result<Option<Value>> {
        match self
            .get(&format!("nonce-snapshots/{}/latest", urlencoding(domain)))
            .await
        {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Bootstrap status of this node (GET /api/bootstrap/status).
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get("bootstrap/status").await
    }

    // -----------------------------------------------------------------
    // Fork-block (QDP-0009)
    // -----------------------------------------------------------------

    /// Submit a signed fork-activation block (POST /api/fork-block).
    pub async fn submit_fork_block(&self, fb: &Value) -> Result<Value> {
        self.post("fork-block", fb).await
    }

    /// Fork-activation status across features (GET /api/fork-block/status).
    pub async fn fork_block_status(&self) -> Result<Value> {
        self.get("fork-block/status").await
    }

    // -----------------------------------------------------------------
    // IPFS
    // -----------------------------------------------------------------

    /// Pin raw bytes to IPFS via the node (POST /api/ipfs/pin).
    /// Returns the assigned content identifier (CID).
    pub async fn ipfs_pin(&self, content: Vec<u8>) -> Result<String> {
        let url = format!("{}/ipfs/pin", self.api_base);
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .header("Content-Type", "application/octet-stream")
            .body(content)
            .send()
            .await?;
        let v = parse_envelope(resp).await?;
        v.get("cid")
            .and_then(|c| c.as_str())
            .map(|s| s.to_string())
            .ok_or_else(|| Error::Node {
                status: 0,
                message: "ipfs pin: missing cid in response".to_string(),
            })
    }

    /// Fetch raw bytes from IPFS by CID (GET /api/ipfs/{cid}).
    pub async fn ipfs_get(&self, cid: &str) -> Result<Vec<u8>> {
        if cid.is_empty() {
            return Err(Error::validation("cid is required"));
        }
        let url = format!("{}/ipfs/{}", self.api_base, urlencoding(cid));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        let bytes = resp.bytes().await?;
        if status.is_success() {
            return Ok(bytes.to_vec());
        }
        let body = String::from_utf8_lossy(&bytes).to_string();
        match status.as_u16() {
            404 => Err(Error::Validation("NOT_FOUND: ipfs get: not found".into())),
            s if (400..500).contains(&s) => {
                Err(Error::Validation(format!("ipfs get HTTP {s}: {body}")))
            }
            s => Err(Error::Node {
                status: s,
                message: format!("ipfs get HTTP {s}: {body}"),
            }),
        }
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
