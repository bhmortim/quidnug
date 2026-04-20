# Architecture: Elections on Quidnug

Detailed data model, protocol flows, and component breakdown.
Assumes familiarity with [`README.md`](README.md).

## Participating components

A production election deployment involves these Quidnug node
roles:

| Node role                        | Operator                          | Responsibilities                                        |
|----------------------------------|-----------------------------------|--------------------------------------------------------|
| **Election Authority Node**      | County / state election office    | Signs registrations, issues ballots, tallies            |
| **Precinct Node**                | Individual polling place          | Poll-book lookups, voting-booth backend                 |
| **Observer Node (read-only)**    | Political parties, press, NGOs    | Independent copy of the chain for verification          |
| **Audit Node**                   | State audit board, academic groups | Statistical audits, paper-vs-digital cross-verification |
| **Voter Device**                 | Each voter                        | Generates VRQ + BQ, signs transactions                  |
| **Secretary of State Node**      | State SOS                         | Aggregates across counties, certifies state-wide races  |

All nodes run the same Quidnug protocol. Differences are
permissions (who can sign what) and read-only vs. read-write on
specific domains.

## Cryptographic primitives used

| Primitive                  | Used for                                              |
|----------------------------|-------------------------------------------------------|
| ECDSA P-256 (standard)    | All signatures on trust edges, events, titles         |
| SHA-256                    | Quid IDs, block hashes, canonical-bytes hashing       |
| **Blind signature (aux)**  | Ballot issuance (anonymous to authority)              |
| Merkle tree                | Registration snapshots, ballot-box audit              |
| HMAC-SHA256               | Inter-node auth (existing Quidnug mechanism)          |
| Nullifier (hash-based)    | One-vote-per-contest enforcement                      |

The blind-signature piece is the only crypto primitive beyond
Quidnug's current library. See §Blind-signature integration for
how this is wired in.

## Domain hierarchy (complete)

```
elections.<jurisdiction>.<cycle>
├── .meta                                       (election-level config)
├── .candidates                                 (candidate quid identity transactions)
├── .registration                               (voter roll)
├── .poll-book.<precinct>                       (precinct-scoped registration subset)
├── .ballot-issuance                            (ballot.issued events)
├── .ballots                                    (BQ identity transactions)
├── .contests.<contest>                         (trust edges = votes)
├── .tally                                      (official tallies)
├── .audit                                      (audit observations, RLA events)
└── .certification                              (final certification events)

For primaries (parallel hierarchy):
elections.<jurisdiction>.<cycle>.primary.<party>.<same-structure>
```

Each sub-domain has:
- Validators (the election authority itself, typically)
- Storage rules (trusted-tier only from authority, Tentative-tier
  from observers, etc.)
- Visibility: most are public-read, public-append-signed

## Quid schemas

### Election Authority Quid

```go
type ElectionAuthorityIdentity struct {
    QuidID         string  // e.g. "election-authority-williamson-2026-nov"
    Jurisdiction   string  // "williamson-county-tx"
    ElectionCycle  string  // "2026-nov-general"
    Attributes     struct {
        OfficialTitle    string    // "Williamson County Election Administrator"
        MasterSchedule   struct {
            PollsOpen         time.Time
            PollsClose        time.Time
            CertificationDue  time.Time
        }
        PrimaryParties   []string  // ["democratic", "republican"] if primary
        ContestCount     int
        PrecinctCount    int
    }
}

// Governor quorum for the authority's domain tree (QDP-0012).
// This is the policy layer — who authorizes consortium
// changes, threshold tweaks, child-domain delegation.
type AuthorityGovernors struct {
    Chief            string   // chief election official (e.g. county clerk)
    StateSOS         string   // Secretary of State's quid
    BipartisanBoard  []string // D-aligned, R-aligned, independent
    ObserverPanel    []string // League of Women Voters, etc.
    Quorum           float64  // e.g., 0.7 (5-of-7 weighted)
    NoticeBlocks     int64    // e.g., 1440 (~72h at 3-minute blocks)
    EmergencyClause  bool     // if true, REMOVE_VALIDATOR gets 1h notice
                              // during active voting week
}

// Each individual governor ALSO has a personal guardian set
// (QDP-0002) for recovering their own key if lost/compromised.
// These are separate humans from the governor quorum above;
// typically chosen per-governor for their life context
// (spouse, lawyer, federal monitor, etc.).
type GovernorGuardians struct {
    GovernorQuid     string
    Guardians        []string // quids of the guardian humans/orgs
    Threshold        uint16   // e.g., 3-of-5
    RecoveryDelay    time.Duration  // e.g., 24h typical
}
```

