"""Sanity tests for the wire-type dataclasses."""

from __future__ import annotations

from quidnug.types import (
    Anchor,
    GuardianRef,
    GuardianSet,
    OwnershipStake,
    Title,
)


def test_guardian_ref_effective_weight_defaults_to_one():
    g = GuardianRef(quid="a")
    assert g.effective_weight == 1
    assert g.weight == 1


def test_guardian_ref_effective_weight_floors_at_one():
    g = GuardianRef(quid="a", weight=0)
    assert g.effective_weight == 1
    g2 = GuardianRef(quid="a", weight=-3)
    assert g2.effective_weight == 1


def test_guardian_set_total_weight_sums_effective_weights():
    s = GuardianSet(
        subject_quid="s",
        guardians=[GuardianRef(quid="a", weight=2), GuardianRef(quid="b", weight=3)],
        threshold=3,
        recovery_delay_seconds=86400,
    )
    assert s.total_weight == 5


def test_title_construction():
    t = Title(
        asset_id="a",
        domain="d",
        title_type="HOME",
        owners=[OwnershipStake(owner_id="o1", percentage=100.0)],
    )
    assert t.owners[0].percentage == 100.0
    assert t.signatures == {}


def test_anchor_variants_compile():
    rot = Anchor(
        kind="rotation",
        signer_quid="q",
        anchor_nonce=1,
        valid_from=0,
        from_epoch=1,
        to_epoch=2,
        new_public_key="pk",
        min_next_nonce=5,
        max_accepted_old_nonce=4,
    )
    assert rot.kind == "rotation"
    inv = Anchor(kind="invalidation", signer_quid="q", anchor_nonce=1, valid_from=0, epoch_to_invalidate=2)
    assert inv.kind == "invalidation"
    cap = Anchor(kind="epoch-cap", signer_quid="q", anchor_nonce=1, valid_from=0, epoch=1, max_nonce=100)
    assert cap.kind == "epoch-cap"
