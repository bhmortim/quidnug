# Elections launch checklist

> Sequential T-90 through T+30 go-live steps for running a
> Quidnug-based election. Work top to bottom; each item
> gates the next. Designed for a county-scale election;
> scale items up/down for different deployments.
>
> Parallels
> [`deploy/public-network/reviews-launch-checklist.md`](../../deploy/public-network/reviews-launch-checklist.md)
> but for elections. Read
> [`README.md`](README.md), [`integration.md`](integration.md),
> and [`operations.md`](operations.md) first — this checklist
> assumes you've done the design work.

---

## T-180 to T-90 days: Foundation

Before you're actually running against a real election,
you need the infrastructure standing.

### Legal + governance foundation

- [ ] Legal counsel reviews + approves the overall design.
      Specifically: does running elections on Quidnug
      satisfy state election law? (Varies by state; some
      require specific vendor certifications, some allow
      custom systems with public audit provisions.)
- [ ] If required: engage an independent election-software
      auditor for pre-certification.
- [ ] Bipartisan oversight board formed + briefed on the
      protocol architecture.
- [ ] Governance structure defined + documented (see
      [`integration.md`](integration.md) §2.2):
      - Governor quorum (who, weights, threshold)
      - Consortium membership (who runs validators)
      - Emergency-notice clause
      - Guardian quorums per governor
- [ ] Legal agreements signed with observer organizations
      (LWV, R party, D party, SoS office) re: key custody,
      emergency procedures, liability.
- [ ] Public governance documentation published at
      `elections.<county>.gov/governance`.

### Hardware + infrastructure

