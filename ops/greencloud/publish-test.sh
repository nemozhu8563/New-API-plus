#!/usr/bin/env bash
set -euo pipefail

# Build and publish one immutable linux/amd64 image to the GreenCloud test app.
# Required: docker, ssh, scp. Override paths/host through environment variables.

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
STAMP=${RELEASE_STAMP:-$(date -u +%Y%m%dT%H%M%SZ)}
COMMIT=${RELEASE_COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD)}
IMAGE=${TEST_IMAGE:-new-api:new-api-test-${STAMP}-${COMMIT}}
HOST=${GREENCLOUD_HOST:-173.249.203.66}
SSH_KEY=${GREENCLOUD_SSH_KEY:-$HOME/.ssh/greencloud_core_rsa2048}
REMOTE_DIR=${GREENCLOUD_TEST_DIR:-/srv/new-api-test}
SERVICE=${GREENCLOUD_TEST_SERVICE:-new-api-test}
REMOTE_USER=${GREENCLOUD_USER:-root}
WORK_DIR=${RELEASE_WORK_DIR:-/tmp/new-api-test-release-${STAMP}-${COMMIT}}

mkdir -p "$WORK_DIR"
docker build --platform linux/amd64 -t "$IMAGE" "$ROOT_DIR"
docker save "$IMAGE" | gzip > "$WORK_DIR/image.tar.gz"
SHA256=$(shasum -a 256 "$WORK_DIR/image.tar.gz" | awk '{print $1}')
printf '%s  %s\n' "$SHA256" "$(basename "$WORK_DIR/image.tar.gz")" > "$WORK_DIR/SHA256SUM"

REMOTE="$REMOTE_USER@$HOST"
ssh -i "$SSH_KEY" -o IdentitiesOnly=yes "$REMOTE" "mkdir -p '$REMOTE_DIR/import/$STAMP-$COMMIT'"
scp -i "$SSH_KEY" -o IdentitiesOnly=yes "$WORK_DIR/image.tar.gz" "$WORK_DIR/SHA256SUM" "$REMOTE:$REMOTE_DIR/import/$STAMP-$COMMIT/"
REMOTE_SHA=$(ssh -i "$SSH_KEY" -o IdentitiesOnly=yes "$REMOTE" "shasum -a 256 '$REMOTE_DIR/import/$STAMP-$COMMIT/image.tar.gz' | awk '{print \$1}'")
test "$REMOTE_SHA" = "$SHA256"

ssh -i "$SSH_KEY" -o IdentitiesOnly=yes "$REMOTE" bash -s -- "$REMOTE_DIR" "$SERVICE" "$IMAGE" "$STAMP-$COMMIT" <<'REMOTE_SCRIPT'
set -euo pipefail
remote_dir=$1
service=$2
image=$3
release_id=$4
cd "$remote_dir"
cp -p compose.yaml "backups/compose.before-$release_id.yaml"
docker load < "import/$release_id/image.tar.gz"
export NEW_API_IMAGE="$image"
docker compose -f compose.yaml config >/dev/null
docker compose -f compose.yaml up -d --no-deps --no-build --pull never --force-recreate "$service"
REMOTE_SCRIPT

ssh -i "$SSH_KEY" -o IdentitiesOnly=yes "$REMOTE" "docker inspect --format '{{.State.Status}} {{.State.Health.Status}} {{.RestartCount}}' '$SERVICE'"
curl -fsS "${TEST_BASE_URL:-https://test.tryvalo.com}/api/status" >/dev/null
printf 'published %s (%s) to %s\n' "$IMAGE" "$SHA256" "$HOST"
