# Security Policy

## Reporting a Vulnerability

**Please do not open public GitHub issues for security vulnerabilities.**

If you have discovered a security issue in Quidnug, report it privately via
GitHub Security Advisories:

1. Open <https://github.com/bhmortim/quidnug/security/advisories/new>
2. Fill in the details (description, reproduction steps, suggested fix if any).
3. Submit the report.

If you cannot use GitHub Security Advisories, email the maintainers directly
(contact details in the repository's top-level `README.md`).

## What to include

- A clear description of the issue and its impact.
- A minimal reproduction (code, request, configuration).
- The commit hash or release version tested.
- Any suggested remediation or mitigation.

## Our commitments

- We will acknowledge receipt within **72 hours**.
- We will provide an initial assessment within **7 days**.
- We will coordinate disclosure with you; by default, we aim to publish a fix
  and advisory within **90 days** of the initial report. Critical issues may
  be expedited.
- We will credit the reporter in the advisory unless you request anonymity.

## Scope

In scope:

- The Go node implementation in `src/`.
- The JavaScript client in `clients/js/`.
- Official release artifacts (binaries and container images published by this
  project).
- Documentation that could lead a correct reader into an insecure
  configuration.

Out of scope:

- Vulnerabilities in dependencies that are already disclosed upstream — file
  those with the upstream project.
- Issues that require an attacker to already have administrative access to
  the host or the operator's key material.
- Social-engineering or physical-access attacks.
- Denial-of-service that requires traffic volumes well beyond what a single
  node is designed to handle, when the operator has not configured the
  documented rate-limit and body-size controls.

## Supported versions

Until the project issues a tagged 1.0 release, security fixes are applied to
the `main` branch only. After 1.0, the most recent minor release (`N`) and
the previous one (`N-1`) will receive security fixes.

## Hall of thanks

Reporters who follow this policy will be credited in release notes and in
this file once their report is fixed and disclosed.