- [ ] Consortium node hardware procured:
      - Authority-primary (in gov data center 1)
      - Authority-backup (in gov data center 2)
      - 3-4 observer nodes (one per observer org's site)
- [ ] Cache tier hardware procured (10+ nodes in
      geographically-diverse regions).
- [ ] Archive node hardware procured (2+ nodes).
- [ ] Precinct device fleet ordered (iPad Pro or equivalent,
      ~1 per precinct + 10% spares).
- [ ] Cloudflare account set up with:
      - Domain registered (`elections.<county>.gov`)
      - Tunnel account for home/remote access
      - Worker deployed for `api.elections.<county>.gov`
      - Monitoring configured

### Software + keys

- [ ] Quidnug node binary built from a specific tagged
      release (e.g. `v2.4.0`). SHA-256 hashes published.
- [ ] All consortium node keys generated OFFLINE on an
      air-gapped machine. Paper backups printed.
- [ ] Guardian quorums installed for each governor key
      (following QDP-0002).
- [ ] Observer keys generated + distributed to observer
      organizations.
- [ ] Precinct device keys generated + preloaded into
      secure enclaves.
- [ ] CLI tooling (`quidnug-cli elections`) tested on
      staging.

### Staging environment

- [ ] Staging cluster set up mirroring production (different
      domain: `elections-staging.<county>.gov`).
- [ ] 100 fake voters registered in staging.
- [ ] Fake election run end-to-end in staging (registration
      → ballot-issuance → voting → tally → recount).
- [ ] Staging election results match expected (seeded) data.

---

## T-90 days: Domain + consortium setup

### Publish the network descriptor

- [ ] Publish `https://elections.<county>.gov/.well-known/quidnug-network.json`
      per QDP-0014 §7. Signed by the operator key. Includes:
      - Operator quid + pubkey
      - API gateway URL
      - Seed nodes + capabilities
      - Governors + quorum
      - Domain tree
- [ ] Publish backup mirror of the well-known file at
      `https://quidnug.com/.well-known/elections/<county>.json`
      (federated publication for resilience).

### Register the election domain tree

- [ ] Register the root election domain:
      ```bash
      quidnug-cli domain register \
          --name "elections.<county>.2026-nov" \
          --validators "<consortium-quids>" \
          --governors "<governor-quids>" \
          --governance-quorum 0.7 \
          --threshold 0.67 \
          --key <authority-primary-key>
      ```
- [ ] Register sub-domains (registration, poll-book,
      ballot-issuance, contests, tally, audit):
      ```bash
      for child in registration poll-book ballot-issuance \
                   ballot-issuance.precinct-001 \
                   [... per precinct ...] \
                   contests.us-senate contests.governor \
                   [... per contest ...] \
                   tally audit; do
          quidnug-cli domain register \
              --name "elections.<county>.2026-nov.$child" \
              --parent-delegation-mode inherit \
              --key <authority-primary-key>
      done
      ```
- [ ] Verify all domains show up in
      `api.elections.<county>.gov/api/domains`.

### Stand up the consortium

- [ ] All 5 consortium members deploy Quidnug node.
- [ ] Each publishes their own `NODE_ADVERTISEMENT` per
      QDP-0014:
      ```bash
      quidnug-cli node advertise \
          --operator-quid <county-authority-quid> \
          --endpoints "https://node<i>.elections.<county>.gov:443,http/2,data-center-1,1,100" \
          --capabilities "validator,archive" \
          --supported-domains "elections.<county>.2026-nov.*" \
          --expires-in "7d" \
          --sign-with <node-i-key>
      ```
- [ ] Each consortium member's operator quid publishes the
      TRUST attestation edge:
      ```bash
      quidnug-cli trust grant \
          --truster <county-authority-quid> \
          --trustee <node-i-quid> \
          --domain "operators.elections.<county>.2026-nov" \
          --level 1.0 \
          --sign-with <county-authority-key>
      ```
- [ ] Run pairwise peering between consortium members
      (everyone trusts everyone at 0.95 in the
      `peering.*` domain).
- [ ] Verify consortium is producing blocks (each member
      should see blocks from all others tier to `Trusted`).

---

## T-60 days: Test + validate

### Software audit

- [ ] External security audit firm completes review of
      node binary, precinct device software, client app.
- [ ] Audit report published publicly.
- [ ] All audit findings addressed or explicitly deferred
      with rationale.

### Dry run #1 (1,000 fake voters)

- [ ] Reset staging to clean state.
- [ ] Register 1,000 fake voters across 10 test precincts.
- [ ] Simulate ballot issuance + voting for all 1,000.
- [ ] Run tally. Results should match expected (seed data).
- [ ] Run observer recount from a cold laptop; should match.
- [ ] Run a "suspected-fraud" scenario: introduce 10
      deliberately-invalid registrations; observer flags them;
      authority revokes.
- [ ] Run a "node-failure" scenario: kill one consortium
      member mid-voting; system continues with 4-of-4.
- [ ] Document all issues found in a `dry-run-1-postmortem.md`.

### Legal final sign-off

- [ ] State SoS office confirms compliance.
- [ ] Court injunction checks (if applicable).
- [ ] Ballot design approved.
- [ ] Candidate list finalized.

---

## T-30 days: Polish + train

### Dry run #2 (10,000 fake voters)

- [ ] Larger-scale test with 10,000 fake voters across all
      real precincts.
- [ ] Observer orgs run independent recounts.
- [ ] Press invited to observe (not the full election, but
      the dry-run to build public confidence).
- [ ] Issues documented + fixed.

### Poll-worker training

- [ ] Poll workers complete certified training on:
      - Precinct device operation
      - Paper-ballot parity procedures
      - Suspected-fraud reporting flow
      - Emergency procedures (network outage, device failure)
- [ ] Test voters cast ballots with each poll worker
      observing.
- [ ] Poll workers sign certification events (published
      on-chain for audit).

### Voter education

- [ ] Public education campaign explaining:
      - How to generate a VRQ (on phone / kiosk / paper)
      - How to verify your own registration
      - How the ballot is anonymous but your vote is verifiable
      - How to recount on your own laptop
- [ ] Accessibility: multilingual + screen-reader content.
- [ ] FAQ addressing common concerns (privacy, verifiability).

### Infrastructure freeze

- [ ] Code freeze: no deploys to production after T-14 days
      except for critical security fixes.
- [ ] Final hardware count verified; precinct devices
      distributed to precincts.
- [ ] All consortium members confirm they are on-call for
      election day.

---

## T-14 days: Early voting opens

- [ ] Publish `EARLY_VOTING_STARTED` event.
- [ ] Dedicated early-voting locations run the same flow as
      election-day precincts; all their events flow through
      the same domain tree.
- [ ] Daily status dashboards showing early-voting turnout.
- [ ] Observer queries running continuously.
- [ ] Incident-report channel open (email, phone, SMS).

### Daily rhythm during early voting

Each day from T-14 to T-1:

- [ ] Morning: confirm all consortium nodes healthy.
- [ ] Midday: check turnout rates vs expected.
- [ ] End-of-day: sum of early-voting events committed,
      ready for tally later.
- [ ] Any incidents: publish `INCIDENT_<type>` event + fix.

---

## T-3 days: Final verification

- [ ] All poll workers confirmed available + trained.
- [ ] All precinct devices verified at each precinct site
      (power on, connectivity, software version).
- [ ] Paper-ballot supplies distributed.
- [ ] Ballot boxes sealed + tamper-evident seals verified.
- [ ] Election-day incident-response team on-call:
      - Chief election official
      - Oversight board members (rotating)
      - IT team lead + two engineers
      - Legal counsel
      - Communications lead

---

## T-1 day: Pre-flight

- [ ] All consortium members connect + verify blockchain
      head matches across all nodes.
- [ ] Final status check (operations.md §5.1).
- [ ] Publish `ELECTION_DAY_READY` event to audit domain.
- [ ] Team rest; don't schedule last-minute changes.

---

## T-0 (Election Day)

### Polls open (typically 7am local)

- [ ] Each precinct chief judge signs + publishes
      `POLLS_OPENED` event.
- [ ] Precinct devices start accepting check-ins.
- [ ] Incident-response team on-call.
- [ ] Status dashboard publicly visible.

### Throughout the day

- [ ] Monitor turnout per precinct (dashboard).
- [ ] Respond to any reported issues within 15 minutes.
- [ ] Publish occasional `STATUS_UPDATE` events with
      turnout summaries.
- [ ] If a precinct has issues: deploy backup staff,
      switch to paper-only check-in if needed.

### Polls close (typically 7pm local)

- [ ] Each precinct chief judge signs `POLLS_CLOSED` event.
- [ ] Consortium waits 30 minutes for straggler events.
- [ ] Tally computation runs automatically.
- [ ] Preliminary results published within 60 minutes of
      close.
- [ ] Observer orgs run independent recounts; confirm
      agreement.

---

## T+1 day: Observer review + audit

- [ ] 24-hour observer-review window. Observers publish
      findings on the audit domain.
- [ ] Any discrepancies flagged for paper-ballot comparison.
- [ ] Paper-ballot audit (statistical sample, per state law)
      compared against digital tally.
- [ ] Audit firm cross-checks: every winning contest's
      signature chain is valid, no suppressed or
      inflated votes.

---

## T+2 days: Canvassing + certification

- [ ] County canvassing board meets.
- [ ] Any challenges from candidates / observers heard.
- [ ] Final adjudication on flagged ballots (typically
      paper-ballot cases).
- [ ] Certification event published with full governor
      quorum signatures:
      ```bash
      quidnug-cli elections certify \
          --year 2026-nov \
          --quorum-signatures "<signatures>" \
          --sign-with <authority-key>
      ```
- [ ] Certified results federate to state network via
      `TRUST_IMPORT`.

---

## T+7 days: Post-election report

- [ ] Operational postmortem written and published:
      - Turnout statistics
      - Any incidents + resolutions
      - Performance metrics (response times, uptime)
      - Comparisons with prior elections
- [ ] All code audit findings that were deferred get
      reviewed for future fixes.

---

## T+30 days: Archival + handoff

- [ ] Full chain data archived to long-term storage
      (archive nodes + offline backup).
- [ ] Election authority quid retired; governor keys
      transitioned or stored.
- [ ] Precinct devices collected + wiped (except key
      material is zeroized).
- [ ] Post-election audit report published.
- [ ] Legal + media archives finalized.
- [ ] Begin planning for next election cycle.

---

## Metrics to track

For every election, track these against targets from
[`operations.md`](operations.md) §4:

| Metric | Target | How to measure |
|---|---|---|
| Total registered voters | (set per election) | Count of `VOTER_REGISTERED` events |
| Total voters who voted | (set per election) | Count of unique BQs with cast-vote events |
| Average voter wait time | < 15 minutes | Logged per precinct |
| Precinct device uptime | > 99% | Device-reported health checks |
| Consortium uptime | > 99.9% | Node-reported health + block production rate |
| Response time (poll worker query) | < 500ms p99 | API logs |
| Response time (observer query) | < 2s p99 | API logs |
| Discrepancies flagged | < 0.01% of ballots | Audit reports |
| Post-election audit results match | 100% | Audit summary |
| Observer satisfaction | > 90% | Survey post-election |

---

## What to do if...

### ...you find evidence of fraud during voting

1. Don't stop voting. Publish a `FRAUD_SUSPECTED` event.
2. Route suspicious activity to IR team immediately.
3. If the fraud appears widespread + actionable, consult
   legal about whether to extend hours or contest results.
4. Post-election audit becomes the forum for adjudication.

### ...the consortium becomes partitioned

1. Don't panic. Partitions happen.
2. Consortium members on each side produce blocks
   independently (tiered `Tentative` across the split).
3. When partition heals, the existing tiered-block-
   acceptance machinery converges.
4. No lost votes (they're in local cache on each side and
   sync up on reconnection).

### ...a precinct device is stolen / compromised

1. Precinct switches to paper-only check-in immediately.
2. IR team revokes the stolen device's node key.
3. Any votes cast through that device flagged for paper
   verification.
4. File a police report; file an `INCIDENT_REPORT` event.

### ...the election is contested

1. Contesting candidate submits a `CONTEST_CHALLENGE`
   event with their basis.
2. Canvassing board reviews per state law.
3. Cryptographic recount is always available (anyone's
   laptop can do it).
4. If paper-ballot audit disagrees with digital tally,
   paper wins; investigation into discrepancy.

---

## References

- [`README.md`](README.md) — election semantics.
- [`architecture.md`](architecture.md) — data model + flows.
- [`integration.md`](integration.md) — architectural-pillar
  integration.
- [`operations.md`](operations.md) — deployment + incident
  response.
- [`threat-model.md`](threat-model.md) — attack analysis.
- [`implementation.md`](implementation.md) — concrete code.
- [`deploy/public-network/reviews-launch-checklist.md`](../../deploy/public-network/reviews-launch-checklist.md)
  — template this doc cribs from.
