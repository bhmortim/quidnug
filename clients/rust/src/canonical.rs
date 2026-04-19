//! Canonical signable bytes — matches Go / Python byte-for-byte.
//!
//! Rule (from `schemas/types/canonicalization.md`):
//! 1. Marshal the object to JSON.
//! 2. Parse back into a generic [`serde_json::Value`].
//! 3. Marshal again with **alphabetized keys**.
//! 4. Exclude specific top-level fields (`signature`, `txId`, …).
//!
//! Step (3) matches Go's `encoding/json` behavior when marshaling a
//! `map[string]interface{}` — keys come out alphabetized. This gives
//! every SDK the same bytes-to-sign regardless of source-language
//! struct-field ordering.

use crate::error::{Error, Result};
use serde::Serialize;
use serde_json::{Map, Value};

/// Return the canonical signable bytes for a serializable value.
///
/// `exclude_fields` lists top-level keys to remove (typically
/// `"signature"` and `"txId"`).
pub fn canonical_bytes<T: Serialize>(value: &T, exclude_fields: &[&str]) -> Result<Vec<u8>> {
    let mut v = serde_json::to_value(value)?;
    match &mut v {
        Value::Object(m) => {
            for f in exclude_fields {
                m.remove(*f);
            }
        }
        _ => {
            return Err(Error::validation("canonical_bytes expects an object"));
        }
    }
    let sorted = sort_keys_deep(v);
    let out = serde_json::to_vec(&sorted)?;
    Ok(out)
}

fn sort_keys_deep(v: Value) -> Value {
    match v {
        Value::Object(m) => {
            let mut pairs: Vec<(String, Value)> = m.into_iter().collect();
            pairs.sort_by(|a, b| a.0.cmp(&b.0));
            let mut out = Map::new();
            for (k, val) in pairs {
                out.insert(k, sort_keys_deep(val));
            }
            Value::Object(out)
        }
        Value::Array(arr) => Value::Array(arr.into_iter().map(sort_keys_deep).collect()),
        other => other,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn stable_across_insertion_order() {
        let a = canonical_bytes(&json!({"b": 1, "a": 2}), &[]).unwrap();
        let b = canonical_bytes(&json!({"a": 2, "b": 1}), &[]).unwrap();
        assert_eq!(a, b);
    }

    #[test]
    fn excludes_fields() {
        let out = canonical_bytes(
            &json!({"type": "TRUST", "signature": "abc", "level": 0.9}),
            &["signature"],
        )
        .unwrap();
        let s = String::from_utf8(out).unwrap();
        assert!(!s.contains("signature"));
        assert!(s.contains("level"));
        assert!(s.contains("type"));
    }

    #[test]
    fn sorts_nested_keys() {
        let out = canonical_bytes(
            &json!({"nested": {"z": 1, "a": 2}, "outer": "x"}),
            &[],
        )
        .unwrap();
        assert_eq!(
            String::from_utf8(out).unwrap(),
            r#"{"nested":{"a":2,"z":1},"outer":"x"}"#
        );
    }
}