**Two-layer key-resilience model.** The governor quorum is the
policy layer — it changes the authority's validator set,
domain parameters, delegation scopes. The per-governor
guardian quorums are the key-recovery layer — each governor
individually can recover a lost key without waiting for quorum
agreement. Both layers are needed; one without the other
leaves obvious failure modes (losing a key = stuck, losing a
policy vote = stuck).

### Voter Registration Quid (VRQ)

```go
type VoterRegistrationQuid struct {
    QuidID      string     // sha256 of voter's public key, 16 hex chars
    PublicKey   string     // voter-generated ECDSA P-256 public key
    Creator     string     // voter's own quid (self-issued)
    Attributes  struct {
        // NOTE: No real-world PII on-chain.
        //       Precinct is public; matched off-chain to name+address
        //       by the authority's secure registration DB.
    }
}
```

The VRQ exists **before** the registration trust edge. The edge
is what grants eligibility; the quid is the voter's identity.

### Voter Registration Trust Edge (the "registration record")

```go
// Issued by election authority after identity verification.
type RegistrationTrustEdge struct {
    Truster       string  // "election-authority-..."
    Trustee       string  // voter's VRQ
    TrustLevel    float64 // 1.0 (registered)
    Domain        string  // "elections.<jurisdiction>.<cycle>.registration"
    Nonce         int64   // monotonic per voter per election
    Attributes    struct {
        Precinct              string
        RegisteredParty       string  // "democratic" | "republican" | "unaffiliated" | ""
        RegistrationMethod    string  // "in-person" | "online" | "dmv" | "mail"
        RegistrationTimestamp int64
    }
    ValidUntil    int64
    Signature     string  // authority's signature
}
```

### Ballot Quid (BQ)

```go
type BallotQuid struct {
    QuidID      string     // voter-generated, e.g., "ballot-Xz7mN2pQ8rK4vL9t"
    PublicKey   string     // voter-generated ECDSA key (different from VRQ's)
    Creator     string     // election-authority (via blind signature — see below)
    Attributes  struct {
        ElectionID          string
        ContestID           string  // optional: BQ-per-contest for enhanced anonymity
        IssuanceEpoch       uint32
        IssuanceMethod      string  // "in-person" | "mail-in" | "early-vote"
    }
    // The signature on this identity transaction is the
    // UNBLINDED authority signature from the blind-signature flow.
    AuthoritySignature string
}
```

Critical property: the authority's signature here is
cryptographically valid (verifies against authority's public
key) but the authority never saw the raw QuidID. This is the
mechanism by which eligibility is proven while anonymity is
preserved.

### Contest Quid

```go
type ContestQuid struct {
    QuidID       string
    Attributes   struct {
        Title             string
        Description       string
        EligiblePrecincts []string
        StartTime         int64
        EndTime           int64
        VotingMethod      string  // "plurality" | "ranked-choice-irv" | "approval" | "rated-3-star" | "yes-no"
        MaxApproved       int     // for approval voting
        Candidates        []string  // list of candidate quids
        TallyDomain       string    // the domain where trust edges count as votes
        RequiresMajority  bool    // for runoffs
    }
    Creator      string  // election-authority
}
```

### Candidate Quid

```go
type CandidateQuid struct {
    QuidID      string   // candidate-generated, e.g., "candidate-jane-smith-2026-senate"
    PublicKey   string   // candidate's signing key
    Creator     string   // candidate themselves
    Attributes  struct {
        LegalName       string
        Party           string
        Office          string
        Bio             string  // or CID to bio blob
        Website         string
        FilingStatement string  // or CID to candidacy paperwork
    }
}

// Authority confirms ballot placement via a trust edge
// election-authority → candidate-quid in domain
// "elections.<j>.<c>.candidates" with attribute of "contest".
```

### Events

All events use Quidnug's standard EVENT transaction. Specific
event types for elections:

```
subject=VRQ:
  voter.registered
  voter.checked-in                # at polling place
  voter.voted                     # final "all contests cast" event
  ballot.issued                   # per-contest issuance record (attached to VRQ)
  voter.registration-updated
  voter.registration-revoked      # for cause

subject=BQ:
  (none directly on BQ — its existence + signed identity is self-contained.
   Votes are trust edges from BQ, not events.)

subject=Contest:
  contest.opened
  contest.closed
  contest.tallied
  contest.challenged
  contest.certified

subject=Paper-Ballot-Box:
  ballot-box.sealed
  ballot-box.opened
  ballot-box.scanned
  ballot-box.discrepancy-detected

subject=Election-Authority:
  election.setup-complete
  election.pre-audit-passed
  election.final-certified
  election.dispute-filed
```

