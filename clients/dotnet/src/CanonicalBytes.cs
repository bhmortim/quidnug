using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace Quidnug.Client;

/// <summary>
/// Canonical signable-bytes encoder — byte-for-byte compatible with
/// the Go, Python, Java, Rust, and JavaScript Quidnug SDKs.
///
/// Rule (see <c>schemas/types/canonicalization.md</c>):
/// <list type="number">
///   <item>Serialize the object to JSON.</item>
///   <item>Parse it back into a generic JSON tree.</item>
///   <item>Serialize again with <b>alphabetized keys</b> (matches Go's
///         <c>encoding/json</c> output for <c>map[string]interface{}</c>).</item>
///   <item>Exclude named top-level fields (typically <c>signature</c>
///         and <c>txId</c>).</item>
/// </list>
/// </summary>
public static class CanonicalBytes
{
    private static readonly JsonSerializerOptions SerializerOpts = new()
    {
        WriteIndented = false,
    };

    /// <summary>
    /// Return the canonical signable bytes for a value.
    /// </summary>
    public static byte[] Of(object obj, params string[] excludeFields)
    {
        if (obj is null) throw new ArgumentNullException(nameof(obj));

        JsonNode? node = obj is JsonNode n ? n.DeepClone() : JsonSerializer.SerializeToNode(obj, SerializerOpts);
        if (node is not JsonObject root)
            throw new ArgumentException("expected an object at the root", nameof(obj));

        foreach (string f in excludeFields) root.Remove(f);

        JsonNode sorted = SortKeysDeep(root);
        string json = sorted.ToJsonString();
        return Encoding.UTF8.GetBytes(json);
    }

    internal static JsonNode SortKeysDeep(JsonNode n)
    {
        switch (n)
        {
            case JsonObject obj:
                var keys = obj.Select(kv => kv.Key).OrderBy(k => k, StringComparer.Ordinal).ToList();
                var fresh = new JsonObject();
                foreach (var k in keys)
                {
                    var v = obj[k];
                    fresh[k] = v is null ? null : SortKeysDeep(v.DeepClone());
                }
                return fresh;
            case JsonArray arr:
                var clonedItems = arr.ToList();
                var result = new JsonArray();
                foreach (var item in clonedItems)
                {
                    result.Add(item is null ? null : SortKeysDeep(item.DeepClone()));
                }
                return result;
            default:
                return n;
        }
    }
}
