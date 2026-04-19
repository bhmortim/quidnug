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
}