## Protocol flows

### Pre-election setup (T-90 days)

```
1. State SOS establishes the operator quid (the persistent
   identity for the county election authority):
     IDENTITY: election-authority-williamson
     (separate from the per-election quid below)

2. Each governor generates their personal quid + installs
   their own guardian set (QDP-0002):
     For each governor G:
       GuardianSetUpdate for G.quid, signed by G
       Guardians: whatever humans G has chosen
       Recovery delay: 24h typical

3. Authority registers the per-election domain tree using
   DOMAIN_REGISTRATION transactions (QDP-0012):
     DOMAIN_REGISTRATION: elections.williamson-county-tx.2026-nov
       validators: {node-authority-primary: 1.0, ...}
       governors: {chief: 2, sos: 2, r-obs: 1, d-obs: 1, lwv: 1}
       governanceQuorum: 0.7  # 5-of-7 weighted
       noticePeriod: 1440 blocks
     (repeat for registration, poll-book, ballot-issuance,
      contests.*, tally, audit sub-domains)

4. Authority publishes the well-known file (QDP-0014):
     /.well-known/quidnug-network.json
       operator: <county-authority-quid>
       governors: [chief, sos, r-obs, d-obs, lwv with weights]
       seeds: [node-authority-primary, node-observer-r, ...]
       signed by county-authority operator key

5. Each consortium node publishes a NODE_ADVERTISEMENT
   (QDP-0014):
     NODE_ADVERTISEMENT for each node
       operator: county-authority-quid (or observer org's operator)
       endpoints: [https://node-xxx.elections.wilco.gov]
       capabilities: {validator: true, archive: true}
       supportedDomains: ["elections.williamson-county-tx.2026-nov.*"]
       expires: +7 days (renewed weekly)

6. Authority creates contest quids:
     For each race: IDENTITY: contest-williamson-2026-us-senate
     etc.

7. Candidates file their candidacy:
     Each candidate creates their own candidate quid, signs an
     identity transaction, and submits to the authority.
     Authority reviews, emits a trust edge from election-authority
     to candidate-quid in domain ".candidates" attributing
     which contest they're registered for.

5. Precinct structure:
     Authority publishes the precinct list (contest→precinct
     mapping) as another signed event.

6. Observer nodes bootstrap:
     Via K-of-K (QDP-0008) from the authority node + state SOS
     node + 1-2 trusted observer nodes.
```

### Voter registration flow (T-270 to T-30 days)

```
┌────────────────────────────────────────────────────────────┐
│ Voter: generates VRQ locally                                 │
│    (uses quidnug-voter CLI or mobile app)                    │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│ Voter: visits DMV / registers online / visits county office │
│    Brings: ID document + VRQ's public key (as QR)           │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│ Registrar: verifies identity (off-chain)                     │
│    Looks up voter in state DB of eligible voters, confirms   │
│    not already registered. Links VRQ to off-chain record.    │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│ Authority's signing service: emits trust edge                 │
│    TRUST: authority → VRQ                                      │
│    domain: elections.<j>.<c>.registration                      │
│    attributes: {precinct, party, method, timestamp}           │
│                                                                │
│ And event:                                                     │
│    voter.registered on the VRQ                                │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│ Push gossip: edge propagates to all precinct nodes + SOS +  │
│    observer nodes within seconds                              │
└─────────────────────────────────────────────────────────────┘
```

### Poll-book query (election day)

```
┌─────────────────────┐
│ Voter at polling    │
│ place. Shows ID.    │
└──────────┬──────────┘
           │
┌──────────▼──────────────────────────────────────┐
│ Poll worker tablet queries local precinct node:   │
│   GET /api/v1/trust/edges?                         │
│     truster=election-authority-williamson-2026-nov│
│     trustee=<voter-VRQ>                            │
│     domain=elections.williamson-2026-nov.registration│
│                                                     │
│ Expected: 1 edge with precinct=042 attribute      │
│ + no voter.voted event on this VRQ yet            │
└──────────┬──────────────────────────────────────┘
           │ OK
┌──────────▼─────────────────────────────────────┐
│ Poll worker emits voter.checked-in event         │
│   EVENT: voter.checked-in on VRQ                 │
│   Signed by poll-worker quid                     │
│                                                  │
│ Voter proceeds to booth                          │
└──────────────────────────────────────────────────┘
```

