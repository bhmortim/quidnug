//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{
    DomainFingerprint, Event, GuardianSet, IdentityRecord, NonceSnapshot, OwnershipStake, Title,
    TrustEdge, TrustResult,
};
use crate::wire::{EventTx, IdentityTx, TitleOwnerWire, TitleTx, TrustTx};
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

    // --- Health, info, nodes ------------------------------------------

    /// Health check (GET /api/health).
    pub async fn health(&self) -> Result<Value> {
        self.get("health").await
    }

    /// Info (GET /api/info).
    pub async fn info(&self) -> Result<Value> {
        self.get("info").await
    }

    /// GET /api/nodes — list known peers, with optional pagination.
    pub async fn nodes(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let qs = build_query(&[("limit", limit_str(limit)), ("offset", limit_str(offset))]);
        self.get(&format!("nodes{qs}")).await
    }

    // --- Identity -----------------------------------------------------

    /// Register an identity for `signer`, optionally with `name` and `home_domain`.
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

    /// GET /api/registry/identity — paginated identity-registry dump.
    pub async fn query_identity_registry(
        &self,
        domain: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let qs = build_query(&[
            ("domain", domain.map(urlencoding)),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        self.get(&format!("registry/identity{qs}")).await
    }

    // --- Trust --------------------------------------------------------

    /// Submit a signed TRUST transaction.
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

    /// POST /api/trust/query — structured relational trust query.
    pub async fn query_relational_trust(
        &self,
        observer: &str,
        target: &str,
        domain: Option<&str>,
        max_depth: Option<u32>,
        include_unverified: bool,
    ) -> Result<TrustResult> {
        let body = serde_json::json!({
            "observer": observer,
            "target": target,
            "domain": domain.unwrap_or("default"),
            "maxDepth": max_depth.unwrap_or(5),
            "includeUnverified": include_unverified,
        });
        self.post_typed("trust/query", &body).await
    }

    /// GET /api/registry/trust — paginated trust-edge listing.
    pub async fn query_trust_registry(
        &self,
        domain: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let qs = build_query(&[
            ("domain", domain.map(urlencoding)),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        self.get(&format!("registry/trust{qs}")).await
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

    // --- Title --------------------------------------------------------

    /// Submit a signed TITLE transaction.
    ///
    /// `owners` percentages may be on the 1.0 (fraction) or 100.0
    /// (percent) scale; values are normalized to fraction on the
    /// wire (server invariant: sum == 1.0).
    pub async fn register_title(
        &self,
        signer: &Quid,
        asset_id: &str,
        owners: Vec<OwnershipStake>,
        domain: &str,
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
        let norm = if (total - 1.0).abs() < 0.001 {
            1.0
        } else if (total - 100.0).abs() < 0.001 {
            0.01
        } else {
            return Err(Error::validation(format!(
                "ownership percentages must sum to 1.0 (or 100.0 for percent); got {total}"
            )));
        };
        let stake_types: Vec<String> = owners
            .iter()
            .map(|s| s.stake_type.clone().unwrap_or_default())
            .collect();
        let wire_owners: Vec<TitleOwnerWire<'_>> = owners
            .iter()
            .zip(stake_types.iter())
            .map(|(s, st)| TitleOwnerWire {
                owner_id: s.owner_id.as_str(),
                percentage: s.percentage * norm,
                stake_type: st.as_str(),
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
            title_type: "",
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/title", &body).await
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

    /// GET /api/registry/title — paginated title-registry dump.
    pub async fn query_title_registry(
        &self,
        domain: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let qs = build_query(&[
            ("domain", domain.map(urlencoding)),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        self.get(&format!("registry/title{qs}")).await
    }

    // --- Events -------------------------------------------------------

    /// Submit an EVENT transaction. Either `payload` (inline) or
    /// `payload_cid` (IPFS-resolved) must be set, but not both.
    pub async fn emit_event(
        &self,
        signer: &Quid,
        subject_id: &str,
        subject_type: &str,
        event_type: &str,
        domain: &str,
        payload: Option<Value>,
        payload_cid: Option<&str>,
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

        // Best-effort sequence resolution: previous-tail + 1, else 1.
        let sequence = match self.get_event_stream(subject_id, Some(domain)).await {
            Ok(Some(stream)) => stream
                .get("latestSequence")
                .and_then(|v| v.as_i64())
                .map(|s| s + 1)
                .unwrap_or(1),
            _ => 1,
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
            sequence,
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

    /// GET /api/streams/{subjectId} — stream metadata or `None` on 404.
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

    /// GET /api/streams/{subjectId}/events — paginated event listing.
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        domain: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Vec<Event>> {
        let qs = build_query(&[
            ("domain", domain.map(urlencoding)),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        let path = format!("streams/{}/events{}", urlencoding(subject_id), qs);
        let data = self.get(&path).await?;
        let raw = data
            .get("data")
            .or_else(|| data.get("events"))
            .cloned()
            .unwrap_or(Value::Null);
        match raw {
            Value::Array(items) => items
                .into_iter()
                .map(|v| serde_json::from_value::<Event>(v).map_err(Error::from))
                .collect(),
            _ => Ok(Vec::new()),
        }
    }

    // --- IPFS ---------------------------------------------------------

    /// POST /api/ipfs/pin — returns the CID of pinned content.
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

    /// GET /api/ipfs/{cid} — fetch raw bytes (no envelope unwrapping).
    pub async fn ipfs_get(&self, cid: &str) -> Result<Vec<u8>> {
        let url = format!("{}/ipfs/{}", self.api_base, urlencoding(cid));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        if status.is_success() {
            let bytes = resp.bytes().await?;
            Ok(bytes.to_vec())
        } else {
            let body = resp.text().await.unwrap_or_default();
            Err(Error::Node {
                status: status.as_u16(),
                message: format!("IPFS fetch failed: {}", truncate(&body, 200)),
            })
        }
    }

    // --- Guardians (QDP-0002 / QDP-0006) ------------------------------

    /// POST /api/guardian/set-update — install or rotate guardians.
    pub async fn submit_guardian_set_update(&self, update: Value) -> Result<Value> {
        self.post("guardian/set-update", &update).await
    }

    /// POST /api/guardian/recovery/init — start the recovery delay.
    pub async fn submit_recovery_init(&self, init: Value) -> Result<Value> {
        self.post("guardian/recovery/init", &init).await
    }

    /// POST /api/guardian/recovery/veto — abort an in-flight recovery.
    pub async fn submit_recovery_veto(&self, veto: Value) -> Result<Value> {
        self.post("guardian/recovery/veto", &veto).await
    }

    /// POST /api/guardian/recovery/commit — finalize a delayed recovery.
    pub async fn submit_recovery_commit(&self, commit: Value) -> Result<Value> {
        self.post("guardian/recovery/commit", &commit).await
    }

    /// POST /api/guardian/resign — guardian leaves the set.
    pub async fn submit_guardian_resignation(&self, r: Value) -> Result<Value> {
        self.post("guardian/resign", &r).await
    }

    /// GET /api/guardian/set/{quid} — current guardian set or `None`.
    pub async fn get_guardian_set(&self, quid: &str) -> Result<Option<GuardianSet>> {
        let path = format!("guardian/set/{}", urlencoding(quid));
        match self.get_typed::<GuardianSet>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/pending-recovery/{quid}.
    pub async fn get_pending_recovery(&self, quid: &str) -> Result<Option<Value>> {
        let path = format!("guardian/pending-recovery/{}", urlencoding(quid));
        match self.get(&path).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/resignations/{quid}.
    pub async fn get_guardian_resignations(&self, quid: &str) -> Result<Vec<Value>> {
        let path = format!("guardian/resignations/{}", urlencoding(quid));
        let data = self.get(&path).await?;
        let raw = data
            .get("data")
            .or_else(|| data.get("resignations"))
            .cloned()
            .unwrap_or(Value::Null);
        match raw {
            Value::Array(items) => Ok(items),
            _ => Ok(Vec::new()),
        }
    }

    // --- Gossip + fingerprints (QDP-0003 / QDP-0005) ------------------

    /// POST /api/domain-fingerprints — publish a signed fingerprint.
    pub async fn submit_domain_fingerprint(&self, fp: Value) -> Result<Value> {
        self.post("domain-fingerprints", &fp).await
    }

    /// GET /api/domain-fingerprints/{domain}/latest.
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

    /// POST /api/anchor-gossip — deliver a cross-domain anchor message.
    pub async fn submit_anchor_gossip(&self, msg: Value) -> Result<Value> {
        self.post("anchor-gossip", &msg).await
    }

    /// POST /api/gossip/push-anchor — push gossip variant (QDP-0005).
    pub async fn push_anchor(&self, msg: Value) -> Result<Value> {
        self.post("gossip/push-anchor", &msg).await
    }

    /// POST /api/gossip/push-fingerprint — push gossip variant (QDP-0005).
    pub async fn push_fingerprint(&self, fp: Value) -> Result<Value> {
        self.post("gossip/push-fingerprint", &fp).await
    }

    // --- Bootstrap + nonce snapshots (QDP-0008) -----------------------

    /// POST /api/nonce-snapshots — publish a K-of-K bootstrap snapshot.
    pub async fn submit_nonce_snapshot(&self, s: Value) -> Result<Value> {
        self.post("nonce-snapshots", &s).await
    }

    /// GET /api/nonce-snapshots/{domain}/latest.
    pub async fn get_latest_nonce_snapshot(&self, domain: &str) -> Result<Option<NonceSnapshot>> {
        let path = format!("nonce-snapshots/{}/latest", urlencoding(domain));
        match self.get_typed::<NonceSnapshot>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/bootstrap/status — node bootstrap-phase status.
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get("bootstrap/status").await
    }

    // --- Fork-block (QDP-0009) ----------------------------------------

    /// POST /api/fork-block — submit a signed fork-activation block.
    pub async fn submit_fork_block(&self, fb: Value) -> Result<Value> {
        self.post("fork-block", &fb).await
    }

    /// GET /api/fork-block/status — activation status across features.
    pub async fn fork_block_status(&self) -> Result<Value> {
        self.get("fork-block/status").await
    }

    // --- Blocks + pending transactions --------------------------------

    /// GET /api/blocks — paginated block listing.
    pub async fn get_blocks(&self, limit: Option<u32>, offset: Option<u32>) -> Result<Value> {
        let qs = build_query(&[("limit", limit_str(limit)), ("offset", limit_str(offset))]);
        self.get(&format!("blocks{qs}")).await
    }

    /// GET /api/blocks/tentative/{domain}.
    pub async fn get_tentative_blocks(&self, domain: &str) -> Result<Value> {
        self.get(&format!("blocks/tentative/{}", urlencoding(domain)))
            .await
    }

    /// GET /api/transactions — pending mempool listing.
    pub async fn get_pending_transactions(
        &self,
        domain: Option<&str>,
        limit: Option<u32>,
        offset: Option<u32>,
    ) -> Result<Value> {
        let qs = build_query(&[
            ("domain", domain.map(urlencoding)),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        self.get(&format!("transactions{qs}")).await
    }

    // --- Domains ------------------------------------------------------

    /// GET /api/domains — known-domain listing.
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    /// Register a new trust domain. Fails with an "already exists"
    /// error if the domain is already known; see [`Self::ensure_domain`]
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

    /// GET /api/node/domains — domains this node currently manages.
    pub async fn get_node_domains(&self) -> Result<Value> {
        self.get("node/domains").await
    }

    /// POST /api/node/domains — update the node's managed-domain list.
    pub async fn update_node_domains(&self, domains: Vec<String>) -> Result<Value> {
        let body = serde_json::json!({ "managedDomains": domains });
        self.post("node/domains", &body).await
    }

    // --- Commit-wait helpers ------------------------------------------
    //
    // Identity and title transactions live in the node's pending
    // pool until the next block is sealed. Code that immediately
    // emits events or title transactions referencing the new quid
    // must wait for commit first; these helpers poll until the
    // record is visible in the committed registry.

    /// Block until the identity with `quid_id` is visible in the
    /// committed registry, or return a validation error when the
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
    /// committed registry.
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

    async fn post_typed<T: DeserializeOwned>(&self, path: &str, body: &Value) -> Result<T> {
        let url = format!("{}/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .json(body)
            .send()
            .await?;
        parse_envelope_typed(resp).await
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

fn limit_str(v: Option<u32>) -> Option<String> {
    v.map(|n| n.to_string())
}

/// Build a `?k=v&...` query string from already-URL-encoded values.
/// Each `(key, Option<value>)` pair is emitted only if `Some`; the
/// value is taken verbatim (caller is responsible for any encoding
/// beyond the simple path-segment helper above).
fn build_query(parts: &[(&str, Option<String>)]) -> String {
    let mut bits: Vec<String> = Vec::new();
    for (k, v) in parts {
        if let Some(val) = v {
            bits.push(format!("{k}={val}"));
        }
    }
    if bits.is_empty() {
        String::new()
    } else {
        format!("?{}", bits.join("&"))
    }
}
