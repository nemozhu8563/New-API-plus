#!/usr/bin/env bash
set -euo pipefail

required_commands=(cloudflared docker age psql redis-cli ufw)
for command in "${required_commands[@]}"; do
  command -v "$command" >/dev/null
done

systemctl is-active --quiet docker
systemctl is-active --quiet postgresql
getent passwd cliproxy >/dev/null
test "$(stat -c '%a:%U:%G' /srv/new-api)" = '750:root:root'
test "$(stat -c '%a:%U:%G' /opt/cliproxy)" = '700:cliproxy:cliproxy'
ufw status | grep -q '^Status: active'
ss -ltnH | awk '{print $4}' | grep -Eq '^127\.0\.0\.1:5432$'
if ss -ltnH | awk '{print $4}' | grep -Eq '(^|:)(3000|3001|6379|8317)$'; then
  echo 'an application/data port is listening before the deployment is approved' >&2
  exit 1
fi

echo 'GreenCloud host preflight passed'
