namespace Quidnug.Client;

/// <summary>Base class for every SDK-raised exception.</summary>
public class QuidnugException : Exception
{
    public IReadOnlyDictionary<string, object?> Details { get; }

    public QuidnugException(string message, IReadOnlyDictionary<string, object?>? details = null)
        : base(message)
    {
        Details = details ?? new Dictionary<string, object?>();
    }

    public QuidnugException(string message, Exception inner, IReadOnlyDictionary<string, object?>? details = null)
        : base(message, inner)
    {
        Details = details ?? new Dictionary<string, object?>();
    }
}

/// <summary>Local precondition failed before any network call.</summary>
public sealed class QuidnugValidationException : QuidnugException
{
    public QuidnugValidationException(string message, IReadOnlyDictionary<string, object?>? details = null)
        : base(message, details) { }
}

/// <summary>Node logically rejected the transaction (nonce replay, quorum, etc.).</summary>
public sealed class QuidnugConflictException : QuidnugException
{
    public QuidnugConflictException(string message, IReadOnlyDictionary<string, object?>? details = null)
        : base(message, details) { }
}

/// <summary>HTTP 503 / feature-not-active / bootstrapping.</summary>
public sealed class QuidnugUnavailableException : QuidnugException
{
    public QuidnugUnavailableException(string message, IReadOnlyDictionary<string, object?>? details = null)
        : base(message, details) { }
}

/// <summary>Transport failure or unexpected 5xx.</summary>
public sealed class QuidnugNodeException : QuidnugException
{
    public int StatusCode { get; }
    public string? ResponseBody { get; }

    public QuidnugNodeException(string message, int statusCode, string? body)
        : base(message)
    {
        StatusCode = statusCode;
        ResponseBody = body is null ? null : (body.Length > 500 ? body[..500] + "..." : body);
    }

    public QuidnugNodeException(string message, Exception inner) : base(message, inner)
    {
        StatusCode = 0;
        ResponseBody = null;
    }
}

/// <summary>Signature / key / crypto failure.</summary>
public sealed class QuidnugCryptoException : QuidnugException
{
    public QuidnugCryptoException(string message) : base(message) { }
}
