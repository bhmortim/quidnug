# Implementation: Elections on Quidnug

Concrete API flows for each participant role. Code examples in
Go and shell; real implementations would layer in HSM integration
and UI.

## 0. Configuration for election nodes

```bash
# Election Authority node
ENABLE_NONCE_LEDGER=true
ENABLE_PUSH_GOSSIP=true          # real-time propagation
ENABLE_LAZY_EPOCH_PROBE=true     # cross-jurisdiction safety
ENABLE_KOFK_BOOTSTRAP=true       # observer onboarding
SUPPORTED_DOMAINS=elections.*
NODE_AUTH_SECRET=<32-byte hex>
REQUIRE_NODE_AUTH=true

# Precinct node — same flags; different SUPPORTED_DOMAINS if restricted
SUPPORTED_DOMAINS=elections.williamson-county-tx.2026-nov.poll-book.precinct-042

# Observer/read-only node — accepts gossip but doesn't produce
# transactions in the election domains
```

## 1. Authority: pre-election setup

```bash
# 1a. Create the authority quid
curl -X POST $AUTHORITY_NODE/api/identities -d '{
  "quidId":"election-authority-williamson-2026-nov",
  "name":"Williamson County 2026 November General Election",
  "creator":"election-authority-williamson-2026-nov",
  "updateNonce":1,
  "homeDomain":"elections.williamson-county-tx.2026-nov",
  "attributes":{
    "jurisdiction":"williamson-county-tx",
    "cycle":"2026-nov-general",
    "pollsOpen":"2026-11-03T07:00:00-06:00",
    "pollsClose":"2026-11-03T19:00:00-06:00",
    "certificationDue":"2026-12-03T23:59:59-06:00",
    "precinctCount":50
  }
}'

# 1b. Install guardian set for the authority
curl -X POST $AUTHORITY_NODE/api/v2/guardian/set-update -d '{
  "subjectQuid":"election-authority-williamson-2026-nov",
  "newSet":{
    "guardians":[
      {"quid":"chief-election-admin-quid","weight":1,"epoch":0},
      {"quid":"bipartisan-board-d-quid","weight":1,"epoch":0},
      {"quid":"bipartisan-board-r-quid","weight":1,"epoch":0},
      {"quid":"bipartisan-board-indep-quid","weight":1,"epoch":0},
      {"quid":"state-sos-tx-quid","weight":2,"epoch":0},
      {"quid":"observer-lwv-williamson-quid","weight":1,"epoch":0}
    ],
    "threshold":4,
    "recoveryDelay":259200000000000,   /* 72 hours */
    "requireGuardianRotation":true
  },
  "anchorNonce":1,
  "validFrom":<now>,
  "primarySignature":{...},
  "newGuardianConsents":[ /* each guardian signs */ ]
}'

# 1c. Create contest quids
for contest in us-senate governor proposition-5 proposition-6 sheriff; do
  curl -X POST $AUTHORITY_NODE/api/identities -d '{
    "quidId":"contest-williamson-2026-'$contest'",
    "name":"'$contest' contest",
    "creator":"election-authority-williamson-2026-nov",
    "updateNonce":1,
    "homeDomain":"elections.williamson-county-tx.2026-nov.contests.'$contest'",
    "attributes":{
      "contest":"'$contest'",
      "votingMethod":"plurality",
      "startTime":"2026-11-03T07:00:00-06:00",
      "endTime":"2026-11-03T19:00:00-06:00",
      "eligiblePrecincts":["001","002","003","...","050"]
    }
  }'
done

# 1d. Candidates file their candidacy
# (Each candidate does this from their own quid)
# Example for Jane Smith for US Senate
curl -X POST $AUTHORITY_NODE/api/identities -d '{
  "quidId":"candidate-jane-smith-2026-senate",
  "name":"Jane Smith for Senate",
  "creator":"candidate-jane-smith-2026-senate",
  "updateNonce":1,
  "attributes":{
    "party":"democratic",
    "office":"US Senate",
    "bioURL":"https://jane-smith.example/bio",
    "filingStatementHash":"<sha256>"
  }
}'

# Authority endorses ballot placement
curl -X POST $AUTHORITY_NODE/api/trust -d '{
  "truster":"election-authority-williamson-2026-nov",
  "trustee":"candidate-jane-smith-2026-senate",
  "trustLevel":1.0,
  "domain":"elections.williamson-county-tx.2026-nov.candidates",
  "nonce":1,
  "description":"Qualified for ballot — US Senate — Williamson County 2026",
  "attributes":{
    "contestRef":"contest-williamson-2026-us-senate"
  }
}'
```

