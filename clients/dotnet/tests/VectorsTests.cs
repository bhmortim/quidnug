using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Xunit;

namespace Quidnug.Client.Tests;

/// <summary>
/// .NET SDK consumer for the v1.0 cross-SDK test vectors at
/// <c>docs/test-vectors/v1.0/</c>.
///
/// Asserts the five conformance properties via the Quidnug.Client
/// public API (Quid.Sign, Quid.Verify, Quid.FromPublicHex,
/// Quid.FromPrivateScalarHex) against the checked-in reference
/// vectors.
///
/// NOTE: the local dev environment at commit time had a broken
/// .NET 9 runtime config, so these tests could not be run
/// locally. Verification in CI is pending. The test file compiles
/// in isolation (only System.Text.Json + Xunit + Quidnug.Client).
/// </summary>
public class VectorsTests
{
    private static string VectorsRoot()
    {
        // Test runs from clients/dotnet/tests/bin/Debug/net8.0/ typically.
        // Walk up until we find docs/test-vectors/v1.0.
        DirectoryInfo? dir = new DirectoryInfo(AppContext.BaseDirectory);
        while (dir != null)
        {
            var candidate = Path.Combine(dir.FullName, "docs", "test-vectors", "v1.0");
            if (Directory.Exists(candidate)) return candidate;
            dir = dir.Parent;
        }
        throw new InvalidOperationException(
            "docs/test-vectors/v1.0 not found walking up from " + AppContext.BaseDirectory);
    }

    private static Dictionary<string, JsonElement> LoadKeys()
    {
        var result = new Dictionary<string, JsonElement>();
        foreach (var path in Directory.GetFiles(
                Path.Combine(VectorsRoot(), "test-keys"), "*.json"))
        {
            var doc = JsonDocument.Parse(File.ReadAllText(path));
            result[doc.RootElement.GetProperty("name").GetString()!] = doc.RootElement.Clone();
        }
        return result;
    }

    private static JsonElement LoadVectorFile(string name)
    {
        var doc = JsonDocument.Parse(File.ReadAllText(
            Path.Combine(VectorsRoot(), name)));
        return doc.RootElement.Clone();
    }

    private static string BytesToHex(byte[] b)
    {
        var sb = new StringBuilder(b.Length * 2);
        foreach (var byt in b) sb.Append(byt.ToString("x2"));
        return sb.ToString();
    }

    private static byte[] HexToBytes(string hex)
    {
        var b = new byte[hex.Length / 2];
        for (int i = 0; i < b.Length; i++)
        {
            b[i] = Convert.ToByte(hex.Substring(i * 2, 2), 16);
        }
        return b;
    }

    /// <summary>
    /// Run the conformance properties for a single case.
    /// </summary>
    private static void RunCase(JsonElement caseNode, JsonElement key,
            byte[] signable, string derivedId)
    {
        var expected = caseNode.GetProperty("expected");
        var name = caseNode.GetProperty("name").GetString()!;

        // Property 1: SHA-256 matches.
        byte[] hash = SHA256.HashData(signable);
        Assert.Equal(expected.GetProperty("sha256_of_canonical_hex").GetString(),
                BytesToHex(hash));

        // Property 2: hex and utf8 forms match.
        Assert.Equal(expected.GetProperty("canonical_signable_bytes_hex").GetString(),
                BytesToHex(signable));
        Assert.Equal(expected.GetProperty("canonical_signable_bytes_utf8").GetString(),
                Encoding.UTF8.GetString(signable));

        // Property 3: ID derivation.
        Assert.Equal(expected.GetProperty("expected_id").GetString(), derivedId);

        // Property 4: reference signature verifies.
        using var qRO = Quid.FromPublicHex(
                key.GetProperty("public_key_sec1_hex").GetString()!);
        Assert.True(qRO.Verify(signable,
                expected.GetProperty("reference_signature_hex").GetString()!),
            $"{name}: SDK Verify rejected reference signature");
        Assert.Equal(64, expected.GetProperty("signature_length_bytes").GetInt32());

        // Property 5: tampered signature rejects.
        byte[] tampered = HexToBytes(expected.GetProperty("reference_signature_hex").GetString()!);
        tampered[5] ^= 0x01;
        Assert.False(qRO.Verify(signable, BytesToHex(tampered)),
            $"{name}: tampered signature accepted");

        // Property 6: SDK sign-verify round-trip.
        using var qSign = Quid.FromPrivateScalarHex(
                key.GetProperty("private_scalar_hex").GetString()!);
        string sdkSig = qSign.Sign(signable);
        Assert.Equal(128, sdkSig.Length);  // 64 bytes = 128 hex
        Assert.True(qSign.Verify(signable, sdkSig));
    }

