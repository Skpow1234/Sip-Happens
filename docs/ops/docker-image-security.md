# Docker image security

Application Dockerfiles pin base images **by digest** (Go builder + distroless/static runtime). Dependabot opens PRs when those digests move.

## Practice

- Prefer digest pins over floating tags (`golang:1.25`, `latest`).
- Rebuild on Dependabot Docker PRs; run CI `docker` job before merge.
- Runtime images are distroless — no shell; debug with ephemeral sidecars or `docker compose run` debug variants if you add them.
- Host only needs Docker; do not install Go on production VMs for runtime.

CVE remediation for a pinned digest: bump the digest in the service Dockerfile, rebuild, redeploy. Track advisories via `govulncheck` in CI (production entrypoints must pass; full-tree scan is advisory).