## 2. Voter: register (bring-your-own-quid)

### 2a. Voter generates their VRQ locally

```bash
# On voter's device (phone app, hardware token, home computer)
# using a voter-facing CLI or app:

$ quidnug-voter init
Generated new keypair.
Your Voter Registration Quid (VRQ): voter-a1b2c3d4e5f67890
Public key: 0x04abcd...
Private key saved to: ~/.quidnug-voter/vrq.key (keep secure!)
```

### 2b. Voter registers in person / online

The registrar's system (at DMV / county office / online portal):

```go
package registrar

import (
    "context"
    "time"
)

func (r *RegistrarService) Register(ctx context.Context, req RegistrationRequest) error {
    // 1. Verify identity off-chain
    record, err := r.verifyIdentity(req.IDDocumentType, req.IDDocumentNumber)
    if err != nil {
        return err
    }

    // 2. Check not already registered
    if r.isAlreadyRegistered(record.SSN_hash) {
        return ErrAlreadyRegistered
    }

    // 3. Determine precinct from address
    precinct := r.precinctForAddress(record.Address)

    // 4. Verify voter's quid (signature challenge)
    challenge := randomBytes(32)
    if !req.VRQ.VerifySignature(challenge, req.ChallengeResponse) {
        return ErrBadVRQSignature
    }

    // 5. Store the VRQ ↔ off-chain-identity link
    r.db.Insert(record.SSN_hash, req.VRQ.ID, precinct)

    // 6. Emit registration trust edge on the authority's chain
    edge := TrustTransaction{
        Type:        "TRUST",
        Truster:     r.authorityQuid,
        Trustee:     req.VRQ.ID,
        TrustLevel:  1.0,
        Domain:      fmt.Sprintf("elections.%s.%s.registration", r.jurisdiction, r.cycle),
        Nonce:       r.nextRegistrationNonce(),
        ValidUntil:  r.nextElectionCutoff(),
        Attributes: map[string]interface{}{
            "precinct":              precinct,
            "registeredParty":       record.RegisteredParty,
            "registrationMethod":    req.Method,
            "registrationTimestamp": time.Now().Unix(),
        },
    }
    return r.submitTrust(ctx, edge)
}
```

### 2c. Voter verifies their registration

```bash
# From anywhere, any Quidnug node
$ curl "$ANY_NODE/api/v1/trust/edges?\
  truster=election-authority-williamson-2026-nov&\
  trustee=voter-a1b2c3d4e5f67890&\
  domain=elections.williamson-county-tx.2026-nov.registration"

{
  "edges": [{
    "truster": "election-authority-williamson-2026-nov",
    "trustee": "voter-a1b2c3d4e5f67890",
    "trustLevel": 1.0,
    "domain": "elections.williamson-county-tx.2026-nov.registration",
    "attributes": {
      "precinct": "042",
      "registeredParty": "democratic",
      "registrationMethod": "dmv",
      "registrationTimestamp": 1713400000
    },
    "validUntil": 1767225599,
    "nonce": 47283
  }]
}

# Voter sees: registered ✓
```

## 3. Poll worker: check in a voter on election day

```go
package pollworker

func (p *PollWorker) CheckInVoter(ctx context.Context, voterID string) (*CheckInResult, error) {
    // 1. Query the precinct node for the registration edge
    edges, err := p.client.GetTrustEdges(ctx, GetEdgesFilter{
        Truster: p.authorityQuid,
        Trustee: voterID,
        Domain:  fmt.Sprintf("elections.%s.%s.registration", p.jurisdiction, p.cycle),
    })
    if err != nil || len(edges) == 0 {
        return nil, ErrNotRegistered
    }

    edge := edges[0]
    if edge.TrustLevel < 1.0 {
        return nil, ErrRegistrationInvalid
    }
    if edge.Attributes["precinct"] != p.precinct {
        return nil, ErrWrongPrecinct
    }

    // 2. Check for prior voter.voted event
    events, err := p.client.GetSubjectEvents(ctx, voterID, "QUID")
    if err != nil { return nil, err }

    for _, ev := range events {
        if ev.EventType == "voter.voted" &&
           ev.Payload["electionId"] == p.electionID {
            return nil, ErrAlreadyVoted
        }
    }

    // 3. Emit voter.checked-in event
    event := EventTransaction{
        SubjectID:   voterID,
        SubjectType: "QUID",
        EventType:   "voter.checked-in",
        Payload: map[string]interface{}{
            "electionId":   p.electionID,
            "precinct":     p.precinct,
            "checkedInAt":  time.Now().Unix(),
        },
        Creator:   p.pollWorkerQuid,
        Signature: p.sign(/*...*/),
    }
    if err := p.client.SubmitEvent(ctx, event); err != nil {
        return nil, err
    }

    return &CheckInResult{
        Party: edge.Attributes["registeredParty"].(string),
        Precinct: p.precinct,
    }, nil
}
```

