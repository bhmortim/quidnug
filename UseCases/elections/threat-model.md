# Threat Model: Elections on Quidnug

Elections are an unusually adversarial domain: nation-state-level
adversaries, insider threats at every layer, voter coercion,
social engineering at massive scale, and legal/reputation
consequences for any detected flaw. This document is
correspondingly detailed.

## Assets, ordered by criticality

1. **Election integrity** — the certified result must reflect
   the votes actually cast.
2. **Ballot secrecy** — no party should learn how any specific
   voter voted.
3. **Voter eligibility enforcement** — only eligible voters
   cast valid votes; no double voting.
4. **Public verifiability** — any citizen should be able to
   independently confirm the result.
5. **Voter registration integrity** — the voter roll must
   reflect eligible citizens, not attacker-controlled quids.
6. **Voter's private keys** — VRQ + BQ private keys represent
   eligibility and cast ballots.
7. **Authority's signing keys** — the issuance key, tally key,
   and guardian keys of the election authority.
8. **Paper ballot chain of custody** — the physical backup.

## Attacker inventory

| Attacker                       | Capability                                              | Primary goal                          |
|--------------------------------|---------------------------------------------------------|---------------------------------------|
| Nation-state                   | Comprehensive: network, infrastructure, personnel        | Change election outcome               |
| Insider: election admin        | Authority signing key access                            | Silent manipulation                   |
| Insider: IT operator           | Node configuration, patches, logs                       | Introduce backdoors                   |
| Insider: poll worker           | Polling-place tablet access                             | Limited-scope manipulation            |
| Voter-buyer                    | Cash, influence                                          | Coerce / buy votes                    |
| Impersonation fraudster        | Stolen ID or VRQ private key                            | Cast unauthorized ballots             |
| Voting machine vendor          | Firmware / software supply chain                        | Systematic manipulation               |
| Partisan activist              | Legitimate voter + social coordination                  | Sybil registration, coordinated fraud |
| Media / investigative outlet   | Public-observer access                                   | Find real flaws (good actor, mostly)  |
| DDoS attacker                  | Bandwidth, botnets                                       | Disrupt polling or tally              |
| Physical attacker              | In-person access                                         | Damage equipment, ballots              |

## Threats and mitigations

### Category A: Election outcome manipulation

#### A1. Authority compromise — silent vote modification

**Attack.** Attacker has compromised the election authority's
signing key and uses it to retroactively forge trust edges (fake
votes) or re-sign the tally with altered numbers.

**Mitigations.**

- **Append-only chain.** Trust edges and events are written to
  the Quidnug blockchain. Attacker can't "modify" existing
  edges — they can only add new ones. Observers see additions.
- **Guardian-based authority recovery** (QDP-0002) — authority's
  own key is rotatable via multi-party quorum. A compromised key
  is invalidated via `AnchorInvalidation`; subsequent signatures
  at the compromised epoch are rejected.
- **Independent tally by observers.** Observer nodes (media,
  parties) run their own tally queries. An attacker adding fake
  edges has to add them to every observer's chain — requiring
  compromising the gossip network, which requires
  orders-of-magnitude more resources.
- **Paper-ballot cross-verification.** Any discrepancy between
  paper and digital is flagged. Paper wins. An attacker adding
  fake digital votes has to also stuff matching paper ballots
  into physical boxes — physical access required.
- **Merkle root snapshots.** At polls-close time, a snapshot of
  the voter roll and ballot issuance is published. Later
  manipulation is detectable by comparing.

**Residual risk.** A coordinated attacker with authority-key
compromise + network-level control + physical ballot-stuffing
could manipulate an election. This is a "sovereign-scale"
attack requiring massive resources. Quidnug doesn't make this
easier; it provides more evidence paths than current systems.

#### A2. Insider adds forged registrations

**Attack.** Corrupt insider at the authority adds fake trust
edges for non-existent voters, allowing fake ballots later.

**Mitigations.**

- **Public registration count.** Anyone can count registered
  voters per precinct. Unusual growth (e.g., Precinct 42 gains
  5000 voters in a week) is visible.
- **Registration domain rate-limits.** Fork-block (QDP-0009) can
  establish per-week / per-month registration caps per
  jurisdiction. Attempts to exceed require explicit fork-block
  signoff.
- **Cross-reference with off-chain voter file.** State SOS's
  voter file comes from DMV + Social Security death index + etc.
  Discrepancies between on-chain count and SOS file flagged.
