# security-reviewer smoke test

This is a deliberate no-op test PR opened to exercise the
`security-reviewer` agent in `quidnug-ops`. It does not touch
any cryptographic code, signing primitives, or trust paths. It
adds only this single markdown file under `docs/`.

The PR's branch and title contain the keyword `security`, which
matches the dispatcher's `_pr_touches_security` heuristic in
`agents/webhook_receiver/worker.py`. That triggers the GitHub
webhook to fan out to two agents:

- `github-sentinel` — classifies the PR (every PR touches this)
- `security-reviewer` — reviews for security issues (only when
  the title/branch matches the security signal list)

After this PR opens, both agents should fire within a few
seconds. `security-reviewer` is expected to comment that the
PR is benign (a docs-only addition with no code change). The
intent is to verify the wiring end-to-end:

1. GitHub webhook → cloudflared → webhook-receiver:8787
2. webhook-receiver inserts row into `webhook_jobs`
3. webhook-worker reads it, calls `_dispatch_targets`
4. mailbox rows posted for `github-sentinel` + `security-reviewer`
5. both agents wake via NOTIFY and process the PR

Close + delete this PR once both agents have responded — the
content here doesn't belong in the repo long-term.
