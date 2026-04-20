using System.Security.Cryptography;

namespace Quidnug.Client;

/// <summary>
/// A cryptographic identity (ECDSA P-256). The quid ID is
/// <c>sha256(publicKey)[0..8]</c> in hex — 16 hex chars — matching
/// every other Quidnug SDK byte-for-byte.
/// </summary>
public sealed class Quid : IDisposable
{
    private readonly ECDsa _key;
    private readonly bool _hasPrivate;

    public string Id { get; }
    public string PublicKeyHex { get; }
    public string? PrivateKeyHex { get; }
    public bool HasPrivateKey => _hasPrivate;

    private Quid(ECDsa key, bool hasPrivate, string id, string publicHex, string? privateHex)
    {
        _key = key;
        _hasPrivate = hasPrivate;
        Id = id;
        PublicKeyHex = publicHex;
        PrivateKeyHex = privateHex;
    }

    /// <summary>Generate a fresh P-256 keypair.</summary>
    public static Quid Generate()
    {
        var ecdsa = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var parameters = ecdsa.ExportParameters(true);
        byte[] pub = ExportSec1Uncompressed(parameters);
        byte[] priv = ecdsa.ExportPkcs8PrivateKey();
        string id = QuidIdOf(pub);
        return new Quid(ecdsa, true, id, ToHex(pub), ToHex(priv));
    }

    /// <summary>Rebuild a Quid from a PKCS8 DER hex-encoded private key.</summary>
    public static Quid FromPrivateHex(string privHex)
    {
        var ecdsa = ECDsa.Create();
        ecdsa.ImportPkcs8PrivateKey(FromHex(privHex), out _);
        var parameters = ecdsa.ExportParameters(false);
        byte[] pub = ExportSec1Uncompressed(parameters);
        string id = QuidIdOf(pub);
        return new Quid(ecdsa, true, id, ToHex(pub), privHex);
    }

    /// <summary>Build a read-only Quid from a SEC1 uncompressed hex public key.</summary>
    public static Quid FromPublicHex(string pubHex)
    {
        byte[] pub = FromHex(pubHex);
        if (pub.Length != 65 || pub[0] != 0x04)
            throw new ArgumentException("expected 65-byte SEC1 uncompressed P-256 point");

        var parameters = new ECParameters
        {
            Curve = ECCurve.NamedCurves.nistP256,
            Q = new ECPoint
            {
                X = pub[1..33],
                Y = pub[33..65],
            },
        };
        var ecdsa = ECDsa.Create(parameters);
        string id = QuidIdOf(pub);
        return new Quid(ecdsa, false, id, pubHex, null);
    }

    /// <summary>
    /// Sign data.
    ///
    /// v1.0 canonical form: returns hex-encoded 64-byte IEEE-1363
    /// raw signature (<c>r||s</c>, each zero-padded to 32 bytes).
    /// Matches the reference node's <c>VerifySignature</c>.
    ///
    /// .NET's <c>ECDsa.SignHash</c> produces IEEE-1363 natively,
    /// so no encoding conversion is required.
    /// </summary>
    public string Sign(ReadOnlySpan<byte> data)
    {
        if (!_hasPrivate) throw new InvalidOperationException("quid is read-only");
        byte[] digest = SHA256.HashData(data);
        byte[] ieee = _key.SignHash(digest);
        return ToHex(ieee);
    }

    /// <summary>
    /// Verify a hex-encoded IEEE-1363 raw 64-byte signature.
    ///
    /// v1.0 canonical form: expects exactly 64 bytes.
    /// Anything else is rejected.
    /// </summary>
    public bool Verify(ReadOnlySpan<byte> data, string sigHex)
    {
        try
        {
            byte[] ieee = FromHex(sigHex);
            if (ieee.Length != 64) return false;
            byte[] digest = SHA256.HashData(data);
            return _key.VerifyHash(digest, ieee);
        }
        catch
        {
            return false;
        }
    }

    /// <summary>
    /// Reconstruct a Quid from a raw private scalar in hex.
    ///
    /// Used primarily by v1.0 test vectors which check in
    /// deterministic keys as raw scalars rather than PKCS8 DER.
    /// </summary>
    public static Quid FromPrivateScalarHex(string scalarHex)
    {
        byte[] d = FromHex(scalarHex);
        if (d.Length > 32) throw new ArgumentException("scalar must be <= 32 bytes");
        byte[] padded = new byte[32];
        Buffer.BlockCopy(d, 0, padded, 32 - d.Length, d.Length);
        var parameters = new ECParameters
        {
            Curve = ECCurve.NamedCurves.nistP256,
            D = padded,
        };
        var ecdsa = ECDsa.Create(parameters);
        var p = ecdsa.ExportParameters(includePrivateParameters: false);
        byte[] sec1 = ExportSec1Uncompressed(p);
        string id = QuidIdOf(sec1);
        string privHex = ToHex(padded);
        // Re-import with both D + Q so the resulting key has
        // full Q populated for signing + verification.
        ecdsa.Dispose();
        parameters = new ECParameters
        {
            Curve = ECCurve.NamedCurves.nistP256,
            D = padded,
            Q = p.Q,
        };
        var ecdsa2 = ECDsa.Create(parameters);
        return new Quid(ecdsa2, true, id, ToHex(sec1), privHex);
    }

