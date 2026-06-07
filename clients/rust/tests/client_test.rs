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

// --- v2 method smoke tests ----------------------------------------------
//
// Exercise the new thin-wrapper methods covering QDPs 0002 / 0003 / 0005 /
// 0008 / 0009. Caller assembles the signed envelope; the SDK forwards it
// and translates the envelope+status code into typed results.

#[tokio::test]
async fn submit_guardian_set_update_posts_envelope() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/guardian/set-update"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "accepted": true }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let env = json!({
        "subjectQuid": "alice0000",
        "guardians": [{"quid": "bob000000", "weight": 1, "epoch": 0}],
        "threshold": 1,
        "recoveryDelaySeconds": 3600,
        "signature": "deadbeef",
    });
    let res = client.submit_guardian_set_update(&env).await.unwrap();
    assert_eq!(res.get("accepted").unwrap().as_bool().unwrap(), true);
}

#[tokio::test]
async fn get_guardian_set_returns_none_on_not_found() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/guardian/set/alice0000"))
        .respond_with(ResponseTemplate::new(404).set_body_json(json!({
            "success": false,
            "error": { "code": "NOT_FOUND", "message": "no set" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let res = client.get_guardian_set("alice0000").await.unwrap();
    assert!(res.is_none());
}

#[tokio::test]
async fn bootstrap_status_returns_envelope_data() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/bootstrap/status"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "phase": "complete", "kEffective": 3 }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let res = client.bootstrap_status().await.unwrap();
    assert_eq!(res.get("phase").unwrap().as_str().unwrap(), "complete");
}

#[tokio::test]
async fn fork_block_status_returns_envelope_data() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/fork-block/status"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true,
            "data": { "active": ["QDP-0005"], "pending": [] }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let res = client.fork_block_status().await.unwrap();
    assert!(res.get("active").unwrap().as_array().unwrap().len() == 1);
}

#[tokio::test]
async fn get_latest_domain_fingerprint_returns_none_on_not_found() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/domain-fingerprints/demo.home/latest"))
        .respond_with(ResponseTemplate::new(404).set_body_json(json!({
            "success": false,
            "error": { "code": "NOT_FOUND", "message": "no fingerprint" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let res = client
        .get_latest_domain_fingerprint("demo.home")
        .await
        .unwrap();
    assert!(res.is_none());
}

#[tokio::test]
async fn submit_recovery_init_posts_envelope() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/guardian/recovery/init"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "txId": "init-tx" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let env = json!({
        "subjectQuid": "alice0000",
        "newPublicKey": "abcd",
        "guardianSignatures": [],
    });
    let res = client.submit_recovery_init(&env).await.unwrap();
    assert_eq!(res.get("txId").unwrap().as_str().unwrap(), "init-tx");
}

#[tokio::test]
async fn query_relational_trust_decodes_typed_result() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/trust/query"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true,
            "data": {
                "observer": "alice0000",
                "target": "bob000000",
                "trustLevel": 0.72,
                "trustPath": ["alice0000", "carol0000", "bob000000"],
                "pathDepth": 2,
                "domain": "demo.home",
            }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let q = json!({"observer": "alice0000", "target": "bob000000", "domain": "demo.home"});
    let res = client.query_relational_trust(&q).await.unwrap();
    assert!((res.trust_level - 0.72).abs() < 1e-9);
    assert_eq!(res.path.len(), 3);
}
