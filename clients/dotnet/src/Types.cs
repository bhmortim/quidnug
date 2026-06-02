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

/// <summary>Endpoint advertised by a node (QDP-0014).</summary>
public sealed class NodeAdvertEndpoint
{
    [JsonPropertyName("url")]                       public string Url { get; set; } = "";
    [JsonPropertyName("protocol")]                  public string? Protocol { get; set; }
    [JsonPropertyName("region")]                    public string? Region { get; set; }
    [JsonPropertyName("priority")]                  public int Priority { get; set; }
    [JsonPropertyName("weight")]                    public int Weight { get; set; }
}

/// <summary>Capabilities a node advertises (QDP-0014).</summary>
public sealed class NodeAdvertCapabilities
{
    [JsonPropertyName("validator")]                 public bool Validator { get; set; }
    [JsonPropertyName("cache")]                     public bool Cache { get; set; }
    [JsonPropertyName("archive")]                   public bool Archive { get; set; }
    [JsonPropertyName("bootstrap")]                 public bool Bootstrap { get; set; }
    [JsonPropertyName("gossipSink")]                public bool GossipSink { get; set; }
    [JsonPropertyName("ipfsGateway")]               public bool IpfsGateway { get; set; }
    [JsonPropertyName("maxBodyBytes")]              public int MaxBodyBytes { get; set; }
    [JsonPropertyName("minPeerProtocol")]           public string? MinPeerProtocol { get; set; }
}

/// <summary>Writable parameters for <c>PublishNodeAdvertisementAsync</c> (QDP-0014).</summary>
public sealed class NodeAdvertisementParams
{
    public string OperatorQuid { get; set; } = "";
    public string Domain { get; set; } = "";
    public List<NodeAdvertEndpoint> Endpoints { get; set; } = new();
    public List<string>? SupportedDomains { get; set; }
    public NodeAdvertCapabilities Capabilities { get; set; } = new();
    public string ProtocolVersion { get; set; } = "";
    /// <summary>TTL for the advertisement; defaults to 6h, max 7d.</summary>
    public TimeSpan Ttl { get; set; } = TimeSpan.Zero;
    /// <summary>Strictly monotonic per-NodeQuid nonce; must be &gt; 0.</summary>
    public long AdvertisementNonce { get; set; }
}

/// <summary>Arguments for <c>DiscoverQuidsAsync</c> (QDP-0014).</summary>
public sealed class DiscoverQuidsParams
{
    /// <summary>Required.</summary>
    public string Domain { get; set; } = "";
    /// <summary>UnixNano lower bound.</summary>
    public long Since { get; set; }
    /// <summary>"activity" | "last-seen" | "first-seen" | "trust-weight".</summary>
    public string? Sort { get; set; }
    /// <summary>Observer quid; enables trust-weight sort and trustWeight column.</summary>
    public string? Observer { get; set; }
    public string? EventType { get; set; }
    public double MinTrustWeight { get; set; }
    public List<string>? ExcludeQuids { get; set; }
    /// <summary>Default 50; max 500.</summary>
    public int Limit { get; set; }
    public int Offset { get; set; }
}
