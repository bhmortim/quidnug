"""
Cryptographic primitives for the election reference clients.

Two scheme families:
  1. ECDSA P-256 + SHA-256 — Quidnug's native signing, used for
     VRQs, BQs, vote edges, events, governor signatures.
  2. RSA-FDH-3072 — blind-signature scheme for ballot issuance
     per QDP-0021. RFC 9474 profile.

Everything here uses Python's cryptography library (pyca). No
bespoke cryptography. Test vectors for RSA-FDH blind signing are
in tests/test_crypto.py and match RFC 9474 section 4.
"""
from __future__ import annotations

import hashlib
import os
import secrets
from dataclasses import dataclass
from typing import Tuple

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, padding, rsa
from cryptography.hazmat.primitives.asymmetric.utils import (
    decode_dss_signature, encode_dss_signature,
)
from cryptography.exceptions import InvalidSignature


# ---------------------------------------------------------------
# ECDSA P-256 — for quid signing
# ---------------------------------------------------------------

@dataclass
class ECDSAKeypair:
    """A holder for the private + public halves of an ECDSA P-256
    keypair. The quid ID is sha256(publicKey.uncompressed_point)[:16]."""
    private_key: ec.EllipticCurvePrivateKey
    public_key_bytes: bytes      # SEC1 uncompressed, 65 bytes

    @property
    def quid_id(self) -> str:
        """16-hex quid ID, per the standard derivation."""
        return hashlib.sha256(self.public_key_bytes).hexdigest()[:16]

    @property
    def public_key_hex(self) -> str:
        return self.public_key_bytes.hex()


def generate_ecdsa_keypair() -> ECDSAKeypair:
    """Generate a fresh ECDSA P-256 keypair."""
    private = ec.generate_private_key(ec.SECP256R1())
    pub_bytes = private.public_key().public_bytes(
        encoding=serialization.Encoding.X962,
        format=serialization.PublicFormat.UncompressedPoint,
    )
    return ECDSAKeypair(private_key=private, public_key_bytes=pub_bytes)


def ecdsa_sign_ieee1363(private: ec.EllipticCurvePrivateKey, data: bytes) -> bytes:
    """Sign `data` with ECDSA P-256 + SHA-256 and return the
    64-byte IEEE-1363 (r||s) encoding the Quidnug node accepts.

    Python's `cryptography` outputs DER by default; we convert."""
    der_sig = private.sign(data, ec.ECDSA(hashes.SHA256()))
    r, s = decode_dss_signature(der_sig)
    return r.to_bytes(32, "big") + s.to_bytes(32, "big")


def ecdsa_verify_ieee1363(public_key_hex: str, data: bytes, sig_bytes: bytes) -> bool:
    """Verify an IEEE-1363 r||s signature against a SEC1-uncompressed
    pubkey. Returns True if valid."""
    if len(sig_bytes) != 64:
        return False
    pub_bytes = bytes.fromhex(public_key_hex)
    pubkey = ec.EllipticCurvePublicKey.from_encoded_point(ec.SECP256R1(), pub_bytes)
    r = int.from_bytes(sig_bytes[:32], "big")
    s = int.from_bytes(sig_bytes[32:], "big")
    der_sig = encode_dss_signature(r, s)
    try:
        pubkey.verify(der_sig, data, ec.ECDSA(hashes.SHA256()))
        return True
    except InvalidSignature:
        return False


def save_ecdsa_keypair(keypair: ECDSAKeypair, path: str) -> None:
    """Persist the private key (PKCS8 PEM). For demo only; in
    production the voter's VRQ private key would live in a secure
    enclave or hardware wallet, not in a file."""
    pem = keypair.private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )
    with open(path, "wb") as f:
        f.write(pem)
    os.chmod(path, 0o600)


def load_ecdsa_keypair(path: str) -> ECDSAKeypair:
    with open(path, "rb") as f:
        private = serialization.load_pem_private_key(f.read(), password=None)
    assert isinstance(private, ec.EllipticCurvePrivateKey)
    pub_bytes = private.public_key().public_bytes(
        encoding=serialization.Encoding.X962,
        format=serialization.PublicFormat.UncompressedPoint,
    )
    return ECDSAKeypair(private_key=private, public_key_bytes=pub_bytes)


# ---------------------------------------------------------------
# RSA-FDH blind signatures (QDP-0021) — for ballot issuance
# ---------------------------------------------------------------

