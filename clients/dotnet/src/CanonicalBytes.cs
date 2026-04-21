using System.Buffers;
using System.Globalization;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace Quidnug.Client;

/// <summary>
/// Canonical signable-bytes encoder — byte-for-byte compatible with
/// the Go, Python, Java, Rust, and JavaScript Quidnug SDKs.
///
/// <para>Two entry points:</para>
/// <list type="bullet">
///   <item><see cref="V1Of"/> — v1.0-spec-conformant. Preserves
///         caller-supplied top-level key order (which must mirror
///         Go struct declaration order for the tx type); sorts
///         nested object keys alphabetically to match Go's
///         <c>encoding/json</c> default for
///         <c>map[string]interface{}</c>.</item>
///   <item><see cref="Of"/> — legacy fully-sorted mode. Sorts
///         every level, including the top. Does NOT match the
///         v1.0 canonical form; signatures produced this way
///         will not verify against a v1.0 node. Deprecated.</item>
/// </list>
///
/// <para>Non-ASCII characters emit as raw UTF-8 (not \uXXXX
/// escapes). We write our own JSON serializer rather than using
/// <see cref="JsonSerializer"/> because even
/// <c>JavaScriptEncoder.UnsafeRelaxedJsonEscaping</c> emits
/// supplementary-plane code points as <c>\uXXXX\uXXXX</c>, which
/// would break cross-SDK interop.</para>
/// </summary>
public static class CanonicalBytes
{
    /// <summary>
    /// v1.0-conformant canonical bytes: preserve top-level
    /// caller insertion order, sort nested object keys
    /// alphabetically.
    ///
    /// <para>Pass transaction fields in Go struct declaration
    /// order (the schema for each tx type in
    /// <c>docs/protocol-v1.0.md</c> §4). Any nested
    /// <see cref="JsonObject"/> inside the root will have its
    /// keys sorted before serialization.</para>
    /// </summary>
    public static byte[] V1Of(object obj, params string[] excludeFields)
    {
        if (obj is null) throw new ArgumentNullException(nameof(obj));

        JsonNode? node = obj is JsonNode n
            ? n.DeepClone()
            : JsonSerializer.SerializeToNode(obj);
        if (node is not JsonObject root)
            throw new ArgumentException("expected an object at the root", nameof(obj));

        foreach (string f in excludeFields) root.Remove(f);

        var buf = new ArrayBufferWriter<byte>(256);
        WriteTopLevelObject(buf, root);
        return buf.WrittenSpan.ToArray();
    }

    /// <summary>
    /// Legacy fully-sorted canonical bytes. Sorts every level,
    /// including the top. Does NOT match the v1.0 canonical
    /// form; retained for backward compatibility with pre-v1.0
    /// signing paths.
    /// </summary>
    [Obsolete("Use V1Of for signatures bound for a v1.0 node.")]
    public static byte[] Of(object obj, params string[] excludeFields)
    {
        if (obj is null) throw new ArgumentNullException(nameof(obj));

        JsonNode? node = obj is JsonNode n
            ? n.DeepClone()
            : JsonSerializer.SerializeToNode(obj);
        if (node is not JsonObject root)
            throw new ArgumentException("expected an object at the root", nameof(obj));

        foreach (string f in excludeFields) root.Remove(f);

        var buf = new ArrayBufferWriter<byte>(256);
        WriteNode(buf, root);
        return buf.WrittenSpan.ToArray();
    }

    private static void WriteNode(IBufferWriter<byte> buf, JsonNode? n)
    {
        switch (n)
        {
            case null:
                WriteAscii(buf, "null");
                return;
            case JsonObject obj:
                WriteObject(buf, obj);
                return;
            case JsonArray arr:
                WriteArray(buf, arr);
                return;
            case JsonValue val:
                WriteValue(buf, val);
                return;
        }
    }

    /// <summary>
    /// Write the root object preserving the caller's key order.
    /// Nested objects are written via <see cref="WriteObject"/>,
    /// which sorts their keys (so nested maps match Go's
    /// alphabetical map-marshal output).
    /// </summary>
    private static void WriteTopLevelObject(IBufferWriter<byte> buf, JsonObject obj)
    {
        buf.Write("{"u8);
        bool first = true;
        foreach (var kv in obj)
        {
            if (!first) buf.Write(","u8);
            first = false;
            WriteString(buf, kv.Key);
            buf.Write(":"u8);
            WriteNode(buf, kv.Value);
        }
        buf.Write("}"u8);
    }

