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
}