- **Periodic audit events.** Scheduled `audit.registration-check`
  events from independent observers comparing on-chain vs. SOS.

**Residual risk.** Insider + compromised oversight could sneak in
forgeries. Normal audit processes (already required by state
law) apply.

#### A3. Vote-changing attack after the polls close

**Attack.** After polls close, attacker (with authority access)
modifies trust edges to change votes.

**Mitigations.**

- **Trust edges are append-only** — attacker can only add new
  edges, not modify. A new edge from the same BQ would show up
  as a duplicate in the tally → flagged.
- **Per-BQ anchor nonce** ensures only one valid first-vote from
  a given BQ; subsequent trust edges with the same nonce are
  rejected by the ledger.
- **Paper ballot cross-check** catches any anomaly.
- **Independent observer tallies** lock in the result within
  minutes of polls closing.

#### A4. Tally algorithm manipulation

**Attack.** The election authority publishes a "tally" that
doesn't correctly reflect the chain state (e.g., counting edges
wrong).

**Mitigations.**

- Tally algorithm is **publicly specified** in contest
  attributes. Any observer can re-run it.
- Open-source tally tools make the logic auditable.
- Authority's published tally is a **signed event** — if it
  differs from independent computation, the challenge is
  specific and on the record.

### Category B: Ballot secrecy attacks

#### B1. Authority learns who voted for whom

**Attack.** Corrupt authority operator tries to correlate BQs
back to VRQs to identify voters.

**Mitigations.**

- **Blind signature** during ballot issuance — authority never
  sees the unblinded BQ ID. This is a cryptographic guarantee:
  the correlation is mathematically unrecoverable without the
  voter's blinding factor (held only on voter's device).
- Even authority-level access to the Issuance Service's logs
  doesn't reveal BQ IDs.

**Residual risk.** The blind signature's security relies on the
cryptographic hardness assumption (RSA + secure blinding). A
flaw in the blind-signature implementation would be catastrophic.
Mitigated by using well-audited implementations and by
multiple-BQ-per-voter design (breaking any one BQ's anonymity
only reveals one contest, not the whole ballot).

#### B2. Traffic analysis to correlate check-in with ballot cast

**Attack.** Observer of network traffic correlates "voter X
checked in at 10:15" with "BQ ABC appeared at 10:17."

**Mitigations.**

- **Mix-network batching** — ballot issuance events are
  processed in batches, not in real-time order. Multiple voters'
  issuance requests are shuffled before publication.
- **Delay randomization** — BQ publication to the chain is
  randomly delayed within a short window.
- **Tor-style anonymity for BQ submissions** (optional) — voter
  submits votes through an anonymizing relay.
- **Polling-place processing** — all voters at precinct 42 check
  in publicly but their BQs are processed together in a batch;
  observer sees many BQs appear "at once" without per-voter
  timing.

**Residual risk.** A very-well-resourced observer (e.g., ISP-
level traffic inspection) with network position could in theory
correlate. Mitigation: defense-in-depth with mix networks + Tor.

#### B3. Coercion via BQ private key

**Attack.** Coercer demands voter's BQ private key to verify
they voted "correctly."

**Mitigations (layered).**

- **Legal** — criminal penalties for vote buying.
- **Decoy BQs** — voter's device can generate N plausible BQs;
  voter shows any to a coercer. Only the voter knows which was
  actually cast.
- **Revocable ballots (where legal)** — voter may re-cast up to
  deadline; final vote counts. Coercer can verify an initial
  vote but voter can change later.
- **Physical private voting booth** — coercer can't accompany
  voter into booth.

**Residual risk.** Perfect coercion resistance is an open
research problem. Current protocol gives better defense-in-depth
than paper-based voting.

### Category C: Voter impersonation

#### C1. Stolen VRQ private key

**Attack.** Attacker steals a voter's VRQ private key and
attempts to vote as them.

**Mitigations.**

- **Physical check-in at polling place** — in-person voters
  must show physical ID matching the VRQ's registered voter.
  An attacker with VRQ key but not the ID can't check in.
- **Online voting (where enabled)** requires additional
  authentication (biometric, SMS-based second factor) —
  application-layer but standard.
- **Voter alert** — if voter notices their checked-in event
  when they haven't voted, they can file a dispute immediately.
- **Guardian recovery of VRQ** — voter can rotate their VRQ via
  guardians, invalidating the stolen key.

