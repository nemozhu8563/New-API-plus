#!/usr/bin/env bash
set -euo pipefail

# Build and publish one immutable linux/amd64 image to the GreenCloud production app.
# This script changes only the production new-api application container. PostgreSQL,
# Redis, keeper, DNS, Caddy, and application secrets are left untouched.
# Required: docker, ssh, scp, curl, shasum. Set CONFIRM_PRODUCTION_RELEASE=YES
# explicitly before running it.

if [[ ${CONFIRM_PRODUCTION_RELEASE:-} != YES ]]; then
  echo 'refusing production release: set CONFIRM_PRODUCTION_RELEASE=YES' >&2
  exit 2
fi

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"

for command in docker ssh scp curl shasum awk sed; do
  command -v "$command" >/dev/null
done

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo 'refusing production release: working tree has uncommitted changes' >&2
  exit 2
fi

STAMP=${RELEASE_STAMP:-$(date -u +%Y%m%dT%H%M%SZ)}
COMMIT=${RELEASE_COMMIT:-$(git rev-parse --short=12 HEAD)}
IMAGE=${PRODUCTION_IMAGE:-new-api:new-api-release-${STAMP}-${COMMIT}}
HOST=${GREENCLOUD_HOST:-173.249.203.66}
SSH_KEY=${GREENCLOUD_SSH_KEY:-$HOME/.ssh/greencloud_core_rsa2048}
REMOTE_DIR=${GREENCLOUD_PRODUCTION_DIR:-/srv/new-api}
SERVICE=${GREENCLOUD_PRODUCTION_SERVICE:-new-api}
REMOTE_USER=${GREENCLOUD_USER:-root}
WORK_DIR=${RELEASE_WORK_DIR:-/tmp/new-api-production-release-${STAMP}-${COMMIT}}
RELEASE_ID=${STAMP}-${COMMIT}
PRODUCTION_BASE_URLS=${PRODUCTION_BASE_URLS:-'https://api.tryvalo.com https://new.tryvalo.com'}

mkdir -p "$WORK_DIR"
docker build --platform linux/amd64 -t "$IMAGE" "$ROOT_DIR"
docker save "$IMAGE" | gzip > "$WORK_DIR/image.tar.gz"
SHA256=$(shasum -a 256 "$WORK_DIR/image.tar.gz" | awk '{print $1}')
printf '%s  %s\n' "$SHA256" "$(basename "$WORK_DIR/image.tar.gz")" > "$WORK_DIR/SHA256SUM"

REMOTE="$REMOTE_USER@$HOST"
ssh -i "$SSH_KEY" -o IdentitiesOnly=yes "$REMOTE" "mkdir -p '$REMOTE_DIR/import/$RELEASE_ID' '$REMOTE_DIR/backups/$RELEASE_ID'"
scp -i "$SSH_KEY" -o IdentitiesOnly=yes \
  "$WORK_DIR/image.tar.gz" "$WORK_DIR/SHA256SUM" \
  "$REMOTE:$REMOTE_DIR/import/$RELEASE_ID/"
REMOTE_SHA=$(ssh -i "$SSH_KEY" -o IdentitiesOnly=yes "$REMOTE" \
  "shasum -a 256 '$REMOTE_DIR/import/$RELEASE_ID/image.tar.gz' | awk '{print \$1}'")
test "$REMOTE_SHA" = "$SHA256"

ssh -i "$SSH_KEY" -o IdentitiesOnly=yes "$REMOTE" bash -s -- \
  "$REMOTE_DIR" "$SERVICE" "$IMAGE" "$RELEASE_ID" <<'REMOTE_SCRIPT'
set -euo pipefail
remote_dir=$1
service=$2
image=$3
release_id=$4
cd "$remote_dir"

cp -p env/images.env "backups/$release_id/images.env.before"
cp -p compose.yaml "backups/$release_id/compose.yaml.before"
docker load < "import/$release_id/image.tar.gz"

image_lines=$(awk -F= '$1 == "NEW_API_IMAGE" { count += 1 } END { print count + 0 }' env/images.env)
test "$image_lines" = 1
sed -i "s|^NEW_API_IMAGE=.*$|NEW_API_IMAGE=$image|" env/images.env

docker compose --env-file env/images.env -f compose.yaml config -q
docker compose --env-file env/images.env -f compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate "$service"

for attempt in $(seq 1 36); do
  state=$(docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}} {{.RestartCount}}' "$service")
  if [[ $state == 'running healthy 0' ]]; then
    curl -fsS http://127.0.0.1:3000/api/status | grep -q '"success":true'
    exit 0
  fi
  sleep 5
done

docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}} {{.RestartCount}}' "$service" >&2
exit 1
REMOTE_SCRIPT

read -r -a PRODUCTION_URLS <<< "$PRODUCTION_BASE_URLS"
for base_url in "${PRODUCTION_URLS[@]}"; do
  curl -fsS "$base_url/api/status" | grep -q '"success":true'
done

printf 'published %s (%s) to %s\n' "$IMAGE" "$SHA256" "$HOST"
