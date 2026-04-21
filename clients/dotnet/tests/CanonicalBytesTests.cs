using System.Text;
using System.Text.Json.Nodes;
using Xunit;

namespace Quidnug.Client.Tests;

public class CanonicalBytesTests
{
    [Fact]
    public void StableAcrossInsertionOrder()
    {
        var a = new Dictionary<string, object?> { ["b"] = 1, ["a"] = 2 };
        var b = new Dictionary<string, object?> { ["a"] = 2, ["b"] = 1 };
        Assert.Equal(CanonicalBytes.Of(a), CanonicalBytes.Of(b));
    }

    [Fact]
    public void ExcludesNamedFields()
    {
        var tx = new Dictionary<string, object?>
        {
            ["type"] = "TRUST",
            ["signature"] = "abc",
            ["level"] = 0.9,
        };
        string s = Encoding.UTF8.GetString(CanonicalBytes.Of(tx, "signature"));
        Assert.DoesNotContain("signature", s);
        Assert.Contains("level", s);
        Assert.Contains("TRUST", s);
    }

    [Fact]
    public void SortsNestedKeysRecursively()
    {
        var outer = new Dictionary<string, object?>
        {
            ["outer"] = "x",
            ["nested"] = new Dictionary<string, object?>
            {
                ["z"] = 1,
                ["a"] = 2,
            },
        };
        string s = Encoding.UTF8.GetString(CanonicalBytes.Of(outer));
        Assert.Equal("{\"nested\":{\"a\":2,\"z\":1},\"outer\":\"x\"}", s);
    }

    /// <summary>
    /// Interop lock: the .NET output for this transaction MUST equal
    /// the reference string that Python / Go / Rust also produce.
    /// If this diverges, a .NET-signed tx will not verify on a
    /// Go-reference node.
    /// </summary>
    [Fact]
    public void Utf8InteropLock()
    {
        var tx = new Dictionary<string, object?>
        {
            ["message"] = "hello 世界 🌍",
            ["a"] = 1,
        };
        string actual = Encoding.UTF8.GetString(CanonicalBytes.Of(tx));
        string expected = "{\"a\":1,\"message\":\"hello 世界 🌍\"}";
        Assert.Equal(expected, actual);
    }

    /// <summary>
    /// V1Of preserves the caller-supplied top-level key order
    /// (Go struct declaration order for the tx type) while
    /// still sorting nested-object keys alphabetically to match
    /// Go's encoding/json default for map[string]interface{}.
    /// This is the v1.0-conformant canonical form.
    /// </summary>
    [Fact]
    public void V1OfPreservesTopLevelOrderSortsNested()
    {
        // Struct-declaration-order insertion (mirrors an EVENT
        // tx: id, type, trustDomain, timestamp, payload).
        var nestedInner = new JsonObject();
        nestedInner["yankee"] = "A";
        nestedInner["alpha"] = "B";
        nestedInner["mike"] = "C";

        var payload = new JsonObject();
        payload["zebra"] = 1;
        payload["apple"] = 2;
        payload["mango"] = 3;
        payload["nested"] = nestedInner;

        var tx = new JsonObject();
        tx["id"] = "abc";
        tx["type"] = "EVENT";
        tx["trustDomain"] = "test";
        tx["timestamp"] = 1729468800L;
        tx["payload"] = payload;

        string s = Encoding.UTF8.GetString(CanonicalBytes.V1Of(tx));

        // Top level in caller-specified order (id, type,
        // trustDomain, timestamp, payload — NOT alphabetical).
        // Nested keys within payload are sorted alphabetically.
        string expected =
            "{\"id\":\"abc\"," +
            "\"type\":\"EVENT\"," +
            "\"trustDomain\":\"test\"," +
            "\"timestamp\":1729468800," +
            "\"payload\":{" +
              "\"apple\":2," +
              "\"mango\":3," +
              "\"nested\":{\"alpha\":\"B\",\"mike\":\"C\",\"yankee\":\"A\"}," +
              "\"zebra\":1" +
            "}}";
        Assert.Equal(expected, s);
    }

    /// <summary>
    /// Regression guard: the legacy Of() sorts every level.
    /// Locks in that behavior so the two helpers stay visibly
    /// distinct; v1.0 users must call V1Of, not Of.
    /// </summary>
    [Fact]
#pragma warning disable CS0618 // legacy Of is intentionally tested
    public void LegacyOfSortsTopLevelToo()
    {
        var tx = new Dictionary<string, object?>
        {
            ["type"] = "EVENT",
            ["id"] = "abc",
        };
        string s = Encoding.UTF8.GetString(CanonicalBytes.Of(tx));
        // Alphabetical: id before type.
        Assert.Equal("{\"id\":\"abc\",\"type\":\"EVENT\"}", s);
    }
#pragma warning restore CS0618
}
