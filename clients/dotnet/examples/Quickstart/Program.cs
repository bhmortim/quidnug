// Two-party trust quickstart.
//
// Assumes a local node at http://localhost:8080.
//
//   cd clients/dotnet/examples/Quickstart
//   dotnet run

using Quidnug.Client;

using var client = new QuidnugClient("http://localhost:8080");

var info = await client.InfoAsync();
Console.WriteLine($"connected to node: {info}");

using var alice = Quid.Generate();
using var bob = Quid.Generate();
Console.WriteLine($"alice={alice.Id} bob={bob.Id}");

await client.RegisterIdentityAsync(alice, name: "Alice", homeDomain: "demo.home");
await client.RegisterIdentityAsync(bob,   name: "Bob",   homeDomain: "demo.home");

await client.GrantTrustAsync(alice, trustee: bob.Id, level: 0.9, domain: "demo.home");

var tr = await client.GetTrustAsync(alice.Id, bob.Id, "demo.home");
Console.WriteLine($"trust {tr.TrustLevel:F3} via {string.Join(" -> ", tr.PathOrEmpty)} (depth {tr.PathDepth})");
