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
/// Rule (see <c>schemas/types/canonicalization.md</c>):
/// <list type="number">
///   <item>Serialize the object to JSON.</item>
///   <item>Parse it back into a generic JSON tree.</item>
///   <item>Serialize again with <b>alphabetized keys</b> (matches Go's
///         <c>encoding/json</c> output for <c>map[string]interface{}</c>).</item>
///   <item>Exclude named top-level fields (typically <c>signature</c>
///         and <c>txId</c>).</item>
///   <item>Non-ASCII characters emit as raw UTF-8 — NOT as \uXXXX
///         escapes. Matches Go's encoding/json default. Critical for
///         cross-SDK signature interop.</item>
/// </list>
///
/// <para>We write our own JSON serializer (rather than configuring
/// <see cref="JsonSerializer"/>) because even
/// <c>JavaScriptEncoder.UnsafeRelaxedJsonEscaping</c> escapes
/// supplementary-plane code points as <c>\uXXXX\uXXXX</c> surrogate
/// pairs, which would break interop with every other SDK. Only a
/// custom writer can emit raw UTF-8 for all scalars uniformly.</para>
/// </summary>
public static class CanonicalBytes
{
    /// <summary>
    /// Return the canonical signable bytes for a value.
    /// </summary>
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
