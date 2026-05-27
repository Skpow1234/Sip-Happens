#!/usr/bin/env bash
# Refresh infra/docker/base-images.env with current linux/amd64 digests for build bases.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/infra/docker/base-images.env"

go_digest="$(docker buildx imagetools inspect golang:1.25 --format '{{json .}}' \
  | python -c "import json,sys; m=json.load(sys.stdin)['manifest']['manifests']; print(next(x['digest'] for x in m if x.get('platform',{}).get('architecture')=='amd64' and x.get('platform',{}).get('os')=='linux'))")"
distro_digest="$(docker buildx imagetools inspect gcr.io/distroless/static-debian12 --format '{{json .}}' \
  | python -c "import json,sys; m=json.load(sys.stdin)['manifest']['manifests']; print(next(x['digest'] for x in m if x.get('platform',{}).get('architecture')=='amd64' and x.get('platform',{}).get('os')=='linux'))")"

cat >"$OUT" <<EOF
# Pinned base image digests (linux/amd64). Refresh via scripts/docker-refresh-digests.sh
# See docs/ops/docker-image-security.md
GO_BUILD_IMAGE=golang:1.25@${go_digest}
RUNTIME_IMAGE=gcr.io/distroless/static-debian12@${distro_digest}
EOF

echo "Updated $OUT"
grep . "$OUT"