**Residual risk.** Stolen key + stolen ID = impersonation
possible. Same threat level as current ID-based voting.

#### C2. Registration of non-existent voters

**Attack.** Attacker creates fake quids and registers them.

**Mitigations.**

- Registration requires in-person or online ID verification.
  Authority's registrar is the check.
- Duplicate detection via off-chain voter file.
- Public registration count — unusual growth is flagged.
- **Monotonic nonces** on registration edges prevent simple
  replay.

### Category D: Double voting

#### D1. Voter attempts to vote at two polling places

**Attack.** Voter checks in at precinct 42, then drives to
precinct 43.

**Mitigation.** Push gossip (QDP-0005) propagates the
`voter.checked-in` event from 42 to every other precinct within
seconds. At 43, the lookup finds the prior check-in → reject.

**Residual risk.** Narrow window during network partition. If
precincts are offline-isolated for some time, a voter could
theoretically double-check-in. Mitigation: precinct nodes refuse
check-in if they haven't seen a heartbeat from the authority in
> 1 minute.

#### D2. Voter requests multiple ballots for same contest

**Attack.** Voter tries to trick the Issuance Service into
signing multiple BQs for the same contest.

**Mitigation.** **Nullifier** — derived from voter's VRQ +
contest ID. Same voter + same contest → same nullifier. Second
request rejected.

#### D3. Authority issues duplicate ballots

**Attack.** Corrupt issuance operator manually creates extra BQ
identity transactions for a compromised voter, inflating their
vote count.

**Mitigation.** Each BQ identity transaction has an authority
signature. The authority's signing key is monitored; unusual
signing patterns visible in push-gossip traffic. Observer nodes
can count the per-VRQ ballot issuances (via public
`ballot.issued` events). A VRQ with ballot.issued events
exceeding the contest count is anomalous.

### Category E: Infrastructure attacks

#### E1. DDoS on authority node on election day

**Attack.** Flood the authority's issuance service to prevent
voters from getting ballots.

**Mitigations.**

- Standard DDoS mitigations (CloudFlare-style).
- Issuance service horizontally scaled across precincts —
  attack on one precinct doesn't affect others.
- Paper ballot fallback — if issuance service is down, voters
  cast paper ballots directly; these are later reconciled.

#### E2. Network partition between precincts

**Attack.** Attacker cuts network links between precincts.

**Mitigations.**

- Push gossip resumes when network heals.
- Paper ballot fallback.
- Precinct-local tally possible in isolation; state-level
  aggregate later.

#### E3. Voting booth software compromise

**Attack.** Supply-chain attack on the open-source voting booth
application.

**Mitigations.**

- Open-source + reproducible builds — anyone can verify the
  running binary.
- Multiple independent implementations encouraged.
- **Paper ballot cross-check** is the backstop. Compromised
  booth software produces votes; paper ballot's human-readable
  version ground-truths.

### Category F: Denial of service via legal process

#### F1. Authority compelled to hand over keys

**Attack.** State actor compels election authority to hand
over signing keys.

**Mitigations.**