### Ballot issuance (blind-signature)

This is the critical flow. Inside the voting booth:

```
┌───────────────────────────────────────────────────────────┐
│ Voter's booth device generates fresh BQs                    │
│   For plurality election with 5 contests, generate 5 BQs    │
│   (one per contest, for enhanced anonymity)                 │
└──────────────────────────┬────────────────────────────────┘
                           │
┌──────────────────────────▼────────────────────────────────┐
│ For each BQ, device builds a BLINDED COMMITMENT             │
│   commitment_i = Blind(BQ_i.id, random_r_i)                 │
└──────────────────────────┬────────────────────────────────┘
                           │
┌──────────────────────────▼────────────────────────────────┐
│ Device presents to authority's Issuance Service:             │
│   POST /api/v2/elections/issue-ballot                        │
│   Body (signed with VRQ's private key):                      │
│     electionId: "elections.williamson-2026-nov"              │
│     contests: [                                               │
│       { contestId: "us-senate",     commitment: ... },      │
│       { contestId: "governor",      commitment: ... },      │
│       { contestId: "proposition-5", commitment: ... },      │
│       ...                                                     │
│     ]                                                          │
│     nullifiers: [                                              │
│       { contestId: "us-senate",     nullifier: ... },        │
│       ...                                                      │
│     ]                                                          │
└──────────────────────────┬────────────────────────────────┘
                           │
┌──────────────────────────▼────────────────────────────────┐
│ Authority's Issuance Service:                                 │
│   1. Verify VRQ signature on request                         │
│   2. Verify VRQ is registered (trust edge exists)            │
│   3. Verify VRQ has been checked in (voter.checked-in event) │
│   4. For each contest:                                        │
│      a. Nullifier not already seen in this contest's domain  │
│      b. If primary: VRQ's party matches contest's party      │
│      c. Contest is currently OPEN                             │
│   5. Emit ballot.issued events on the VRQ:                   │
│      EVENT: ballot.issued                                     │
│        payload: { contestId, nullifier }                      │
│      (public record — "VRQ received a ballot for this       │
│      contest" without revealing which BQ)                    │
│   6. Sign each commitment with authority's blind-sig key     │
│   7. Return blinded_signatures[] to voter                    │
└──────────────────────────┬────────────────────────────────┘
                           │
┌──────────────────────────▼────────────────────────────────┐
│ Voter's device unblinds each signature:                      │
│   For each (blinded_sig_i, r_i):                             │
│     unblinded_sig_i = Unblind(blinded_sig_i, r_i)            │
│                                                               │
│ Now each BQ has an authority-signed identity transaction.   │
│ Device holds onto BQ private keys + unblinded sigs.          │
└──────────────────────────┬────────────────────────────────┘
                           │
┌──────────────────────────▼────────────────────────────────┐
│ Ballots are now "issued" — voter can cast.                    │
└───────────────────────────────────────────────────────────┘
```

**Nullifier scheme** — one subtle point. The nullifier prevents
the voter from requesting multiple ballots for the same contest.
It's derived deterministically:

```
nullifier_for_contest = HMAC-SHA256(
    key = voter-VRQ-private-key,
    message = "ballot-issuance" + electionId + contestId
)
```

Two properties:
1. Only the voter (holding VRQ private key) can compute it.
2. The same voter for the same contest always gets the same
   nullifier.

The authority stores all seen nullifiers per contest. Second
request with the same nullifier is rejected. The nullifier
itself doesn't reveal the voter's identity (it's a hash,
appears random to observers).

### Vote casting

```
Inside the booth, voter makes selections on touchscreen UI:

  US Senate: ● Jane Smith   ○ Bob Jones   ○ Carol Li
  Governor:  ○ Alice Wang   ● Dan Chen    ○ (no vote)
  Proposition 5: ● YES  ○ NO

Voter confirms. Device emits:

For each vote:
  TRUST transaction from BQ to candidate/option:

    For US Senate vote:
      truster: ballot-Xz7m... (BQ for us-senate contest)
      trustee: candidate-jane-smith-2026-senate
      trustLevel: 1.0
      domain: elections.williamson-2026-nov.contests.us-senate
      nonce: 1
      signature: <signed with BQ's private key>

    For Prop 5 vote:
      truster: ballot-ABC9... (BQ for prop-5 contest)
      trustee: option-yes-williamson-2026
      trustLevel: 1.0
      domain: elections.williamson-2026-nov.contests.proposition-5
      nonce: 1
      signature: <signed with BQ's private key>

  (No vote in Governor: NO trust edge emitted.)

Each BQ's identity transaction is also submitted alongside its
first vote so the chain has a record of the BQ's authorized
existence.

The BQ's identity transaction's signature (from authority) is
verified at tally time. The trust edge's signature (from BQ) is
verified at submission time.

Voter's voter.voted event is emitted on their VRQ:
  EVENT: voter.voted
  payload: { electionId, contestCount: 5, completedAt: ... }
  signer: voting-booth-quid + VRQ-cosigned
```

