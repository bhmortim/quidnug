using System.Text;
using Xunit;

namespace Quidnug.Client.Tests;

public class QuidTests
{
    [Fact]
    public void GenerateHasExpectedIdFormat()
    {
        using var q = Quid.Generate();
        Assert.Equal(16, q.Id.Length);
        Assert.True(q.HasPrivateKey);
        Assert.NotNull(q.PublicKeyHex);
        Assert.NotNull(q.PrivateKeyHex);
    }

    [Fact]
    public void SignVerifyRoundtrip()
    {
        using var q = Quid.Generate();
        byte[] data = Encoding.UTF8.GetBytes("hello");
        string sig = q.Sign(data);
        Assert.True(q.Verify(data, sig));
        Assert.False(q.Verify(Encoding.UTF8.GetBytes("tampered"), sig));
    }

    [Fact]
    public void PrivateHexRoundtrip()
    {
        using var q = Quid.Generate();
        Assert.NotNull(q.PrivateKeyHex);
        using var r = Quid.FromPrivateHex(q.PrivateKeyHex!);
        Assert.Equal(q.Id, r.Id);
        Assert.Equal(q.PublicKeyHex, r.PublicKeyHex);
    }

    [Fact]
    public void ReadOnlyQuidCannotSign()
    {
        using var q = Quid.Generate();
        using var ro = Quid.FromPublicHex(q.PublicKeyHex);
        Assert.False(ro.HasPrivateKey);
        Assert.Equal(q.Id, ro.Id);
        Assert.Throws<InvalidOperationException>(
            () => ro.Sign(Encoding.UTF8.GetBytes("x")));
    }

    [Fact]
    public void CrossVerifyBetweenQuidsFails()
    {
        using var a = Quid.Generate();
        using var b = Quid.Generate();
        string sig = a.Sign(Encoding.UTF8.GetBytes("shared"));
        Assert.False(b.Verify(Encoding.UTF8.GetBytes("shared"), sig));
    }
}
