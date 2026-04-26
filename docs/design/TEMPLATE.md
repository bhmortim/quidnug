# QDP-NNNN: Short Title in Title Case

| Field         | Value                                                                |
|---------------|----------------------------------------------------------------------|
| Status        | Draft                                                                |
| Track         | Protocol / Protocol (hard fork) / Ecosystem / Informational / Meta   |
| Author        | The Quidnug Authors                                                  |
| Created       | YYYY-MM-DD                                                           |
| Discussion    | link to GitHub issue or PR                                           |
| Requires      | QDP-XXXX (or -)                                                      |
| Supersedes    | QDP-YYYY (or -)                                                      |
| Superseded-by | -                                                                    |
| Enables       | -                                                                    |
| Activation    | fork block, version, or N/A                                          |

> Delete this blockquote before submitting. Copy this file to
> `NNNN-kebab-slug.md` using the next unused QDP number. See
> [QDP-0000](0000-qdp-process.md) for the lifecycle, status
> values, track taxonomy, and writing conventions. An unset
> metadata field is a single hyphen, not the word "none".

## 1. Summary

Two to four sentences. What problem, what you propose, what an
implementer has to change. A reader should know within thirty
seconds whether this QDP is relevant to them.

## 2. Motivation

The concrete pain today. A failing scenario, a captured attack
transcript, a user story that cannot be served by the current
protocol, a missing primitive other QDPs keep tripping over.
Not "this would be nice" but "here is what breaks without it."

## 3. Goals and non-goals

**Goals:** bulleted, each one a verifiable outcome.

- Goal one.
- Goal two.

**Non-goals:** what this QDP explicitly does not try to solve,
so reviewers do not argue about the wrong thing.

- Non-goal one.
- Non-goal two.

## 4. Background

The state of the code today. Quote exact structs, functions,
file paths, and line numbers. Reviewers should be able to
verify claims about current behavior without leaving this
document.

## 5. Design

The proposal. Specific enough that two independent
implementations produce interoperable wire formats and
validation behavior.

### 5.1 Wire format changes

### 5.2 State changes

### 5.3 Algorithmic changes

### 5.4 Validation rules

## 6. Security considerations

Attacks this enables, attacks it closes, residual risk that
survives the change. Include the threat model you assumed.

## 7. Privacy considerations

What new data becomes observable. Who can observe it.
Retention, decay, and deletion semantics. If nothing changes
here, say so explicitly.

## 8. Backward compatibility

- Breaks for existing nodes: yes or no; list specific behaviors.
- Breaks for SDKs: yes or no; list specific affected packages.
- Activation strategy: fork block, opt-in flag, or version gate.

## 9. Test vectors

Concrete inputs and expected outputs so implementations can
be cross-checked. At minimum three cases: happy path, edge
case, adversarial. Inline them; do not link to an external
gist that can move.

```json
{
  "input":    "...",
  "expected": "..."
}
```

## 10. Reference implementation

Link to PR or branch once code lands. Empty during Draft.

## 11. Open questions

Things the author has not decided yet. This section shrinks
as the QDP moves through review. Empty is fine at Last Call.

## 12. Changelog

- YYYY-MM-DD Draft published
- YYYY-MM-DD Last Call
- YYYY-MM-DD Accepted
- YYYY-MM-DD Phase 1 landed
- YYYY-MM-DD Landed