### Tally

```go
func (t *Tallier) TallyContest(ctx context.Context, contestID string) (*Tally, error) {
    contest, err := t.GetContest(ctx, contestID)
    if err != nil { return nil, err }

    // 1. Get all trust edges in this contest's tally domain.
    edges, err := t.GetTrustEdges(ctx, &GetEdgesFilter{
        Domain: contest.Attributes["tallyDomain"].(string),
    })
    if err != nil { return nil, err }

    // 2. For each edge, validate.
    valid := []Edge{}
    for _, e := range edges {
        // a. Truster must be a valid BQ for this election.
        bqID, err := t.GetIdentity(ctx, e.Truster)
        if err != nil { continue }
        if bqID.Creator != t.authorityQuid { continue }
        if bqID.Attributes["electionId"] != contest.Attributes["electionId"] { continue }

        // b. Authority's signature on BQ identity must verify.
        if !t.verifyAuthoritySignature(bqID) { continue }

        // c. Trust edge's signature must verify (BQ signed it).
        if !t.verifyEdgeSignature(e, bqID.PublicKey) { continue }

        // d. For primary contests, BQ's party must match contest.

        valid = append(valid, e)
    }

    // 3. Apply voting method.
    switch contest.Attributes["votingMethod"] {
    case "plurality":
        return t.TallyPlurality(contest, valid), nil
    case "ranked-choice-irv":
        return t.TallyIRV(contest, valid), nil
    case "approval":
        return t.TallyApproval(contest, valid), nil
    case "yes-no":
        return t.TallyYesNo(contest, valid), nil
    }
    return nil, fmt.Errorf("unknown voting method")
}
```

### Paper-ballot cross-verification

```
End of election day:

1. Poll workers seal physical ballot box.
   Event: ballot-box.sealed
     payload: { precinct, physicalSeal: <tamper-seal-serial> }
   signer: poll-workers-quorum

2. Ballot box transported to counting facility (chain of
   custody documented in Quidnug events throughout).

3. Counting facility opens box.
   Event: ballot-box.opened
     payload: { sealVerified: true | false, observedBy: [...] }

4. Paper ballots are scanned.
   Each paper ballot's QR code yields: BQ-ID + authority
   issuance signature.
   Compared against the digital chain:
     - Does the digital chain have a trust edge from this BQ
       in each contest marked on the paper?
     - Do the digital trust-edge targets match the paper?

5. Discrepancies logged:
   Event: ballot-box.discrepancy-detected
     payload: { ballotID, expected: ..., actual: ..., reason: ... }

6. Statistical audit (risk-limiting):
   Sample N random paper ballots.
   For each: verify paper-to-digital match.
   Compute statistical confidence level.
   Event: audit.rla-completed
     payload: { sampleSize, matches, confidenceLevel, pass: true }
```

### Certification

```
After certification period elapses:

1. All recounts finalized.
2. All disputes resolved (or time-barred).
3. Risk-limiting audit passed.

Authority emits certification events per contest:
  EVENT: contest.certified
    subjectId: contest quid
    payload:
      tallyResult: { ... }
      totalEligibleVoters: ...
      totalCastBallots: ...
      disputesResolved: [...]
      auditsPassed: true
      certifiedBy: election-authority-williamson-2026-nov
    cosignedBy:
      - chief-election-admin
      - bipartisan-board-D
      - bipartisan-board-R
      - state-sos
```

## Blind-signature integration

This is the one non-standard-Quidnug piece. The authority's
Ballot Issuance Service implements a blind-signature scheme
against the authority's issuance key.

### Options for implementation

**Option A (Recommended): RSA blind signatures as auxiliary**

Authority maintains a separate RSA keypair for blind signatures.
The public key is published in the authority's attributes. Each
issuance key is short-lived (per election); expired keys are
retired (another `election.issuance-key-retired` event).