## 4. Voter's device: ballot issuance (blind signature)

```go
package voter

import (
    "crypto/ecdsa"
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "math/big"
)

// Minimal client-side blind-signature implementation (RSA-based)
type BlindSigner struct {
    authorityRSAPublicKey *rsa.PublicKey
}

// 1. Generate fresh BQ
func (v *Voter) GenerateBallotQuid() (*BallotQuid, error) {
    priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil { return nil, err }

    pubBytes := elliptic.Marshal(priv.PublicKey.Curve, priv.PublicKey.X, priv.PublicKey.Y)
    quidID := fmt.Sprintf("ballot-%s", hex.EncodeToString(sha256sum(pubBytes)[:8]))

    return &BallotQuid{
        QuidID:     quidID,
        PublicKey:  pubBytes,
        PrivateKey: priv,
    }, nil
}

// 2. Blind the BQ ID
func (v *Voter) BlindBQID(bqID string) (*BlindedCommitment, error) {
    // Hash the BQ ID.
    hashed := sha256.Sum256([]byte(bqID))
    m := new(big.Int).SetBytes(hashed[:])

    // Random blinding factor r, gcd(r, N) = 1.
    N := v.authorityRSAPublicKey.N
    var r *big.Int
    for {
        r, _ = rand.Int(rand.Reader, N)
        if new(big.Int).GCD(nil, nil, r, N).Cmp(big.NewInt(1)) == 0 {
            break
        }
    }

    // Blinded commitment: m' = m * r^e mod N
    e := big.NewInt(int64(v.authorityRSAPublicKey.E))
    rToE := new(big.Int).Exp(r, e, N)
    commitment := new(big.Int).Mul(m, rToE)
    commitment.Mod(commitment, N)

    return &BlindedCommitment{
        BQID:       bqID,
        Commitment: commitment.Bytes(),
        Blinding:   r.Bytes(),
    }, nil
}

// 3. Submit to authority's issuance service
func (v *Voter) SubmitBallotRequest(ctx context.Context, req IssuanceRequest) (*IssuanceResponse, error) {
    // Sign with VRQ's private key
    sig := v.signWithVRQ(req)
    return v.authorityClient.IssueBallot(ctx, req, sig)
}

// 4. Unblind the authority's response
func (v *Voter) UnblindSignature(blinded *big.Int, blindingFactor *big.Int) *big.Int {
    N := v.authorityRSAPublicKey.N
    rInv := new(big.Int).ModInverse(blindingFactor, N)
    unblinded := new(big.Int).Mul(blinded, rInv)
    unblinded.Mod(unblinded, N)
    return unblinded
}

// 5. Compose the BQ identity transaction with unblinded sig
func (v *Voter) BuildBallotIdentity(bq *BallotQuid, unblindedSig []byte, electionID, contestID string) *IdentityTransaction {
    return &IdentityTransaction{
        QuidID:     bq.QuidID,
        PublicKey:  bq.PublicKey,
        Creator:    v.authorityQuid,
        UpdateNonce: 1,
        Attributes: map[string]interface{}{
            "electionId":      electionID,
            "contestId":       contestID,
            "issuanceEpoch":   0,
            "issuanceMethod":  "in-person",
        },
        AuthoritySignature: unblindedSig,  // RSA-verifiable against authority key
    }
}
```

**Full voting-booth flow:**

