using System.Text.Json.Serialization;

namespace Quidnug.Client;

/// <summary>Relational trust query result.</summary>
public sealed record TrustResult(
    [property: JsonPropertyName("observer")]   string Observer,
    [property: JsonPropertyName("target")]     string Target,
    [property: JsonPropertyName("trustLevel")] double TrustLevel,
    [property: JsonPropertyName("trustPath")]  List<string>? Path,
    [property: JsonPropertyName("pathDepth")]  int PathDepth,
    [property: JsonPropertyName("domain")]     string Domain)
{
    public List<string> PathOrEmpty => Path ?? new List<string>();
}

/// <summary>Direct outbound trust edge.</summary>
public sealed record TrustEdge(
    [property: JsonPropertyName("truster")]    string Truster,
    [property: JsonPropertyName("trustee")]    string Trustee,
    [property: JsonPropertyName("trustLevel")] double TrustLevel,
    [property: JsonPropertyName("domain")]     string Domain,
    [property: JsonPropertyName("nonce")]      long Nonce);

/// <summary>Identity record snapshot.</summary>
public sealed record IdentityRecord(
    [property: JsonPropertyName("quidId")]      string QuidId,
    [property: JsonPropertyName("name")]        string? Name,
    [property: JsonPropertyName("homeDomain")]  string? HomeDomain,
    [property: JsonPropertyName("publicKey")]   string? PublicKey,
    [property: JsonPropertyName("updateNonce")] long UpdateNonce);

/// <summary>Ownership stake in a title.</summary>
public sealed record OwnershipStake(
    [property: JsonPropertyName("ownerId")]    string OwnerId,
    [property: JsonPropertyName("percentage")] double Percentage,
    [property: JsonPropertyName("stakeType")]  string? StakeType = null);

/// <summary>Title record.</summary>
public sealed record Title(
    [property: JsonPropertyName("assetId")]      string AssetId,
    [property: JsonPropertyName("domain")]       string? Domain,
    [property: JsonPropertyName("titleType")]    string? TitleType,
    [property: JsonPropertyName("ownershipMap")] List<OwnershipStake>? Owners);

/// <summary>One event in a subject's stream.</summary>
public sealed record Event(
    [property: JsonPropertyName("subjectId")]   string SubjectId,
    [property: JsonPropertyName("subjectType")] string SubjectType,
    [property: JsonPropertyName("eventType")]   string EventType,
    [property: JsonPropertyName("payload")]     Dictionary<string, object>? Payload,
    [property: JsonPropertyName("payloadCid")]  string? PayloadCid,
    [property: JsonPropertyName("timestamp")]   long Timestamp,
    [property: JsonPropertyName("sequence")]    long Sequence);

/// <summary>Guardian entry (QDP-0002).</summary>
public sealed record GuardianRef(
    [property: JsonPropertyName("quid")]   string Quid,
    [property: JsonPropertyName("weight")] int Weight,
    [property: JsonPropertyName("epoch")]  int Epoch);

/// <summary>Guardian set (QDP-0002).</summary>
public sealed record GuardianSet(
    [property: JsonPropertyName("subjectQuid")]          string SubjectQuid,
    [property: JsonPropertyName("guardians")]            List<GuardianRef>? Guardians,
    [property: JsonPropertyName("threshold")]            int Threshold,
    [property: JsonPropertyName("recoveryDelaySeconds")] long RecoveryDelaySeconds);

/// <summary>Cross-domain fingerprint (QDP-0003).</summary>
public sealed record DomainFingerprint(
    [property: JsonPropertyName("domain")]       string Domain,
    [property: JsonPropertyName("blockHeight")]  long BlockHeight,
    [property: JsonPropertyName("blockHash")]    string BlockHash,
    [property: JsonPropertyName("producerQuid")] string ProducerQuid,
    [property: JsonPropertyName("timestamp")]    long Timestamp);