    private static void WriteObject(IBufferWriter<byte> buf, JsonObject obj)
    {
        buf.Write("{"u8);
        var keys = obj.Select(kv => kv.Key).OrderBy(k => k, StringComparer.Ordinal).ToList();
        for (int i = 0; i < keys.Count; i++)
        {
            if (i > 0) buf.Write(","u8);
            WriteString(buf, keys[i]);
            buf.Write(":"u8);
            WriteNode(buf, obj[keys[i]]);
        }
        buf.Write("}"u8);
    }

    private static void WriteArray(IBufferWriter<byte> buf, JsonArray arr)
    {
        buf.Write("["u8);
        for (int i = 0; i < arr.Count; i++)
        {
            if (i > 0) buf.Write(","u8);
            WriteNode(buf, arr[i]);
        }
        buf.Write("]"u8);
    }

    private static void WriteValue(IBufferWriter<byte> buf, JsonValue v)
    {
        var elem = v.GetValue<JsonElement>();
        switch (elem.ValueKind)
        {
            case JsonValueKind.String:
                WriteString(buf, elem.GetString() ?? string.Empty);
                break;
            case JsonValueKind.Number:
                WriteNumber(buf, elem);
                break;
            case JsonValueKind.True:
                WriteAscii(buf, "true");
                break;
            case JsonValueKind.False:
                WriteAscii(buf, "false");
                break;
            case JsonValueKind.Null:
                WriteAscii(buf, "null");
                break;
            default:
                // Object/Array shouldn't appear inside JsonValue.
                WriteAscii(buf, "null");
                break;
        }
    }

    private static void WriteNumber(IBufferWriter<byte> buf, JsonElement elem)
    {
        // Try int64 first (most numbers in Quidnug payloads are integer-
        // valued) so we avoid "1.0" when the source was 1. Fall back to
        // decimal / double.
        if (elem.TryGetInt64(out long l))
        {
            WriteAscii(buf, l.ToString(CultureInfo.InvariantCulture));
            return;
        }
        if (elem.TryGetDouble(out double d))
        {
            WriteAscii(buf, d.ToString("R", CultureInfo.InvariantCulture));
            return;
        }
        WriteAscii(buf, elem.GetRawText());
    }

    private static void WriteString(IBufferWriter<byte> buf, string s)
    {
        buf.Write("\""u8);
        // JSON requires escaping: " \ and control chars below 0x20.
        // Non-ASCII characters ARE emitted as raw UTF-8.
        int i = 0;
        while (i < s.Length)
        {
            int code;
            int consumed;
            if (char.IsHighSurrogate(s[i]) && i + 1 < s.Length && char.IsLowSurrogate(s[i + 1]))
            {
                code = char.ConvertToUtf32(s[i], s[i + 1]);
                consumed = 2;
            }
            else
            {
                code = s[i];
                consumed = 1;
            }

            switch (code)
            {
                case '"':  buf.Write("\\\""u8); break;
                case '\\': buf.Write("\\\\"u8); break;
                case '\n': buf.Write("\\n"u8); break;
                case '\r': buf.Write("\\r"u8); break;
                case '\t': buf.Write("\\t"u8); break;
                case '\b': buf.Write("\\b"u8); break;
                case '\f': buf.Write("\\f"u8); break;
                default:
                    if (code < 0x20)
                    {
                        // control char
                        WriteAscii(buf, $"\\u{code:x4}");
                    }
                    else
                    {
                        // emit raw UTF-8
                        string slice = s.Substring(i, consumed);
                        int bytesNeeded = Encoding.UTF8.GetByteCount(slice);
                        Span<byte> span = buf.GetSpan(bytesNeeded);
                        int written = Encoding.UTF8.GetBytes(slice, span);
                        buf.Advance(written);
                    }
                    break;
            }
            i += consumed;
        }
        buf.Write("\""u8);
    }

    private static void WriteAscii(IBufferWriter<byte> buf, string s)
    {
        Span<byte> span = buf.GetSpan(s.Length);
        for (int i = 0; i < s.Length; i++) span[i] = (byte)s[i];
        buf.Advance(s.Length);
    }
}