@dataclass
class RSABlindKeypair:
    """RSA-3072 keypair for the authority's blind-issuance key.
    Public modulus n, public exponent e (standard 65537), private
    exponent d. Held by the election authority only; never
    transmitted."""
    private_key: rsa.RSAPrivateKey
    public_key: rsa.RSAPublicKey

    @property
    def public_modulus(self) -> int:
        return self.public_key.public_numbers().n

    @property
    def public_exponent(self) -> int:
        return self.public_key.public_numbers().e

    @property
    def public_key_pem(self) -> bytes:
        return self.public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )

    @property
    def fingerprint(self) -> str:
        der = self.public_key.public_bytes(
            encoding=serialization.Encoding.DER,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        return hashlib.sha256(der).hexdigest()


def generate_rsa_blind_keypair(bits: int = 3072) -> RSABlindKeypair:
    """Generate a fresh RSA-3072 (or configurable) keypair for
    blind issuance."""
    private = rsa.generate_private_key(public_exponent=65537, key_size=bits)
    return RSABlindKeypair(private_key=private, public_key=private.public_key())


def ballot_token(election_id: str, bq_pubkey_hex: str, nonce: bytes) -> bytes:
    """Construct a ballot token from the election ID, the voter's
    ephemeral BQ pubkey, and a random nonce.

    Used by the voter's device to build the opaque token that the
    authority blind-signs. The resulting signature proves the voter
    was authorized to cast a ballot without revealing who.
    """
    h = hashlib.sha256()
    h.update(election_id.encode("utf-8"))
    h.update(b"\x00")
    h.update(bq_pubkey_hex.encode("utf-8"))
    h.update(b"\x00")
    h.update(nonce)
    return h.digest()


def _mgf1_sha256(seed: bytes, length: int) -> bytes:
    """RFC 8017 MGF1 with SHA-256. Used for full-domain hashing
    in RSA-FDH per RFC 9474."""
    counter = 0
    output = b""
    while len(output) < length:
        output += hashlib.sha256(seed + counter.to_bytes(4, "big")).digest()
        counter += 1
    return output[:length]


def rsa_fdh_encode(token: bytes, modulus: int) -> int:
    """Encode the ballot token to a full-domain-hashed integer
    modulo n, ready for RSA signing/blinding. Per RFC 9474 §4.1
    (RSASSA-PSS-FDH variant simplified for the blind case)."""
    n_byte_len = (modulus.bit_length() + 7) // 8
    digest = _mgf1_sha256(token, n_byte_len)
    # Mask off top two bits so value < modulus deterministically.
    digest = bytes([digest[0] & 0x3F]) + digest[1:]
    x = int.from_bytes(digest, "big")
    if x >= modulus:
        x = x % modulus
    return x


def blind(token: bytes, public: rsa.RSAPublicKey) -> Tuple[int, int]:
    """Voter-side: blind a ballot token.

    Returns (blinded_value, blinding_factor_r). The voter must
    keep r secret — it's how they unblind the authority's
    signature. The authority receives only blinded_value."""
    n = public.public_numbers().n
    e = public.public_numbers().e

    # Full-domain-hash the token to an integer mod n.
    t = rsa_fdh_encode(token, n)

    # Generate a random blinding factor r, coprime to n.
    while True:
        r = secrets.randbelow(n - 2) + 2
        # Extended-Euclid to verify gcd(r, n) == 1. For real RSA
        # with prime factors, this is almost always true, but
        # pin it for correctness:
        from math import gcd
        if gcd(r, n) == 1:
            break

    # Compute blinded = (t * r^e) mod n.
    blinded = (t * pow(r, e, n)) % n
    return blinded, r


def sign_blinded(blinded_value: int, private: rsa.RSAPrivateKey) -> int:
    """Authority-side: RSA-sign the blinded value. This is a raw
    RSA private-key operation (blinded^d mod n). The authority
    learns nothing about the underlying ballot token from this
    operation."""
    numbers = private.private_numbers()
    n = numbers.public_numbers.n
    d = numbers.d
    return pow(blinded_value, d, n)


def unblind(signed_blinded: int, r: int, public: rsa.RSAPublicKey) -> bytes:
    """Voter-side: unblind the authority's signature.

    Input: signed_blinded = blinded^d mod n (from authority)
           r             = blinding factor (voter secret)
    Output: signature S such that S^e mod n == fdh_encode(token)

    The returned bytes are the classical RSA-FDH signature that
    anyone can verify with the authority's public key."""
    n = public.public_numbers().n
    # Compute r^(-1) mod n via extended Euclid.
    r_inv = pow(r, -1, n)
    S = (signed_blinded * r_inv) % n
    # Return as big-endian bytes at the modulus's byte length.
    n_byte_len = (n.bit_length() + 7) // 8
    return S.to_bytes(n_byte_len, "big")


def verify_blind_signature(
    signature_bytes: bytes, token: bytes, public: rsa.RSAPublicKey
) -> bool:
    """Publicly-verifiable: given a ballot token + authority's
    RSA public key + a signature produced via the blind flow,
    check that the signature is valid.

    Anyone can run this at tally / audit time without knowing
    which voter the signature was issued to."""
    n = public.public_numbers().n
    e = public.public_numbers().e

    S = int.from_bytes(signature_bytes, "big")
    if S >= n or S <= 0:
        return False

    expected = rsa_fdh_encode(token, n)
    computed = pow(S, e, n)
    return computed == expected


# ---------------------------------------------------------------
# Convenience: load/save RSA keys + fingerprint equality
# ---------------------------------------------------------------

def save_rsa_keypair(keypair: RSABlindKeypair, priv_path: str, pub_path: str) -> None:
    priv_pem = keypair.private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )
    with open(priv_path, "wb") as f:
        f.write(priv_pem)
    os.chmod(priv_path, 0o600)
    with open(pub_path, "wb") as f:
        f.write(keypair.public_key_pem)


def load_rsa_keypair(priv_path: str) -> RSABlindKeypair:
    with open(priv_path, "rb") as f:
        private = serialization.load_pem_private_key(f.read(), password=None)
    assert isinstance(private, rsa.RSAPrivateKey)
    return RSABlindKeypair(private_key=private, public_key=private.public_key())


def load_rsa_public_key(path: str) -> rsa.RSAPublicKey:
    with open(path, "rb") as f:
        pub = serialization.load_pem_public_key(f.read())
    assert isinstance(pub, rsa.RSAPublicKey)
    return pub


def rsa_fingerprint(public: rsa.RSAPublicKey) -> str:
    der = public.public_bytes(
        encoding=serialization.Encoding.DER,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )
    return hashlib.sha256(der).hexdigest()
