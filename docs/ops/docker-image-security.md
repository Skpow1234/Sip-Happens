# Docker image security

Application Dockerfiles pin base images **by digest** (Go builder + distroless/static runtime). Dependabot opens PRs when those digests move.

## Practice

- Prefer digest pins over floating tags (`golang:1.25`, `latest`).
- Rebuild on Dependabot Docker PRs; run CI `docker` job before merge.
- Runtime images are distroless — no shell; debug with ephemeral sidecars or `docker compose run` debug variants if you add them.
- Host only needs Docker; do not install Go on production VMs for runtime.

CVE remediation for a pinned digest: bump the digest in the service Dockerfile, rebuild, redeploy. Track advisories via `govulncheck` in CI (production entrypoints must pass; full-tree scan is advisory). The `docker` CI job also runs Trivy CRITICAL/HIGH against freshly built images (`ignore-unfixed: true`).

`core-api` copies only `infra/demo/sip-happens-demo.pub` (seed fingerprint). Demo **private** keys stay out of the image (and are listed in `.dockerignore`) — they are Compose-dev mounts only.

Published images: tag `v*` (or run the **release** workflow) pushes `core-api` and `ssh-gateway` to GHCR. Prefer pulling those digests/tags onto the host instead of building on the production VM when you can.