Voter code:
```python
# Pseudocode
commitment = blind(BQ_id, authority_pubkey, random_r)
POST /api/v2/elections/issue-ballot
response = blinded_signature
unblinded_sig = unblind(blinded_signature, random_r)
# unblinded_sig verifies against authority_pubkey on BQ_id
```

Authority side:
```python
signature = rsa_sign(commitment, authority_privkey)
```

Quidnug's identity transaction storage accepts the unblinded
signature as the authority's signature field; verification is a
standard RSA verify.

**Option B: EC-Schnorr blind signatures** (deferred pending QDP)

More aligned with Quidnug's P-256 family, but Schnorr blind
signatures have known concurrency attacks that need careful
protocol design (blind Schnorr requires ROS-hard setup or
alternatives like Okamoto-Schnorr).

### The issuance service

```go
type IssuanceService struct {
    authorityQuid        string
    authorityPrivateKey  ecdsa.PrivateKey  // for Quidnug signatures
    blindSigPrivateKey   rsa.PrivateKey    // for blind sigs
    ledger               *NonceLedger
    nullifierStore       map[string]map[string]bool  // contest → nullifier → seen
}

func (s *IssuanceService) IssueBallot(req IssueRequest) (*IssueResponse, error) {
    // Verify request signed with VRQ's private key.
    if !s.verifyVRQSig(req) {
        return nil, ErrBadVRQSignature
    }

    // Check VRQ is registered for this election.
    if !s.isRegistered(req.VRQ, req.ElectionID) {
        return nil, ErrNotRegistered
    }

    // Check VRQ has checked in (or is otherwise eligible — mail, early vote).
    if !s.hasCheckedIn(req.VRQ, req.ElectionID) {
        return nil, ErrNotCheckedIn
    }

    // For each contest in the request:
    response := &IssueResponse{}
    for _, contest := range req.Contests {
        // 1. Nullifier not already seen.
        if s.nullifierStore[contest.ContestID][contest.Nullifier] {
            return nil, fmt.Errorf("already voted in contest %s", contest.ContestID)
        }

        // 2. For primary: VRQ's party matches contest's party.
        if isPrimaryContest(contest.ContestID) && !s.partyMatches(req.VRQ, contest.ContestID) {
            return nil, fmt.Errorf("party mismatch for %s", contest.ContestID)
        }

        // 3. Contest is open.
        if !s.isContestOpen(contest.ContestID) {
            return nil, ErrContestNotOpen
        }

        // 4. Mark nullifier consumed.
        s.nullifierStore[contest.ContestID][contest.Nullifier] = true

        // 5. Emit ballot.issued event.
        event := EventTransaction{
            SubjectID:   req.VRQ,
            SubjectType: "QUID",
            EventType:   "ballot.issued",
            Payload: map[string]interface{}{
                "contestId":  contest.ContestID,
                "nullifier":  contest.Nullifier,
                "timestamp":  time.Now().Unix(),
            },
        }
        s.submitEvent(event)

        // 6. Blind-sign the commitment.
        blindedSig := rsa.SignPKCS1v15(nil, &s.blindSigPrivateKey, crypto.SHA256, contest.Commitment)
        response.BlindSignatures = append(response.BlindSignatures, ContestSig{
            ContestID:       contest.ContestID,
            BlindSignature:  blindedSig,
        })
    }
    return response, nil
}
```

The nullifier store is in-memory per issuance epoch; persisted
out to snapshots for audit purposes. It's small (one entry per
voter per contest → ~hundreds of thousands of entries per
county-wide election).

## Scale estimates

Mid-size US county election:
- 200,000 registered voters
- 100,000 actual turnout (50%)
- 20 contests per ballot
- 50 precincts

Workload:
- Registration edges: 200K (one-time, during registration period)
- Poll-book lookups: 100K (one per voter check-in)
- Ballot.issued events: 100K × 20 = 2M (one per voter per contest)
- Vote trust edges: 100K × ~10 average-contests-voted = 1M
- Gossip propagation: spread across 50 precinct nodes + state + observers

Quidnug's consortium-scale target handles this comfortably.
Peak load during polls-close hour: ~20K BQs / 20K vote-sets
submitted simultaneously. Multiple Ballot Issuance Service
instances at the authority node handle parallelism.

## Next

- [`implementation.md`](implementation.md) — concrete API code
  per role
- [`threat-model.md`](threat-model.md) — attacks + mitigations