```go
func (v *VotingBooth) ConductVoting(ctx context.Context, voterVRQ string, selections VoterSelections) error {
    // 1. Generate fresh BQs per contest
    bqPerContest := map[string]*BallotQuid{}
    for _, contest := range v.election.Contests {
        bq, _ := v.voter.GenerateBallotQuid()
        bqPerContest[contest.ID] = bq
    }

    // 2. Build nullifiers per contest
    nullifiers := map[string][]byte{}
    for _, contest := range v.election.Contests {
        nullifiers[contest.ID] = v.computeNullifier(voterVRQ, contest.ID)
    }

    // 3. Blind BQ IDs
    commitments := map[string]*BlindedCommitment{}
    for contestID, bq := range bqPerContest {
        c, _ := v.voter.BlindBQID(bq.QuidID)
        commitments[contestID] = c
    }

    // 4. Request ballot issuance
    req := IssuanceRequest{
        VRQ: voterVRQ,
        ElectionID: v.election.ID,
        Contests: buildContestReqs(commitments, nullifiers),
    }
    resp, err := v.voter.SubmitBallotRequest(ctx, req)
    if err != nil { return err }

    // 5. Unblind signatures and build BQ identity transactions
    for _, contestSig := range resp.BlindSignatures {
        bq := bqPerContest[contestSig.ContestID]
        unblinded := v.voter.UnblindSignature(
            new(big.Int).SetBytes(contestSig.BlindSignature),
            new(big.Int).SetBytes(commitments[contestSig.ContestID].Blinding),
        )
        identityTx := v.voter.BuildBallotIdentity(bq, unblinded.Bytes(), v.election.ID, contestSig.ContestID)

        // Submit BQ identity
        if err := v.submitIdentity(ctx, identityTx); err != nil {
            return err
        }
    }

    // 6. Submit vote trust edges
    for contestID, candidate := range selections {
        bq := bqPerContest[contestID]
        vote := TrustTransaction{
            Truster: bq.QuidID,
            Trustee: candidate,
            TrustLevel: 1.0,
            Domain: fmt.Sprintf("elections.%s.%s.contests.%s",
                v.election.Jurisdiction, v.election.Cycle, contestID),
            Nonce: 1,
        }
        vote.Signature = signWithBQKey(vote, bq.PrivateKey)
        if err := v.submitTrust(ctx, vote); err != nil { return err }
    }

    // 7. Print paper ballot
    v.printPaperBallot(bqPerContest, selections)

    // 8. Emit voter.voted event
    event := EventTransaction{
        SubjectID:   voterVRQ,
        SubjectType: "QUID",
        EventType:   "voter.voted",
        Payload: map[string]interface{}{
            "electionId":     v.election.ID,
            "contestCount":   len(selections),
            "completedAt":    time.Now().Unix(),
        },
    }
    return v.submitEvent(ctx, event)
}
```

## 5. Anyone: recount

```bash
# Run from any Quidnug node with election domain visibility
$ curl "$ANY_NODE/api/v1/trust/edges?\
  domain=elections.williamson-county-tx.2026-nov.contests.us-senate"

# The node returns the full list of trust edges. Client-side tally:

$ curl "..." | jq '.edges | group_by(.trustee) |
     map({candidate: .[0].trustee, votes: length}) |
     sort_by(.votes) | reverse'

[
  {"candidate": "candidate-bob-jones-2026-senate", "votes": 51039},
  {"candidate": "candidate-jane-smith-2026-senate", "votes": 47282},
  {"candidate": "candidate-carol-li-2026-senate", "votes": 21476}
]
```

