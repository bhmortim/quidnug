//! Integration tests for the HTTP client against wiremock.

use quidnug::{Client, Error, Quid, TrustParams};
use serde_json::json;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[tokio::test]
async fn grant_trust_posts_correct_envelope() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/transactions/trust"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "txId": "abc" }
        })))
        .mount(&server)
        .await;

    let client = Client::new(&server.uri()).unwrap();
    let q = Quid::generate();
    let res = client
        .grant_trust(
            &q,
            TrustParams {
                trustee: "bob",
                level: 0.9,
                domain: "demo.home",
                nonce: 1,
            },
        )
        .await
        .unwrap();
    assert_eq!(res.get("txId").unwrap().as_str().unwrap(), "abc");
}

#[tokio::test]
async fn error_envelope_409_raises_conflict() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/transactions/trust"))
        .respond_with(ResponseTemplate::new(409).set_body_json(json!({
            "success": false,
            "error": { "code": "NONCE_REPLAY", "message": "stale nonce" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let q = Quid::generate();
    let err = client
        .grant_trust(
            &q,
            TrustParams {
                trustee: "bob",
                level: 0.9,
                domain: "demo.home",
                nonce: 1,
            },
        )
        .await
        .unwrap_err();
    match err {
        Error::Conflict { code, .. } => assert_eq!(code, "NONCE_REPLAY"),
        _ => panic!("expected Conflict, got {err:?}"),
    }
}

#[tokio::test]
async fn unavailable_on_503() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/health"))
        .respond_with(ResponseTemplate::new(503).set_body_json(json!({
            "success": false,
            "error": { "code": "BOOTSTRAPPING", "message": "warming up" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let err = client.health().await.unwrap_err();
    assert!(matches!(err, Error::Unavailable { .. }));
}

#[tokio::test]
async fn level_validation_fails_before_network() {
    let client = Client::new("http://127.0.0.1:1").unwrap();
    let q = Quid::generate();
    let err = client
        .grant_trust(
            &q,
            TrustParams {
                trustee: "bob",
                level: 1.5,
                domain: "x",
                nonce: 1,
            },
        )
        .await
        .unwrap_err();
    assert!(matches!(err, Error::Validation(_)));
}

// ---------------------------------------------------------------
// Domain + commit-wait helpers (against wiremock).
// ---------------------------------------------------------------

#[tokio::test]
async fn ensure_domain_swallows_already_exists() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/domains"))
        .respond_with(ResponseTemplate::new(400).set_body_json(json!({
            "success": false,
            "error": { "code": "BAD_REQUEST", "message": "trust domain test.dom already exists" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let out = client.ensure_domain("test.dom").await.unwrap();
    assert_eq!(out["status"], "success");
    assert_eq!(out["domain"], "test.dom");
}

#[tokio::test]
async fn ensure_domain_propagates_other_errors() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/domains"))
        .respond_with(ResponseTemplate::new(500).set_body_json(json!({
            "success": false,
            "error": { "code": "INTERNAL", "message": "database connection lost" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    assert!(client.ensure_domain("test.dom").await.is_err());
}

#[tokio::test]
async fn wait_for_identity_returns_once_committed() {
    let server = MockServer::start().await;
    // First two polls 404, third returns the identity.
    Mock::given(method("GET"))
        .and(path("/api/identity/abc123"))
        .respond_with(ResponseTemplate::new(404).set_body_json(json!({
            "success": false,
            "error": { "code": "NOT_FOUND", "message": "identity not found" }
        })))
        .up_to_n_times(2)
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/identity/abc123"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true,
            "data": {
                "quidId": "abc123",
                "publicKey": "04...",
                "creator": "abc123",
                "updateNonce": 1,
                "name": "alice"
            }
        })))
        .mount(&server)
        .await;

    let client = Client::new(&server.uri()).unwrap();
    let rec = client
        .wait_for_identity(
            "abc123",
            "",
            std::time::Duration::from_secs(5),
            std::time::Duration::from_millis(50),
        )
        .await
        .unwrap();
    assert_eq!(rec.quid_id, "abc123");
}

#[tokio::test]
async fn wait_for_identity_respects_deadline() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/identity/nope"))
        .respond_with(ResponseTemplate::new(404).set_body_json(json!({
            "success": false,
            "error": { "code": "NOT_FOUND", "message": "none" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let err = client
        .wait_for_identity(
            "nope",
            "",
            std::time::Duration::from_millis(200),
            std::time::Duration::from_millis(50),
        )
        .await
        .unwrap_err();
    let s = format!("{}", err).to_lowercase();
    assert!(
        s.contains("did not commit") || s.contains("timeout"),
        "unexpected error: {}",
        s
    );
}

// --- Coverage tests for the v1 + v2 surface added in the audit sweep ----

#[tokio::test]
async fn peers_hits_peers_endpoint() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/peers"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "peers": [], "count": 0 }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let r = client.peers().await.unwrap();
    assert_eq!(r.get("count").unwrap().as_u64().unwrap(), 0);
}

#[tokio::test]
async fn get_peer_returns_none_on_peer_not_found() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/peers/nodex"))
        .respond_with(ResponseTemplate::new(404).set_body_json(json!({
            "success": false, "error": { "code": "PEER_NOT_FOUND", "message": "n/a" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    assert!(client.get_peer("nodex").await.unwrap().is_none());
}

#[tokio::test]
async fn top_domains_path() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/domains/top"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": []
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    client.top_domains().await.unwrap();
}

#[tokio::test]
async fn query_domain_validates_query_type() {
    let server = MockServer::start().await;
    let client = Client::new(&server.uri()).unwrap();
    let err = client.query_domain("d", "bogus", "x").await.unwrap_err();
    assert!(matches!(err, Error::Validation(_)));
}

#[tokio::test]
async fn audit_endpoints_work() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/audit/head"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "height": 0 }
        })))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/audit/entry/999"))
        .respond_with(ResponseTemplate::new(404).set_body_json(json!({
            "success": false, "error": { "code": "NOT_FOUND" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    client.audit_head().await.unwrap();
    assert!(client.audit_entry(999).await.unwrap().is_none());
}

#[tokio::test]
async fn moderation_action_post() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/moderation/actions"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "id": "tx" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let action = json!({
        "moderatorQuid": "mod",
        "targetType": "QUID",
        "targetId": "t",
        "scope": "hide",
        "reasonCode": "spam",
        "nonce": 1,
    });
    let r = client.create_moderation_action(&action).await.unwrap();
    assert_eq!(r.get("id").unwrap().as_str().unwrap(), "tx");
}

#[tokio::test]
async fn dsr_post_get_round_trip() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/privacy/dsr"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "id": "tx" }
        })))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/privacy/dsr/tx"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "request": {} }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    client
        .create_dsr(&json!({"subjectQuid": "s", "requestType": "ERASURE", "nonce": 1}))
        .await
        .unwrap();
    let _ = client.get_dsr_status("tx").await.unwrap().unwrap();
}

#[tokio::test]
async fn discovery_endpoints_hit_v2_prefix() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/v2/discovery/domain/foo"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": {}
        })))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/v2/discovery/trusted-quids"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": {}
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    client.discover_domain("foo").await.unwrap();
    client.discover_trusted_quids().await.unwrap();
}

#[tokio::test]
async fn dns_claim_and_resolve() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/v2/dns/claim"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "id": "tx" }
        })))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/v2/dns/resolve/example.com/A"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "records": [] }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let claim = json!({
        "domain": "example.com",
        "ownerQuid": "own",
        "rootQuid": "root",
        "nonce": 1,
    });
    client.submit_dns_claim(&claim).await.unwrap();
    client.resolve_dns_record("example.com", "A").await.unwrap();
}
