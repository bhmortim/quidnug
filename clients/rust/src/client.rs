//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{
    AnchorGossipMessage, DomainFingerprint, Event, ForkBlock, GuardianRecoveryCommit,
    GuardianRecoveryInit, GuardianRecoveryVeto, GuardianResignation, GuardianSet,
    GuardianSetUpdate, IdentityRecord, NonceSnapshot, OwnershipStake, Title, TrustEdge, TrustResult,
};
use crate::wire::{EventTx, IdentityTx, OwnershipStakeWire, TitleTx, TrustTx};
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

/// Parameters for [`Client::register_title`].
#[derive(Debug, Clone)]
pub struct TitleParams<'a> {
    /// Asset quid ID being titled.
    pub asset_id: &'a str,
    /// Ownership map. Percentages may be on the 1.0 (fraction) or
    /// 100.0 (percent) scale and are normalized to fractions on
    /// the wire.
    pub owners: Vec<OwnershipStake>,
    /// Domain. Defaults to `"default"`.
    pub domain: &'a str,
    /// Optional title-type discriminator.
    pub title_type: &'a str,
}

/// Parameters for [`Client::emit_event`].
///
/// Exactly one of `payload` or `payload_cid` must be set.
#[derive(Debug, Clone)]
pub struct EventParams<'a> {
    /// Subject quid (`QUID`) or asset id (`TITLE`).
    pub subject_id: &'a str,
    /// `"QUID"` or `"TITLE"`.
    pub subject_type: &'a str,
    /// Event-type discriminator.
    pub event_type: &'a str,
    /// Domain. Defaults to `"default"`.
    pub domain: &'a str,
    /// Inline payload. Mutually exclusive with `payload_cid`.
    pub payload: Option<Value>,
    /// IPFS CID of payload. Mutually exclusive with `payload`.
    pub payload_cid: &'a str,
    /// `0` = auto-detect by querying the subject's stream.
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

    // --- Nodes, blocks, transactions ----------------------------------

    /// GET /api/nodes — list known peers.
    pub async fn nodes(&self, limit: Option<i64>, offset: Option<i64>) -> Result<Value> {
        self.get(&with_query("nodes", &[("limit", limit), ("offset", offset)]))
            .await
    }

    /// GET /api/blocks — committed blocks.
    pub async fn get_blocks(&self, limit: Option<i64>, offset: Option<i64>) -> Result<Value> {
        self.get(&with_query(
            "blocks",
            &[("limit", limit), ("offset", offset)],
        ))
        .await
    }

    /// GET /api/blocks/tentative/{domain} — uncommitted block view.
    pub async fn get_tentative_blocks(&self, domain: &str) -> Result<Value> {
        self.get(&format!("blocks/tentative/{}", urlencoding(domain)))
            .await
    }

    /// GET /api/transactions — pending (mempool) transactions.
    pub async fn get_pending_transactions(
        &self,
        limit: Option<i64>,
        offset: Option<i64>,
    ) -> Result<Value> {
        self.get(&with_query(
            "transactions",
            &[("limit", limit), ("offset", offset)],
        ))
        .await
    }

    // --- Domains -------------------------------------------------------

    /// GET /api/domains — list all known trust domains.
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    /// GET /api/node/domains — domains this node manages.
    pub async fn get_node_domains(&self) -> Result<Value> {
        self.get("node/domains").await
    }

    /// POST /api/node/domains — replace the node's managed-domain set.
    pub async fn update_node_domains(&self, domains: &[String]) -> Result<Value> {
        let body = serde_json::json!({ "managedDomains": domains });
        self.post("node/domains", &body).await
    }

    // --- Registry queries ----------------------------------------------

    /// GET /api/registry/identity — paginated dump / single lookup.
    pub async fn query_identity_registry(
        &self,
        limit: Option<i64>,
        offset: Option<i64>,
        quid_id: Option<&str>,
    ) -> Result<Value> {
        let mut q: Vec<(&str, String)> = Vec::new();
        if let Some(l) = limit {
            q.push(("limit", l.to_string()));
        }
        if let Some(o) = offset {
            q.push(("offset", o.to_string()));
        }
        if let Some(qid) = quid_id {
            q.push(("quid_id", qid.to_string()));
        }
        self.get(&with_query_str("registry/identity", &q)).await
    }

    /// GET /api/registry/trust — paginated trust-edge listing.
    pub async fn query_trust_registry(
        &self,
        limit: Option<i64>,
        offset: Option<i64>,
        truster: Option<&str>,
        trustee: Option<&str>,
    ) -> Result<Value> {
        let mut q: Vec<(&str, String)> = Vec::new();
        if let Some(l) = limit {
            q.push(("limit", l.to_string()));
        }
        if let Some(o) = offset {
            q.push(("offset", o.to_string()));
        }
        if let Some(t) = truster {
            q.push(("truster", t.to_string()));
        }
        if let Some(t) = trustee {
            q.push(("trustee", t.to_string()));
        }
        self.get(&with_query_str("registry/trust", &q)).await
    }

    /// GET /api/registry/title — paginated title listing.
    pub async fn query_title_registry(
        &self,
        limit: Option<i64>,
        offset: Option<i64>,
        asset_id: Option<&str>,
        owner_id: Option<&str>,
    ) -> Result<Value> {
        let mut q: Vec<(&str, String)> = Vec::new();
        if let Some(l) = limit {
            q.push(("limit", l.to_string()));
        }
        if let Some(o) = offset {
            q.push(("offset", o.to_string()));
        }
        if let Some(a) = asset_id {
            q.push(("asset_id", a.to_string()));
        }
        if let Some(o) = owner_id {
            q.push(("owner_id", o.to_string()));
        }
        self.get(&with_query_str("registry/title", &q)).await
    }

    /// POST /api/trust/query — structured relational trust query.
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
        let data = self.post("trust/query", &body).await?;
        Ok(serde_json::from_value(data)?)
    }

    /// GET /api/domains/{name}/query — read-side domain query.
    pub async fn query_domain(
        &self,
        domain: &str,
        type_: &str,
        param: &str,
    ) -> Result<Value> {
        let path = format!(
            "domains/{}/query?type={}&param={}",
            urlencoding(domain),
            urlencoding(type_),
            urlencoding(param)
        );
        self.get(&path).await
    }

    // --- Title / events -----------------------------------------------

    /// Submit a TITLE transaction. Owner percentages may be supplied
    /// on either the 1.0 (fraction) or 100.0 (percent) scale; they're
    /// normalized to fractions for the wire (server invariant: sum == 1).
    pub async fn register_title<'a>(
        &self,
        signer: &Quid,
        params: TitleParams<'a>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if params.asset_id.is_empty() {
            return Err(Error::validation("asset_id is required"));
        }
        if params.owners.is_empty() {
            return Err(Error::validation("owners is required"));
        }
        let total: f64 = params.owners.iter().map(|s| s.percentage).sum();
        let norm: f64 = if (total - 1.0).abs() < 0.001 {
            1.0
        } else if (total - 100.0).abs() < 0.001 {
            0.01
        } else {
            return Err(Error::validation(format!(
                "ownership percentages must sum to 1.0 (or 100.0); got {total}"
            )));
        };
        let wire_owners: Vec<OwnershipStakeWire> = params
            .owners
            .iter()
            .map(|s| OwnershipStakeWire {
                owner_id: &s.owner_id,
                percentage: s.percentage * norm,
                stake_type: s.stake_type.as_deref().unwrap_or(""),
            })
            .collect();
        let domain = if params.domain.is_empty() {
            "default"
        } else {
            params.domain
        };
        let mut tx = TitleTx {
            id: String::new(),
            tx_type: "TITLE",
            trust_domain: domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            asset_id: params.asset_id,
            owners: wire_owners,
            previous_owners: Vec::new(),
            signatures: BTreeMap::new(),
            expiry_date: 0,
            title_type: params.title_type,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/title", &body).await
    }

    /// Submit an EVENT transaction. Signer must own `subject_id`.
    ///
    /// Exactly one of `params.payload` or `params.payload_cid` must
    /// be provided.  When `params.sequence == 0` the method queries
    /// the subject's stream to auto-detect the next sequence — mirrors
    /// the Python `emit_event` behavior.
    pub async fn emit_event<'a>(
        &self,
        signer: &Quid,
        params: EventParams<'a>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if !matches!(params.subject_type, "QUID" | "TITLE") {
            return Err(Error::validation(
                "subject_type must be 'QUID' or 'TITLE'",
            ));
        }
        if params.event_type.is_empty() {
            return Err(Error::validation("event_type is required"));
        }
        let has_payload = params.payload.is_some();
        let has_cid = !params.payload_cid.is_empty();
        if has_payload == has_cid {
            return Err(Error::validation(
                "exactly one of payload or payload_cid is required",
            ));
        }

        let domain = if params.domain.is_empty() {
            "default"
        } else {
            params.domain
        };

        let sequence = if params.sequence > 0 {
            params.sequence
        } else {
            match self.get_event_stream(params.subject_id, domain).await {
                Ok(Some(stream)) => stream
                    .get("latestSequence")
                    .and_then(|v| v.as_i64())
                    .unwrap_or(0)
                    + 1,
                _ => 1,
            }
        };

        let mut tx = EventTx {
            id: String::new(),
            tx_type: "EVENT",
            trust_domain: domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            subject_id: params.subject_id,
            subject_type: params.subject_type,
            sequence,
            event_type: params.event_type,
            payload: params.payload.clone(),
            payload_cid: params.payload_cid,
            previous_event_id: "",
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("events", &body).await
    }

    /// GET /api/streams/{subject} — stream metadata or `None` on 404.
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
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/streams/{subject}/events — paginated stream events.
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        domain: &str,
        limit: Option<i64>,
        offset: Option<i64>,
    ) -> Result<Vec<Event>> {
        let mut q: Vec<(&str, String)> = Vec::new();
        if !domain.is_empty() {
            q.push(("domain", domain.to_string()));
        }
        if let Some(l) = limit {
            q.push(("limit", l.to_string()));
        }
        if let Some(o) = offset {
            q.push(("offset", o.to_string()));
        }
        let path = format!("streams/{}/events", urlencoding(subject_id));
        let data = self.get(&with_query_str(&path, &q)).await?;
        // Server may envelope events at `data.data` or `data.events`,
        // or pass the array through directly.
        let arr = match &data {
            Value::Array(a) => a.clone(),
            Value::Object(m) => match m.get("data").or_else(|| m.get("events")) {
                Some(Value::Array(a)) => a.clone(),
                _ => Vec::new(),
            },
            _ => Vec::new(),
        };
        let mut events = Vec::with_capacity(arr.len());
        for v in arr {
            events.push(serde_json::from_value(v)?);
        }
        Ok(events)
    }

    // --- IPFS ----------------------------------------------------------

    /// POST /api/ipfs/pin — pin raw bytes; returns the CID.
    pub async fn ipfs_pin(&self, content: &[u8]) -> Result<String> {
        let url = format!("{}/{}", self.api_base, "ipfs/pin");
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
            .and_then(|v| v.as_str())
            .or_else(|| data.get("value").and_then(|v| v.as_str()))
            .ok_or_else(|| Error::UnexpectedResponse("IPFS pin missing cid".into()))?
            .to_string();
        Ok(cid)
    }

    /// GET /api/ipfs/{cid} — fetch raw bytes.
    pub async fn ipfs_get(&self, cid: &str) -> Result<Vec<u8>> {
        let url = format!("{}/ipfs/{}", self.api_base, urlencoding(cid));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        if status.is_client_error() || status.is_server_error() {
            let body = resp.text().await.unwrap_or_default();
            return Err(Error::Node {
                status: status.as_u16(),
                message: format!("IPFS fetch failed: {}", truncate(&body, 200)),
            });
        }
        let bytes = resp.bytes().await?;
        Ok(bytes.to_vec())
    }

    // --- Guardian sets + recovery (QDP-0002 / QDP-0006) ----------------

    /// POST /api/guardian/set-update — install or rotate guardians.
    pub async fn submit_guardian_set_update(
        &self,
        update: &GuardianSetUpdate,
    ) -> Result<Value> {
        self.post("guardian/set-update", &serde_json::to_value(update)?)
            .await
    }

    /// POST /api/guardian/recovery/init — start the M-of-N recovery delay.
    pub async fn submit_recovery_init(
        &self,
        init: &GuardianRecoveryInit,
    ) -> Result<Value> {
        self.post("guardian/recovery/init", &serde_json::to_value(init)?)
            .await
    }

    /// POST /api/guardian/recovery/veto — abort an in-flight recovery.
    pub async fn submit_recovery_veto(
        &self,
        veto: &GuardianRecoveryVeto,
    ) -> Result<Value> {
        self.post("guardian/recovery/veto", &serde_json::to_value(veto)?)
            .await
    }

    /// POST /api/guardian/recovery/commit — finalize the delayed recovery.
    pub async fn submit_recovery_commit(
        &self,
        commit: &GuardianRecoveryCommit,
    ) -> Result<Value> {
        self.post("guardian/recovery/commit", &serde_json::to_value(commit)?)
            .await
    }

    /// POST /api/guardian/resign — a guardian leaves the set.
    pub async fn submit_guardian_resignation(
        &self,
        resignation: &GuardianResignation,
    ) -> Result<Value> {
        self.post("guardian/resign", &serde_json::to_value(resignation)?)
            .await
    }

    /// GET /api/guardian/set/{quid} — current guardian set or `None`.
    pub async fn get_guardian_set(&self, quid_id: &str) -> Result<Option<GuardianSet>> {
        let path = format!("guardian/set/{}", urlencoding(quid_id));
        match self.get_typed::<GuardianSet>(&path).await {
            Ok(s) => Ok(Some(s)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/pending-recovery/{quid} — in-flight recovery
    /// for `quid_id`, or `None` if there is none.
    pub async fn get_pending_recovery(&self, quid_id: &str) -> Result<Option<Value>> {
        let path = format!("guardian/pending-recovery/{}", urlencoding(quid_id));
        match self.get(&path).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/resignations/{quid} — guardian-resignation history.
    pub async fn get_guardian_resignations(&self, quid_id: &str) -> Result<Vec<Value>> {
        let path = format!("guardian/resignations/{}", urlencoding(quid_id));
        let data = self.get(&path).await?;
        let arr = match &data {
            Value::Array(a) => a.clone(),
            Value::Object(m) => match m.get("data").or_else(|| m.get("resignations")) {
                Some(Value::Array(a)) => a.clone(),
                _ => Vec::new(),
            },
            _ => Vec::new(),
        };
        Ok(arr)
    }

    // --- Cross-domain gossip + fingerprints (QDP-0003 / QDP-0005) ------

    /// POST /api/domain-fingerprints — publish a signed fingerprint.
    pub async fn submit_domain_fingerprint(
        &self,
        fp: &DomainFingerprint,
    ) -> Result<Value> {
        self.post("domain-fingerprints", &serde_json::to_value(fp)?)
            .await
    }

    /// GET /api/domain-fingerprints/{domain}/latest — latest fingerprint
    /// for a domain, or `None` if one has never been published.
    pub async fn get_latest_domain_fingerprint(
        &self,
        domain: &str,
    ) -> Result<Option<DomainFingerprint>> {
        let path = format!("domain-fingerprints/{}/latest", urlencoding(domain));
        match self.get_typed::<DomainFingerprint>(&path).await {
            Ok(fp) => Ok(Some(fp)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// POST /api/anchor-gossip — deliver a cross-domain anchor message.
    /// The node is idempotent on `message_id`; resubmissions return
    /// `data.duplicate=true`.
    pub async fn submit_anchor_gossip(
        &self,
        message: &AnchorGossipMessage,
    ) -> Result<Value> {
        self.post("anchor-gossip", &serde_json::to_value(message)?)
            .await
    }

    /// POST /api/gossip/push-anchor — QDP-0005 push variant.
    pub async fn push_anchor(&self, message: &AnchorGossipMessage) -> Result<Value> {
        self.post("gossip/push-anchor", &serde_json::to_value(message)?)
            .await
    }

    /// POST /api/gossip/push-fingerprint — QDP-0005 push variant.
    pub async fn push_fingerprint(&self, fp: &DomainFingerprint) -> Result<Value> {
        self.post("gossip/push-fingerprint", &serde_json::to_value(fp)?)
            .await
    }

    // --- Bootstrap (QDP-0008) ------------------------------------------

    /// POST /api/nonce-snapshots — publish a K-of-K bootstrap snapshot.
    pub async fn submit_nonce_snapshot(
        &self,
        snapshot: &NonceSnapshot,
    ) -> Result<Value> {
        self.post("nonce-snapshots", &serde_json::to_value(snapshot)?)
            .await
    }

    /// GET /api/nonce-snapshots/{domain}/latest.
    pub async fn get_latest_nonce_snapshot(
        &self,
        domain: &str,
    ) -> Result<Option<NonceSnapshot>> {
        let path = format!("nonce-snapshots/{}/latest", urlencoding(domain));
        match self.get_typed::<NonceSnapshot>(&path).await {
            Ok(s) => Ok(Some(s)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/bootstrap/status.
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get("bootstrap/status").await
    }

    // --- Fork-block (QDP-0009) -----------------------------------------

    /// POST /api/fork-block — submit a signed fork-activation block.
    pub async fn submit_fork_block(&self, fb: &ForkBlock) -> Result<Value> {
        self.post("fork-block", &serde_json::to_value(fb)?).await
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

/// Build a `path?k=v&...` URL fragment, omitting `None` values.
fn with_query(path: &str, params: &[(&str, Option<i64>)]) -> String {
    let parts: Vec<String> = params
        .iter()
        .filter_map(|(k, v)| v.map(|v| format!("{}={}", k, v)))
        .collect();
    if parts.is_empty() {
        path.to_string()
    } else {
        format!("{}?{}", path, parts.join("&"))
    }
}

/// Build a `path?k=v&...` URL fragment from string params.
fn with_query_str(path: &str, params: &[(&str, String)]) -> String {
    if params.is_empty() {
        return path.to_string();
    }
    let parts: Vec<String> = params
        .iter()
        .map(|(k, v)| format!("{}={}", k, urlencoding(v)))
        .collect();
    format!("{}?{}", path, parts.join("&"))
}
