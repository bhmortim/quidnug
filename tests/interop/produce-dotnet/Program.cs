// .NET cross-SDK interop vector producer.
//
//   dotnet run --project tests/interop/produce-dotnet -- \
//       > tests/interop/vectors-dotnet.json

using System.Text.Json;
using System.Text.Json.Nodes;
using Quidnug.Client;

const string keypairFile = "tests/interop/.keypair-dotnet";

Quid signer;
if (File.Exists(keypairFile))
{
    signer = Quid.FromPrivateHex(File.ReadAllText(keypairFile).Trim());
}
else
{
    signer = Quid.Generate();
    File.WriteAllText(keypairFile, signer.PrivateKeyHex);
}

object MakeCase(string name, object tx, string[] exclude)
{
    byte[] cb = CanonicalBytes.Of(tx, exclude);
    string sig = signer.Sign(cb);
    return new
    {
        name,
        tx,
        excludeFields = exclude,
        canonicalBytesHex = Convert.ToHexString(cb).ToLowerInvariant(),
        signatureHex = sig,
    };
}

var cases = new object[]
{
    MakeCase("trust-basic", new Dictionary<string, object?>
    {
        ["type"] = "TRUST",
        ["timestamp"] = 1_700_000_000L,
        ["trustDomain"] = "interop.test",
        ["signerQuid"] = signer.Id,
        ["truster"] = signer.Id,
        ["trustee"] = "abc0123456789def",
        ["trustLevel"] = 0.9,
        ["nonce"] = 1L,
    }, new[] { "signature", "txId" }),

    MakeCase("identity-with-attrs", new Dictionary<string, object?>
    {
        ["type"] = "IDENTITY",
        ["timestamp"] = 1_700_000_000L,
        ["trustDomain"] = "interop.test",
        ["signerQuid"] = signer.Id,
        ["definerQuid"] = signer.Id,
        ["subjectQuid"] = signer.Id,
        ["updateNonce"] = 1L,
        ["schemaVersion"] = "1.0",
        ["name"] = "Alice",
        ["homeDomain"] = "interop.home",
        ["attributes"] = new Dictionary<string, object?>
        {
            ["role"] = "admin",
            ["tier"] = 3,
        },
    }, new[] { "signature", "txId" }),

    MakeCase("event-unicode-nested", new Dictionary<string, object?>
    {
        ["type"] = "EVENT",
        ["timestamp"] = 1_700_000_000L,
        ["trustDomain"] = "interop.test",
        ["subjectId"] = signer.Id,
        ["subjectType"] = "QUID",
        ["eventType"] = "NOTE",
        ["sequence"] = 1L,
        ["payload"] = new Dictionary<string, object?>
        {
            ["message"] = "hello 世界 🌍",
            ["nested"] = new Dictionary<string, object?>
            {
                ["z"] = 1,
                ["a"] = "x",
                ["m"] = new[] { 1, 2, 3 },
            },
        },
    }, new[] { "signature", "txId", "publicKey" }),

    MakeCase("numerical-edge", new Dictionary<string, object?>
    {
        ["type"] = "EVENT",
        ["timestamp"] = 1_700_000_000L,
        ["trustDomain"] = "interop.test",
        ["subjectId"] = signer.Id,
        ["subjectType"] = "QUID",
        ["eventType"] = "MEASURE",
        ["sequence"] = 42L,
        ["payload"] = new Dictionary<string, object?>
        {
            ["count"] = 0,
            ["weight"] = 0.5,
            ["n64"] = 9_007_199_254_740_991L,
        },
    }, new[] { "signature", "txId", "publicKey" }),
};

var output = new
{
    sdk = "dotnet",
    version = "2.0.0",
    quid = new { id = signer.Id, publicKeyHex = signer.PublicKeyHex },
    cases,
};

Console.WriteLine(JsonSerializer.Serialize(output, new JsonSerializerOptions
{
    WriteIndented = true,
    Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
}));
