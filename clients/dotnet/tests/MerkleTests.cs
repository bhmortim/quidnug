using System.Security.Cryptography;
using System.Text;
using Xunit;

namespace Quidnug.Client.Tests;

public class MerkleTests
{
    private static byte[] Sha(byte[] data) => SHA256.HashData(data);

    [Fact]
    public void SingleSiblingRight()
    {
        byte[] tx = Encoding.UTF8.GetBytes("tx-1");
        byte[] sib = Sha(Encoding.UTF8.GetBytes("tx-2"));
        byte[] leaf = Sha(tx);
        byte[] concat = new byte[64];
        Buffer.BlockCopy(leaf, 0, concat, 0, 32);
        Buffer.BlockCopy(sib, 0, concat, 32, 32);
        byte[] root = Sha(concat);

        var frames = new[] { new Merkle.MerkleProofFrame(Convert.ToHexString(sib).ToLowerInvariant(), "right") };
        Assert.True(Merkle.VerifyInclusionProof(tx, frames, Convert.ToHexString(root).ToLowerInvariant()));
    }

    [Fact]
    public void FourLeafTree()
    {
        byte[][] leaves = new byte[4][];
        for (int i = 0; i < 4; i++) leaves[i] = Sha(Encoding.UTF8.GetBytes($"tx-{i}"));

        byte[] p0 = Sha(Concat(leaves[0], leaves[1]));
        byte[] p1 = Sha(Concat(leaves[2], leaves[3]));
        byte[] root = Sha(Concat(p0, p1));

        var frames = new[]
        {
            new Merkle.MerkleProofFrame(Convert.ToHexString(leaves[3]).ToLowerInvariant(), "right"),
            new Merkle.MerkleProofFrame(Convert.ToHexString(p0).ToLowerInvariant(), "left"),
        };
        Assert.True(Merkle.VerifyInclusionProof(
            Encoding.UTF8.GetBytes("tx-2"), frames, Convert.ToHexString(root).ToLowerInvariant()));
    }

    [Fact]
    public void TamperedTxRejected()
    {
        byte[] sib = Sha(Encoding.UTF8.GetBytes("tx-2"));
        byte[] leaf = Sha(Encoding.UTF8.GetBytes("tx-1"));
        byte[] root = Sha(Concat(leaf, sib));
        var frames = new[] { new Merkle.MerkleProofFrame(Convert.ToHexString(sib).ToLowerInvariant(), "right") };
        Assert.False(Merkle.VerifyInclusionProof(
            Encoding.UTF8.GetBytes("tampered"), frames, Convert.ToHexString(root).ToLowerInvariant()));
    }

    [Fact]
    public void MalformedFrameThrows()
    {
        var frames = new[] { new Merkle.MerkleProofFrame("nothex", "right") };
        Assert.Throws<QuidnugValidationException>(
            () => Merkle.VerifyInclusionProof(
                Encoding.UTF8.GetBytes("x"), frames, new string('a', 64)));
    }

    [Fact]
    public void EmptyTxThrows()
    {
        Assert.Throws<QuidnugCryptoException>(
            () => Merkle.VerifyInclusionProof(
                ReadOnlySpan<byte>.Empty, Array.Empty<Merkle.MerkleProofFrame>(), new string('a', 64)));
    }

    private static byte[] Concat(byte[] a, byte[] b)
    {
        byte[] c = new byte[a.Length + b.Length];
        Buffer.BlockCopy(a, 0, c, 0, a.Length);
        Buffer.BlockCopy(b, 0, c, a.Length, b.Length);
        return c;
    }
}
