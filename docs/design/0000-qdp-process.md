# QDP-0000: The QDP Process

| Field         | Value                                                                     |
|---------------|---------------------------------------------------------------------------|
| Status        | Landed                                                                    |
| Track         | Meta                                                                      |
| Author        | The Quidnug Authors                                                       |
| Created       | 2026-04-23                                                                |
| Discussion    | this file                                                                 |
| Requires      | -                                                                         |
| Supersedes    | -                                                                         |
| Superseded-by | -                                                                         |
| Enables       | every subsequent QDP                                                      |
| Activation    | N/A (this document defines the process that governs its peers)            |

## 1. Summary

Quidnug evolves through Quidnug Design Proposals (QDPs). Each
QDP is a numbered markdown file in `docs/design/` that
specifies a change to the protocol, its wire formats, its
ecosystem, or the process itself. This document defines what
counts as a QDP, how one moves from idea to Landed, which
statuses exist, how tracks are classified, and how numbers are
allocated. Read it before opening the PR for your first QDP.

## 2. What a QDP is

A QDP is the durable decision record for a specific change.
It exists to:

- Force the author to write down the motivation, design, and
  trade-offs before any code lands.
- Give reviewers a stable surface to argue about (the
  document) separate from the implementation.
- Leave a permanent record of how the protocol reached its
  current shape, so future contributors can reconstruct the
  reasoning behind any behavior they see in the code.
- Coordinate implementations across multiple SDK authors and
  node operators on the same interoperable target.

A QDP is worth writing when any of the following is true:

- The change modifies a wire format, a validation rule, a
  consensus parameter, or a transaction type.
- The change is big enough that two reasonable engineers would
  design it differently and you want to settle the choice
  before building.
- The change spans both the protocol and one or more SDKs or
  integrations.
- The change is a policy decision (moderation, data subject
  rights, content takedown) that the community should be able
  to audit later.
- The change modifies this process itself.

## 3. What a QDP is not

QDPs are not changelogs, bug-fix writeups, user docs, or
marketing material. They do not replace code review: the PR
that implements a QDP is reviewed the same way any other PR
is. They do not replace the README, the architecture doc, or
the use-case companion docs.

If a change is a straightforward bug fix, a refactor with no
externally visible behavior change, a dependency bump, or a
doc edit, it does not need a QDP.

## 4. Lifecycle

```
  Idea
    |   informal discussion, GitHub issue labeled qdp-discuss
    v
  Draft  ---------->  Withdrawn
    |   PR open, iterate
    v
  Last Call  ------>  Rejected
    |   14 calendar days, objections only
    v
  Accepted  ------->  Superseded by QDP-YYYY
    |   merged, numbered, not yet live
    v
  Phase N landed
    |   partial implementation; author lists remaining phases
    v
  Landed  --------->  Deprecated
      live in production at block and version recorded in the
      QDP's backward-compatibility section and changelog
```

Transitions:

- **Idea to Draft.** Open a PR adding
  `docs/design/NNNN-slug.md` with Status=Draft. Reserve NNNN
  by picking the next unused integer (see section 7).
- **Draft to Last Call.** Author declares the design stable.
  Maintainers (the operator plus domain governors per
  QDP-0012) post a comment "Last Call on QDP-NNNN closes on
  YYYY-MM-DD". Minimum fourteen calendar days.
- **Last Call to Accepted.** No unaddressed objections at
  close of Last Call. Merge the PR with Status=Accepted and
  an explicit activation plan.
- **Accepted to Phase N landed or Landed.** Author or
  contributor ships the implementation in one or more PRs,
  then updates the QDP's Status field and changelog to
  reflect actual deployment state.
- **Withdrawn or Rejected.** Close the PR without merging. If
  the QDP was reserved with a number, move the file under
  `docs/design/withdrawn/` or `docs/design/rejected/` so
  future readers can still find it by number. Never reuse a
  number.
- **Superseded.** When a later QDP replaces this one, that
  later QDP lists this number under `Supersedes`. A follow-up
  PR updates this QDP's `Superseded-by` field and sets its
  Status to Superseded.
- **Deprecated.** For capabilities being phased out. Set the
  status and note the deprecation window and replacement QDP
  (if any) in the changelog.

## 5. Status values

| Status           | Meaning                                                                   |
|------------------|---------------------------------------------------------------------------|
| Draft            | Under active discussion. Shape may still change materially.               |
| Last Call        | Stable proposal in its final review window.                               |
| Accepted         | Decision made, implementation not yet shipped.                            |
| Phase N landed   | Partial implementation. Remaining phases listed in the QDP's body.        |
| Landed           | Fully shipped and live in production.                                     |
| Withdrawn        | Author retracted before acceptance.                                       |
| Rejected         | Decision made against at Last Call.                                       |
| Superseded       | Replaced by a later QDP; see Superseded-by.                               |
| Deprecated       | Still live but being phased out; successor (if any) named in changelog.   |
| Informational    | Process, roadmap, or position paper. Never activates; never rejects.      |

Annotations such as "Draft, design only, no code landed" are
acceptable shorthand when a QDP is explicitly documentation-
first. They collapse to Draft for index purposes.

## 6. Tracks

Track classifies what layer the QDP touches. Known values:

| Track                                         | Meaning                                                                                             |
|-----------------------------------------------|-----------------------------------------------------------------------------------------------------|
| Protocol                                      | Wire protocol, validation rules, or state machine. Default for most QDPs.                           |
| Protocol (hard fork)                          | Consensus-breaking change. Requires fork-block activation per QDP-0009.                             |
| Protocol (soft fork)                          | Backward compatible addition. Old nodes keep working; new nodes gain new capability.                |
| Protocol (auxiliary crypto)                   | Adds new cryptographic primitives (blind signatures, group keys) used by other QDPs.                |
| Protocol (cryptographic payload layer)        | Payload-layer encryption or encoding changes.                                                       |
| Protocol + architecture                       | Spans wire protocol and overall system architecture (federation, sharding).                         |
| Protocol + ops                                | Protocol change with operator-facing impact (monitoring, abuse prevention, audit).                  |
| Protocol + ops + legal                        | Policy-driven protocol changes (moderation, data subject rights).                                   |
| Protocol + ecosystem                          | Protocol change that touches SDKs, integrations, or client behavior.                                |
| Protocol + infrastructure                     | Protocol change with node-infrastructure implications (bootstrap, discovery).                       |
| Ecosystem                                     | SDKs, integrations, client libraries, docs. No wire change.                                         |
| Informational                                 | Roadmaps, position papers, architectural overviews.                                                 |
| Meta                                          | About the QDP process itself. This document is Track=Meta.                                          |

New tracks may be added by proposing them in a Meta-track QDP.
Do not stretch one of the above to cover a genuinely new
category.

## 7. Numbering and namespaces

### QDP-NNNN

Core protocol and ecosystem proposals. Monotonic integer
allocation. Check the highest existing number in
`docs/design/` and take the next one. Reservation happens at
PR creation; two authors racing for the same number should
coordinate in the PR and the second one to land takes the
next integer.

As of 2026-04-23, the next free number is **0025**.

Gaps in the sequence (withdrawn or rejected numbers) are
preserved as files under `docs/design/withdrawn/` or
`docs/design/rejected/` so every integer corresponds to a
real historical decision.

### QRP-NNNN

Quidnug Reviews Protocol. Application-layer spec for the
reviews ecosystem. QRP-0001 is the current active spec. QRPs
live in `examples/reviews-and-comments/` (or a future
`docs/qrp/` directory), not `docs/design/`. QRPs are built
**on** the Quidnug protocol; they are not changes **to** it.
The numbering is separate because the audiences are separate:
protocol implementers vs. reviews consumers.

### Future namespaces

Additional application-layer protocols built on Quidnug may
claim their own prefix. Reserve a new prefix by writing a
Meta-track QDP that defines the namespace, where its files
live, and how numbers are allocated.

## 8. Writing conventions

- Filename: `NNNN-kebab-slug.md` in `docs/design/`.
- Title: `# QDP-NNNN: Title in Title Case` as the first line.
- Metadata table immediately after the title, using the fields
  in [TEMPLATE.md](TEMPLATE.md). Fields that do not apply get
  a single hyphen ("-"), not the word "none" or omission.
- Section numbering starts at 1. Summary is always section 1.
- Reference code with file paths and line numbers in the form
  `src/core/types.go:170`. Reviewers should be able to follow
  the reference and see the current state.
- Reference other QDPs by number: `QDP-0012`, not "the domain
  governance proposal".
- Test vectors live in section 9 of the standard template.
  Inline them; do not link to an external gist that can move.
- When a QDP ships in phases, each phase corresponds to a
  merged PR, each PR links back to the QDP, and the QDP's
  changelog records the merge commit.
- Do not use em dashes (U+2014). Prefer commas, colons,
  parentheses, or sentence breaks. The site build strips em
  dashes at sync time; keep the repo consistent.

## 9. Where discussion happens

Two surfaces, both linked from the QDP's `Discussion` field:

1. **GitHub issue** labeled `qdp-discuss` for the open-ended
   "does this problem matter" phase before a draft exists.
2. **GitHub PR** for the line-by-line review of the draft
   itself.

Close the issue when the PR opens; the PR becomes the single
authoritative surface through Last Call. Notes from external
calls (voice, chat, offline review) are summarized into the
PR as comments so the decision record is self-contained.

## 10. Activation for protocol-track QDPs

Accepted is not Landed. A protocol-track QDP becomes binding
at a specific `(block-height, epoch)` pair that governors of
the affected domain co-sign per QDP-0012. The activation plan
lives in section 8 of the QDP (backward compatibility).

For Protocol (hard fork) QDPs: nodes that have not upgraded
by the activation block tier the post-activation chain as
Untrusted until they upgrade. This is the intended behavior
of QDP-0009 (fork-block migration trigger) and is the whole
point of the fork-block mechanism.

For Protocol (soft fork) QDPs: old nodes continue to accept
blocks produced by new-rule nodes. New capability is gated
on a protocol version handshake, not on chain state.

For non-protocol QDPs (Ecosystem, Informational, Meta): there
is no fork block. Landed means the associated PRs merged and
the change is live in whatever SDKs, docs, or process the QDP
affected.

## 11. How this document changes

QDP-0000 can be amended through the same process it governs.
Open a PR modifying this file with Track=Meta and a short
motivation. Meta-track changes go to Last Call for seven days
(half the usual window) because they are procedural, not
technical.

Adding a new status value, a new track, or a new namespace
counts as a Meta-track amendment and must ship as a QDP,
not as a silent edit to this file.

## 12. Changelog

- 2026-04-23 Draft published
- 2026-04-23 Landed (retroactively ratifies the process
  already in use across QDPs 0001 through 0024)
