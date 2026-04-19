// Install a 2-of-3 guardian set for an owner quid (QDP-0002).
//
//   cd clients/dotnet/examples/GuardianRecovery
//   dotnet run

using Quidnug.Client;

using var client = new QuidnugClient("http://localhost:8080");
using var owner = Quid.Generate();
using var g1 = Quid.Generate();
using var g2 = Quid.Generate();
using var g3 = Quid.Generate();

await client.RegisterIdentityAsync(owner, name: "Owner");
await client.RegisterIdentityAsync(g1, name: "Guardian-1");
await client.RegisterIdentityAsync(g2, name: "Guardian-2");
await client.RegisterIdentityAsync(g3, name: "Guardian-3");

var newSet = new
{
    subjectQuid = owner.Id,
    guardians = new object[]
    {
        new { quid = g1.Id, weight = 1, epoch = 0 },
        new { quid = g2.Id, weight = 1, epoch = 0 },
        new { quid = g3.Id, weight = 1, epoch = 0 },
    },
    threshold = 2,
    recoveryDelaySeconds = 60, // 60s — demo only; prod uses days
};

var update = new Dictionary<string, object?>
{
    ["subjectQuid"] = owner.Id,
    ["newSet"] = newSet,
    ["anchorNonce"] = 1,
    ["validFrom"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
};

// Each signer adds their signature over the canonical update bytes.
byte[] signable = CanonicalBytes.Of(update,
    "primarySignature", "newGuardianConsents", "currentGuardianSigs");
update["primarySignature"] = new { keyEpoch = 0, signature = owner.Sign(signable) };
update["newGuardianConsents"] = new object[]
{
    new { guardianQuid = g1.Id, keyEpoch = 0, signature = g1.Sign(signable) },
    new { guardianQuid = g2.Id, keyEpoch = 0, signature = g2.Sign(signable) },
    new { guardianQuid = g3.Id, keyEpoch = 0, signature = g3.Sign(signable) },
};

await client.SubmitGuardianSetUpdateAsync(update);
Console.WriteLine($"guardian set installed for {owner.Id}: 2-of-3, 60s delay");

var current = await client.GetGuardianSetAsync(owner.Id);
Console.WriteLine($"readback: threshold={current?.Threshold}, guardians={current?.Guardians?.Count}");