For a more robust tally (verifying signatures, applying the
contest's voting method), use a Go-based tally tool:

```bash
$ quidnug-tally \
    --node $ANY_NODE \
    --contest contest-williamson-2026-us-senate \
    --election-authority election-authority-williamson-2026-nov

Verifying BQ signatures...    ✓ 119,797 valid
Verifying no-double-vote...   ✓ (no duplicate nullifiers)
Applying plurality tally...

Results:
  Bob Jones:    51,039 (42.6%)   WINNER
  Jane Smith:   47,282 (39.5%)
  Carol Li:     21,476 (17.9%)
  Total:       119,797

Tally hash: sha256:abc123...  (for independent comparison)
```

Independent observers on different nodes should compute the same
tally hash. If they disagree, discrepancy is loud.

## 6. Voter: verify their own vote

```bash
$ quidnug-voter verify --election williamson-2026-nov
  My VRQ: voter-a1b2c3d4e5f67890

  I have records of 5 BQs from this election:

  ┌────────────────────────────────────────────────────────┐
  │ Contest: US Senate                                      │
  │ My BQ:   ballot-Xz7mN2pQ8rK4vL9t                        │
  │ I cast:  candidate-jane-smith                           │
  │                                                          │
  │ Verifying on chain...                                   │
  │   ✓ BQ identity tx: found, authority-signed             │
  │   ✓ Trust edge: found, BQ → candidate-jane-smith        │
  │   ✓ Edge signature: valid against BQ pubkey             │
  │   ✓ Edge counted in tally                                │
  └────────────────────────────────────────────────────────┘

  [Similar for each of my other BQs]

  All my votes were cast and counted. ✓
```

## 7. Paper-ballot audit

```go
func (a *Auditor) CrossVerify(ctx context.Context, paperBallots []PaperBallot) (*AuditResult, error) {
    result := &AuditResult{
        TotalPaper:  len(paperBallots),
        TotalDigital: 0,
        Matches:      0,
        Discrepancies: []Discrepancy{},
    }

    // 1. For each paper ballot, look up its BQs on the chain
    for _, paper := range paperBallots {
        for _, bqID := range paper.BQIDs {
            identity, err := a.client.GetIdentity(ctx, bqID)
            if err != nil {
                result.Discrepancies = append(result.Discrepancies, Discrepancy{
                    BallotID: paper.ID,
                    Reason:   "digital BQ not found for paper ballot",
                })
                continue
            }

            // Verify BQ is authority-signed for this election
            if !a.verifyAuthoritySig(identity) {
                result.Discrepancies = append(result.Discrepancies, ...)
                continue
            }

            // Cross-check the vote
            digitalEdges := a.getEdgesFor(bqID)
            paperVotes := paper.VotesForBQ(bqID)
            if !matchesVotes(digitalEdges, paperVotes) {
                result.Discrepancies = append(result.Discrepancies, Discrepancy{
                    BQID: bqID,
                    PaperVote: paperVotes,
                    DigitalVote: digitalEdges,
                    Reason: "mismatch between paper and digital",
                })
                continue
            }
            result.Matches++
        }
    }

    // 2. For each digital vote, ensure there's a paper ballot
    digitalOnly := a.findDigitalBQsWithoutPaper(paperBallots)
    for _, bq := range digitalOnly {
        result.Discrepancies = append(result.Discrepancies, Discrepancy{
            BQID:   bq,
            Reason: "digital vote with no paper ballot",
        })
    }

    result.TotalDigital = a.countAllBQs()

    // 3. Emit audit event
    a.submitEvent(ctx, EventTransaction{
        SubjectID:   a.electionQuid,
        SubjectType: "QUID",
        EventType:   "audit.cross-verify-completed",
        Payload: map[string]interface{}{
            "paperCount":       result.TotalPaper,
            "digitalCount":     result.TotalDigital,
            "matches":          result.Matches,
            "discrepancyCount": len(result.Discrepancies),
        },
    })

    return result, nil
}
```

## 8. Testing

```go
func TestElection_FullFlow(t *testing.T) {
    // Setup: authority, contests, candidates
    // Voter: generates VRQ, registers
    // Poll worker: checks in voter
    // Voting booth: issues blind-signed BQs, cast votes
    // Tally: correct results
    // Voter: verifies own vote
    // Observer: independent tally matches
}

func TestElection_DoubleVoteBlocked(t *testing.T) {
    // Voter requests ballot for same contest twice
    // Second request rejected (nullifier duplicate)
}

func TestElection_WrongPartyPrimaryBlocked(t *testing.T) {
    // Democratic voter requests Republican primary ballot
    // Issuance service rejects
}

func TestElection_InvalidBQSignature(t *testing.T) {
    // Vote trust edge signed with wrong key
    // Tally excludes the edge
}

func TestElection_PaperDigitalCrossVerify(t *testing.T) {
    // Set up mock paper ballots matching digital
    // Auditor cross-verifies; all match
    // Flip one digital record; auditor flags discrepancy
}

func TestElection_RecountMatches(t *testing.T) {
    // Two independent observer nodes run tally
    // Results byte-identical
}

func TestElection_AuthorityKeyCompromise(t *testing.T) {
    // Authority key rotated via guardian recovery mid-election
    // Pre-rotation ballots still valid
    // Post-rotation ballots use new key for issuance
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- [`../credential-verification-network/`](../credential-verification-network/)
  — similar authority + registered-entity structure
- [`../healthcare-consent-management/`](../healthcare-consent-management/)
  — similar bring-your-own-identity + guardian override