    // --- Per-type signable-bytes builders (Go-struct-order JSON) ---
    //
    // System.Text.Json writes dictionary entries in insertion order
    // (unlike Newtonsoft); we exploit this by adding fields in the
    // order the Go struct declares them.

    private static byte[] TrustSignable(JsonElement inp)
    {
        using var ms = new MemoryStream();
        using (var writer = new Utf8JsonWriter(ms, new JsonWriterOptions { SkipValidation = false }))
        {
            writer.WriteStartObject();
            writer.WriteString("id", inp.GetProperty("id").GetString());
            writer.WriteString("type", "TRUST");
            writer.WriteString("trustDomain", inp.GetProperty("trustDomain").GetString());
            writer.WriteNumber("timestamp", inp.GetProperty("timestamp").GetInt64());
            writer.WriteString("signature", "");
            writer.WriteString("publicKey", inp.GetProperty("publicKey").GetString());
            writer.WriteString("truster", inp.GetProperty("truster").GetString());
            writer.WriteString("trustee", inp.GetProperty("trustee").GetString());
            WriteGoCompatFloat(writer, "trustLevel", inp.GetProperty("trustLevel").GetDouble());
            writer.WriteNumber("nonce", inp.GetProperty("nonce").GetInt64());
            if (TryGetNonEmptyString(inp, "description", out var desc))
                writer.WriteString("description", desc);
            if (TryGetNonZeroInt64(inp, "validUntil", out var vu))
                writer.WriteNumber("validUntil", vu);
            writer.WriteEndObject();
        }
        return ms.ToArray();
    }

    private static string TrustId(JsonElement inp)
    {
        using var ms = new MemoryStream();
        using (var writer = new Utf8JsonWriter(ms))
        {
            writer.WriteStartObject();
            writer.WriteString("Truster", inp.GetProperty("truster").GetString());
            writer.WriteString("Trustee", inp.GetProperty("trustee").GetString());
            WriteGoCompatFloat(writer, "TrustLevel", inp.GetProperty("trustLevel").GetDouble());
            writer.WriteString("TrustDomain", inp.GetProperty("trustDomain").GetString());
            writer.WriteNumber("Timestamp", inp.GetProperty("timestamp").GetInt64());
            writer.WriteEndObject();
        }
        return BytesToHex(SHA256.HashData(ms.ToArray()));
    }

    private static byte[] IdentitySignable(JsonElement inp)
    {
        using var ms = new MemoryStream();
        using (var writer = new Utf8JsonWriter(ms))
        {
            writer.WriteStartObject();
            writer.WriteString("id", inp.GetProperty("id").GetString());
            writer.WriteString("type", "IDENTITY");
            writer.WriteString("trustDomain", inp.GetProperty("trustDomain").GetString());
            writer.WriteNumber("timestamp", inp.GetProperty("timestamp").GetInt64());
            writer.WriteString("signature", "");
            writer.WriteString("publicKey", inp.GetProperty("publicKey").GetString());
            writer.WriteString("quidId", inp.GetProperty("quidId").GetString());
            writer.WriteString("name", inp.GetProperty("name").GetString());
            if (TryGetNonEmptyString(inp, "description", out var desc))
                writer.WriteString("description", desc);
            if (inp.TryGetProperty("attributes", out var attrs) && attrs.ValueKind == JsonValueKind.Object)
            {
                writer.WritePropertyName("attributes");
                attrs.WriteTo(writer);
            }
            writer.WriteString("creator", inp.GetProperty("creator").GetString());
            writer.WriteNumber("updateNonce", inp.GetProperty("updateNonce").GetInt64());
            if (TryGetNonEmptyString(inp, "homeDomain", out var hd))
                writer.WriteString("homeDomain", hd);
            writer.WriteEndObject();
        }
        return ms.ToArray();
    }

