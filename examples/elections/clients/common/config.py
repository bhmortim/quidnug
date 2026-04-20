"""
YAML config loader. Used by every client to find its node URL,
key paths, election identifier.
"""
from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import Optional

import yaml


@dataclass
class ClientConfig:
    node_url: str = "http://localhost:8087"
    election_id: str = "example-election.2026-nov"
    authority_operator_quid: Optional[str] = None
    authority_rsa_pubkey_path: Optional[str] = None
    key_path: Optional[str] = None
    extra: dict = field(default_factory=dict)


def load_config(path: Optional[str] = None) -> ClientConfig:
    """Load config from a YAML file. If path is None, try in
    order: $QUIDNUG_ELECTIONS_CONFIG, ~/.quidnug/elections.yaml,
    ./elections-config.yaml. Returns defaults if none found."""
    if path is None:
        candidates = [
            os.environ.get("QUIDNUG_ELECTIONS_CONFIG"),
            os.path.expanduser("~/.quidnug/elections.yaml"),
            "./elections-config.yaml",
        ]
        for c in candidates:
            if c and os.path.exists(c):
                path = c
                break

    if path is None or not os.path.exists(path):
        return ClientConfig()

    with open(path, "r") as f:
        data = yaml.safe_load(f) or {}

    cfg = ClientConfig(
        node_url=data.get("node_url", "http://localhost:8087"),
        election_id=data.get("election_id", "example-election.2026-nov"),
        authority_operator_quid=data.get("authority_operator_quid"),
        authority_rsa_pubkey_path=data.get("authority_rsa_pubkey_path"),
        key_path=data.get("key_path"),
    )
    cfg.extra = {k: v for k, v in data.items() if k not in {
        "node_url", "election_id", "authority_operator_quid",
        "authority_rsa_pubkey_path", "key_path",
    }}
    return cfg