- **Multi-party guardian set** — no single person has the keys.
- **HSM-protected** — keys can't be "handed over" in practice
  (HSMs only sign, don't export).
- **Guardian-set rotation** under legal duress — new guardians
  replace any compelled ones.

**Residual risk.** Sufficiently broad legal compulsion defeats
distributed trust. No protocol helps here; this is a
jurisdictional problem.

## Attack scenarios — end-to-end

### Scenario 1: "Attacker wants to add 50,000 fake votes for Candidate X"

Required actions:
1. Register 50,000 fake VRQs (requires defeating identity
   verification 50,000 times — detectable).
2. Check-in each (requires either physical presence at polling
   places, or mail-in — each has its own verification).
3. Issue 50,000 ballots (nullifiers per contest).
4. Cast 50,000 votes.
5. Print 50,000 paper ballots and stuff them into ballot boxes
   (physical access to every precinct).
6. Ensure observer nodes see the same chain (network-level
   compromise of gossip).
7. Ensure state SOS doesn't compare to their voter file.
8. Ensure the risk-limiting audit picks a sample that doesn't
   catch the forgeries.

Defeating all 8 is a massive operation. Any single defense
working catches the attack.

### Scenario 2: "Attacker wants to know how a specific voter voted"

Required actions:
1. Compromise the voter's VRQ private key.
2. Compromise the blind-signature cryptographic secret (the
   voter's blinding factor, on their device).
3. OR: compromise the Issuance Service's logs during the blind-
   signature phase before the commitment is unblinded.

Step 2 is cryptographically hard; step 3 is only possible during
a short window and would be visible in the logs the authority
is supposed to retain.

### Scenario 3: "Attacker wants to suppress a specific voter"

Required actions:
1. Prevent the voter from registering (difficult — they can
   register via alternate channels).
2. OR: cause the registration trust edge to be invalid (requires
   authority key compromise).
3. OR: physically prevent voting (in-person intimidation).

On-chain suppression is hard because any modification to a
voter's registration is an append-only event — revocation
leaves a trail.

## Monitoring

Operator dashboards and alerts:

| Metric                                              | Alert condition              |
|-----------------------------------------------------|------------------------------|
| Registrations per hour                              | > baseline × 3               |
| Check-ins per precinct per minute                   | > precinct capacity          |
| Ballot.issued events per minute                     | > baseline × 3               |
| Nullifier-duplicate rejections                      | > 0.1% of issuances          |
| Vote trust-edge signature failures                  | > 0 (should be 0)            |
| Tally discrepancies between independent nodes       | any                          |
| Paper-digital cross-verify discrepancies            | > sampling tolerance         |
| Authority signing key epoch                         | change during election day   |
| `quidnug_gossip_rate_limited_total{producer=auth}`  | > 0                          |
| `quidnug_probe_failure_total`                       | > baseline                   |
| Guardian-resignation events on authority            | > 0 during election          |

## Incident response

Playbooks (excerpts):

1. **Authority signing key compromise detected.**
   - Invalidate affected epoch immediately (`AnchorInvalidation`).
   - Initiate emergency guardian recovery.
   - Pause new ballot issuance at all precinct nodes.
   - Communicate with bipartisan oversight board.
   - Continue accepting paper ballots; digital rejoins after
     rotation complete.

2. **Unusual registration spike.**
   - Observer flags via monitoring.
   - Temporarily suspend auto-approval of online registrations
     pending investigation.
   - Cross-check with state voter file.
   - Public notice if fraud confirmed.

3. **Paper-digital mismatch beyond statistical tolerance.**
   - Full hand-count of affected precinct.
   - Forensic review of voting-booth devices (open-source, so
     binary verification possible).
   - Recount via paper.
   - Certification delayed per statutory process.

## Comparison with traditional election attack surfaces

| Attack surface              | Traditional                                 | Quidnug                                      |
|-----------------------------|---------------------------------------------|----------------------------------------------|
| Tally manipulation          | Central tabulator                           | Independently verifiable by observers        |
| Voter roll manipulation     | Central DB                                  | Public chain, anyone counts                  |
| Ballot stuffing             | Paper ballot-box access                     | Paper + digital cross-verify                 |
| Impersonation               | ID document + polls                         | Same + cryptographic VRQ                     |
| Coercion                    | Booth privacy + laws                        | Same + decoy BQs                             |
| Tech supply chain           | Vendor firmware                             | Open-source + paper backup                   |
| Insider admin               | DB access                                   | Multi-party guardian quorum                  |
| Post-certification challenge| Lawsuits                                    | Chain replay + paper recount                 |

## Not defended against (explicit limits)

1. **Sovereign-scale adversary** with nation-state resources
   targeting multiple layers simultaneously.
2. **Compromise of the blind-signature cryptographic primitive**
   (an implementation flaw would be catastrophic; mitigated by
   using well-audited libraries).
3. **Out-of-protocol vote buying** that doesn't require
   cryptographic proof (e.g., handing voters cash based on
   trust).
4. **Registration-eligibility fraud at the DMV** or other
   identity-issuance upstream sources.
5. **Coordinated mass voter suppression** via real-world
   intimidation (a legal / policing matter).
6. **Perfect receipt-freeness** — the protocol offers layered
   defenses but no cryptographic guarantee absent full
   homomorphic tallying (future work).

## References

- [QDP-0001 Nonce Ledger](../../docs/design/0001-global-nonce-ledger.md)
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
- [QDP-0008 K-of-K Bootstrap](../../docs/design/0008-kofk-bootstrap.md)
- [QDP-0009 Fork-Block Trigger](../../docs/design/0009-fork-block-trigger.md)
- Academic references: Chaum's blind signatures; Benaloh's
  end-to-end verifiable voting; Neff's shuffles; Helios voting
  system; pollbook-integrity papers.