    private static string IdentityId(JsonElement inp)
    {
        using var ms = new MemoryStream();
        using (var writer = new Utf8JsonWriter(ms))
        {
            writer.WriteStartObject();
            writer.WriteString("QuidID", inp.GetProperty("quidId").GetString());
            writer.WriteString("Name", inp.GetProperty("name").GetString());
            writer.WriteString("Creator", inp.GetProperty("creator").GetString());
            writer.WriteString("TrustDomain", inp.GetProperty("trustDomain").GetString());
            writer.WriteNumber("UpdateNonce", inp.GetProperty("updateNonce").GetInt64());
            writer.WriteNumber("Timestamp", inp.GetProperty("timestamp").GetInt64());
            writer.WriteEndObject();
        }
        return BytesToHex(SHA256.HashData(ms.ToArray()));
    }

    private static void WriteGoCompatFloat(Utf8JsonWriter writer, string name, double value)
    {
        // Go's encoding/json elides ".0" on integer-valued floats.
        // Mirror that: whole-number floats in int64 range go as integers.
        if (double.IsFinite(value) && value == Math.Floor(value) && Math.Abs(value) < 1e15)
            writer.WriteNumber(name, (long)value);
        else
            writer.WriteNumber(name, value);
    }

    private static bool TryGetNonEmptyString(JsonElement node, string name, out string value)
    {
        if (node.TryGetProperty(name, out var p) && p.ValueKind == JsonValueKind.String)
        {
            value = p.GetString()!;
            return !string.IsNullOrEmpty(value);
        }
        value = "";
        return false;
    }

    private static bool TryGetNonZeroInt64(JsonElement node, string name, out long value)
    {
        if (node.TryGetProperty(name, out var p) && p.ValueKind == JsonValueKind.Number)
        {
            value = p.GetInt64();
            return value != 0;
        }
        value = 0;
        return false;
    }

    // --- Test cases ---

    [Fact]
    public void TrustVectors()
    {
        var vf = LoadVectorFile("trust-tx.json");
        Assert.Equal("TRUST", vf.GetProperty("tx_type").GetString());
        var keys = LoadKeys();
        foreach (var c in vf.GetProperty("cases").EnumerateArray())
        {
            var key = keys[c.GetProperty("signer_key_ref").GetString()!];
            var inp = c.GetProperty("input");
            byte[] signable = TrustSignable(inp);
            string id = TrustId(inp);
            RunCase(c, key, signable, id);
        }
    }

    [Fact]
    public void IdentityVectors()
    {
        var vf = LoadVectorFile("identity-tx.json");
        Assert.Equal("IDENTITY", vf.GetProperty("tx_type").GetString());
        var keys = LoadKeys();
        foreach (var c in vf.GetProperty("cases").EnumerateArray())
        {
            var key = keys[c.GetProperty("signer_key_ref").GetString()!];
            var inp = c.GetProperty("input");
            byte[] signable = IdentitySignable(inp);
            string id = IdentityId(inp);
            RunCase(c, key, signable, id);
        }
    }

    [Fact]
    public void SdkSignProducesIEEE1363()
    {
        using var q = Quid.Generate();
        byte[] data = Encoding.UTF8.GetBytes("test-data");
        string sigHex = q.Sign(data);
        Assert.Equal(128, sigHex.Length);
        Assert.True(q.Verify(data, sigHex));
    }
}