    public void Dispose() => _key.Dispose();

    // --- Helpers --------------------------------------------------------

    private static byte[] ExportSec1Uncompressed(ECParameters parameters)
    {
        byte[] x = FixedLen(parameters.Q.X!, 32);
        byte[] y = FixedLen(parameters.Q.Y!, 32);
        byte[] sec1 = new byte[65];
        sec1[0] = 0x04;
        Buffer.BlockCopy(x, 0, sec1, 1, 32);
        Buffer.BlockCopy(y, 0, sec1, 33, 32);
        return sec1;
    }

    private static byte[] FixedLen(byte[] src, int len)
    {
        if (src.Length == len) return src;
        byte[] dst = new byte[len];
        if (src.Length < len)
        {
            Buffer.BlockCopy(src, 0, dst, len - src.Length, src.Length);
        }
        else
        {
            // leading sign byte
            Buffer.BlockCopy(src, src.Length - len, dst, 0, len);
        }
        return dst;
    }

    private static string QuidIdOf(byte[] publicKey)
    {
        byte[] digest = SHA256.HashData(publicKey);
        return ToHex(digest.AsSpan()[..8]);
    }

    private static string ToHex(ReadOnlySpan<byte> b)
    {
        const string hex = "0123456789abcdef";
        char[] out_ = new char[b.Length * 2];
        for (int i = 0; i < b.Length; i++)
        {
            out_[i * 2]     = hex[(b[i] >> 4) & 0x0f];
            out_[i * 2 + 1] = hex[ b[i]       & 0x0f];
        }
        return new string(out_);
    }

    private static string ToHex(byte[] b) => ToHex(b.AsSpan());

    private static byte[] FromHex(string s)
    {
        if ((s.Length & 1) != 0) throw new ArgumentException("odd hex length");
        byte[] b = new byte[s.Length / 2];
        for (int i = 0; i < b.Length; i++)
        {
            b[i] = (byte)((Hex(s[i * 2]) << 4) | Hex(s[i * 2 + 1]));
        }
        return b;

        static int Hex(char c) => c switch
        {
            >= '0' and <= '9' => c - '0',
            >= 'a' and <= 'f' => c - 'a' + 10,
            >= 'A' and <= 'F' => c - 'A' + 10,
            _ => throw new ArgumentException("bad hex char"),
        };
    }

    private static byte[] Ieee1363ToDer(byte[] ieee)
    {
        int half = ieee.Length / 2;
        byte[] r = MinimallyEncode(ieee.AsSpan(0, half).ToArray());
        byte[] s = MinimallyEncode(ieee.AsSpan(half, half).ToArray());
        // SEQUENCE { INTEGER r, INTEGER s }
        int len = 2 + r.Length + 2 + s.Length;
        byte[] der = new byte[2 + len];
        int idx = 0;
        der[idx++] = 0x30; der[idx++] = (byte)len;
        der[idx++] = 0x02; der[idx++] = (byte)r.Length;
        Buffer.BlockCopy(r, 0, der, idx, r.Length); idx += r.Length;
        der[idx++] = 0x02; der[idx++] = (byte)s.Length;
        Buffer.BlockCopy(s, 0, der, idx, s.Length);
        return der;
    }

    private static byte[] DerToIeee1363(byte[] der)
    {
        int idx = 0;
        if (der[idx++] != 0x30) throw new FormatException("not SEQUENCE");
        idx++; // len
        if (der[idx++] != 0x02) throw new FormatException("not INTEGER (r)");
        int rLen = der[idx++];
        byte[] r = der[idx..(idx + rLen)]; idx += rLen;
        if (der[idx++] != 0x02) throw new FormatException("not INTEGER (s)");
        int sLen = der[idx++];
        byte[] s = der[idx..(idx + sLen)];

        byte[] rFixed = FixedLen(StripLeadingZero(r), 32);
        byte[] sFixed = FixedLen(StripLeadingZero(s), 32);
        byte[] ieee = new byte[64];
        Buffer.BlockCopy(rFixed, 0, ieee, 0, 32);
        Buffer.BlockCopy(sFixed, 0, ieee, 32, 32);
        return ieee;
    }

    private static byte[] MinimallyEncode(byte[] b)
    {
        // Strip leading zeros, but keep one if high bit would be set.
        int i = 0;
        while (i < b.Length - 1 && b[i] == 0) i++;
        byte[] trimmed = b[i..];
        if ((trimmed[0] & 0x80) != 0)
        {
            byte[] pad = new byte[trimmed.Length + 1];
            Buffer.BlockCopy(trimmed, 0, pad, 1, trimmed.Length);
            return pad;
        }
        return trimmed;
    }

    private static byte[] StripLeadingZero(byte[] b)
    {
        if (b.Length > 1 && b[0] == 0) return b[1..];
        return b;
    }
}
