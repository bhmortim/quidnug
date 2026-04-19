using System.Security.Cryptography;

namespace Quidnug.Client;

/// <summary>
/// QDP-0010 compact Merkle inclusion-proof verifier.
///
/// <code>
///   leaf   = sha256(txBytes)
///   parent = sha256(sibling || self) if side == "left"
///            sha256(self || sibling) if side == "right"
///   root   = parent after walking every frame
/// </code>
/// </summary>
public static class Merkle
{
    /// <summary>One frame in an inclusion proof.</summary>
    public record MerkleProofFrame(string Hash, string Side);

    /// <summary>
    /// Verify a proof reconstructs the expected root.
    /// Returns true on success; false on a clean mismatch.
    /// Throws <see cref="QuidnugValidationException"/> for malformed input.
    /// </summary>
    public static bool VerifyInclusionProof(
        ReadOnlySpan<byte> txBytes,
        IEnumerable<MerkleProofFrame> frames,
        string expectedRootHex)
    {
        if (txBytes.Length == 0)
            throw new QuidnugCryptoException("txBytes is empty");

        byte[] root;
        try
        {
            root = Convert.FromHexString(expectedRootHex);
        }
        catch (FormatException)
        {
            throw new QuidnugValidationException("expectedRoot is not valid hex");
        }
        if (root.Length != 32)
            throw new QuidnugValidationException($"expectedRoot must be 32 bytes (got {root.Length})");

        byte[] current = SHA256.HashData(txBytes);
        int i = 0;
        foreach (var f in frames)
        {
            byte[] sib;
            try { sib = Convert.FromHexString(f.Hash); }
            catch (FormatException) { throw new QuidnugValidationException($"frame {i} hash is not valid hex"); }
            if (sib.Length != 32)
                throw new QuidnugValidationException($"frame {i} hash must be 32 bytes (got {sib.Length})");

            byte[] concat = new byte[64];
            switch (f.Side)
            {
                case "left":
                    Buffer.BlockCopy(sib, 0, concat, 0, 32);
                    Buffer.BlockCopy(current, 0, concat, 32, 32);
                    break;
                case "right":
                    Buffer.BlockCopy(current, 0, concat, 0, 32);
                    Buffer.BlockCopy(sib, 0, concat, 32, 32);
                    break;
                default:
                    throw new QuidnugValidationException(
                        $"frame {i} side must be 'left' or 'right', got '{f.Side}'");
            }
            current = SHA256.HashData(concat);
            i++;
        }

        return CryptographicOperations.FixedTimeEquals(current, root);
    }
}
