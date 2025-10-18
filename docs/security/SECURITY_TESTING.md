# Security & Penetration Testing Playbook

This document outlines the minimum security validation activities for OpenMCP Chain
services. Teams should embed these checks into their delivery pipelines and use the
penetration testing guidance ahead of major releases.

## 1. Static and Dependency Analysis

1. **Go security linters** – Run `gosec ./...` and address high/critical
   findings. False positives should be documented with a risk justification.
2. **Dependency audit** – Execute `go list -m all | govulncheck` to scan third
   party libraries. Apply patches or workarounds for reported CVEs before
   deployment.
3. **Container image scanning** – Use `trivy fs .` (or an equivalent scanner) to
   review Dockerfiles, base images, and bundled binaries.

## 2. Configuration & Secrets Review

- Ensure `auth.jwt.secret` is provided via environment variables or secret
  managers; never commit plaintext secrets.
- Validate database credentials and OAuth client secrets reside in dedicated
  secret stores (e.g. HashiCorp Vault, AWS Secrets Manager).
- Enforce TLS termination at the edge proxy and disable insecure cipher suites.

## 3. Runtime Hardening Tests

1. **Authentication & authorization**
   - Attempt to call `/api/v1/tasks` without tokens (expect `401`).
   - Replay expired or tampered JWTs (expect `401`).
   - Use accounts lacking `tasks.write` to submit tasks (expect `403`).
2. **Rate limiting & brute-force detection**
   - Perform credential stuffing simulations against `/api/v1/auth/token` and
     confirm lockout/alerting paths.
3. **Input validation**
   - Exercise task submission payloads with oversized `goal` strings, embedded
     SQL/command injection patterns, and invalid UTF-8 to ensure graceful
     rejection.

## 4. Penetration Testing Workflow

1. **Scoping** – Document exposed services, authentication mode (JWT or OAuth),
   and test credentials. Include diagrams of upstream identity providers if
   applicable.
2. **Reconnaissance** – Enumerate open ports, HTTP routes, and third-party
   integrations. Capture TLS certificate chains and cipher configuration.
3. **Exploit attempts**
   - Privilege escalation between roles (`tasks.read` ➜ `tasks.write`).
   - Token forging/signature bypass (check algorithm downgrades and issuer
     validation).
   - SQL injection against the MySQL auth tables (`auth_users`, `auth_roles`,
     `auth_permissions`).
4. **Post-exploitation** – Validate log integrity by reviewing `data/audit.log`
   for traces of the attack. Confirm incident response runbooks include
   revocation of compromised tokens and forced password resets.

## 5. Reporting & Remediation

- Summarise findings with CVSS scores, reproduction steps, and recommended fixes.
- Track remediation work in the project issue tracker. Security fixes should be
  backported to supported release branches.
- Close the loop by re-running the relevant security tests once patches are
  applied.

For questions or to schedule an official penetration test, contact the security
engineering team at `security@openmcp.io`.
