# GreenCloud Direct Data Transfer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transfer the final `new-api`, Redis, CPA, and CPA keeper state directly from GCP `sub2api-prod` to GreenCloud without putting large data bytes through the local Codex/gcloud execution channel.

**Architecture:** GCP creates logical database dumps and state archives, then streams them over a normal outbound SSH connection directly to a temporary GreenCloud `migration` account. That account has no shell and accepts only seven fixed receiver commands; a root-owned receiver immediately encrypts each stream with GreenCloud's existing age recipient, writes it under a fixed cutover directory, verifies decryption, and reports hashes only. The Mac transfers only a short-lived SSH private key and small scripts, never a dump, archive, or decrypted secret.

**Tech Stack:** GCP Compute Engine `sub2api-prod` (`stalwart-elixir-490811-q6`, zone `us-west1-b`), GreenCloud `nemo-Phoenix` (`173.249.203.66`), OpenSSH, age, Docker, PostgreSQL custom dumps, Redis RDB, GNU tar, gzip, systemd.

## Global Constraints

- This runbook supersedes every earlier instruction to use `gcloud compute ssh` stdout, `gcloud compute scp`, or the Mac filesystem for a database dump, Redis RDB, CPA archive, or keeper archive. Those paths are allowed only for small key/script files.
- The direct transfer is GCP → GreenCloud. It is not GCP → Mac → GreenCloud, and no `gcloud` command carries the dump bytes.
- Keep the existing GreenCloud age identity at `/etc/new-api-migration/age.key` on GreenCloud only. Do not copy, print, or commit it.
- Keep CPA private. This transfer does not start `cliproxyapi`, `new-api`, `cpacodexkeeper`, or `cloudflared`, and it does not change Cloudflare.
- Do not paste a DSN, API key, OAuth/auth file, tunnel token, Feishu webhook, `SESSION_SECRET`, or any private key into this document, shell history, chat, or Git. Encrypted source archives remain private production data.
- A completed receiver output means only that the encrypted artifact is intact; it is not permission to cut over. The Cloudflare/DNS change remains a separate, explicitly approved maintenance-window action in [the main migration plan](2026-07-10-greencloud-new-api-cpa-cloudflare-migration.md).
- GCP remains the sole production write authority until the final freeze has completed and GreenCloud's restored stack has passed its local acceptance checks. Retain GCP intact for the seven-day rollback period.

---

## Why the prior path fails and what this changes

The truncation is in the local command-execution transit path, not evidence that PostgreSQL, `pg_dump`, or GreenCloud cannot handle the export. A command such as `gcloud compute ssh ... 'pg_dump ...' > local-file` sends the dump through the local runner; the runner can terminate a large stdout stream after a few hundred KiB. `gcloud compute scp` has shown the same limitation for large files.

The commands below first open an ordinary interactive shell on GCP and then execute the stream there. The data path is:

```text
GCP pg_dump / docker cp / tar
  -> GCP send-to-greencloud helper (hashes only are retained on GCP)
  -> SSH encrypted transport over GCP outbound network
  -> GreenCloud forced-command receiver
  -> age-encrypted artifact under /srv/new-api/exports/<cutover-id>/
```

The stream is not resumable by design: it never writes a plaintext dump to disk. If a stream fails, the receiver leaves no completed artifact and the single artifact is re-exported. This is appropriate for the currently measured custom dump size (about 20.7 MB, with about 446 MB logical database content). If a future data set makes a re-export too expensive, create an explicitly approved encrypted-at-source resumable transfer design; do not weaken the forced-command account into a general shell or reuse an incomplete file.

## Transfer manifest

The receiver accepts exactly these commands and filenames. All completed target files are age encrypted. Source and target plaintext SHA-256 hashes are compared without retaining a plaintext copy on either migration directory.

| Receiver command | GreenCloud encrypted artifact | Source data | Source impact |
| --- | --- | --- | --- |
| `receive-new-api-postgres` | `new-api-postgres.dump.age` | Production `new-api` PostgreSQL custom dump | Read-only, consistent snapshot |
| `receive-new-api-redis` | `new-api-redis.rdb.tar.gz.age` | Fresh production Redis `dump.rdb` packaged as a tar stream | `BGSAVE` only; no write outage |
| `receive-new-api-config` | `new-api-config.tar.age` | Production Compose/config inputs needed to reconstruct private target env files | Read-only |
| `receive-cpa-postgres` | `cpa-postgres.dump.age` | CPA's dedicated PostgreSQL database | Read-only rehearsal; stopped CPA for final export |
| `receive-cpa-files` | `cpa-files.tar.gz.age` | `/opt/cliproxy/pgstore`, `config.yaml`, and `.env` | Read-only rehearsal; stopped CPA for final export |
| `receive-cpa-keeper` | `cpa-keeper.tar.gz.age` | `/opt/cpacodexkeeper/runtime` | Read-only rehearsal; stopped keeper for final export |
| `receive-cpa-keeper-config` | `cpa-keeper-config.tar.age` | `/opt/cpacodexkeeper/.env` | Read-only |

`cloudflared` is intentionally absent: create or attach the approved target Tunnel only during the Cloudflare cutover task. Do not copy an old Tunnel token merely because it is available on GCP.

## Task 1: Establish an empty, auditable transfer boundary

**Files:**

- Create on GreenCloud only: `/etc/new-api-migration/cutover-id`
- Create on GreenCloud only: `/srv/new-api/exports/<cutover-id>/`
- Verify only: `/etc/new-api-migration/age.key`
- Do not create: a local-Mac dump, a GCP plaintext dump, or a GreenCloud plaintext staging dump

**Consumes:** The already provisioned GreenCloud host, its age identity, and its closed UFW application/data ports.

**Produces:** One empty, root-owned transfer directory with an explicit identifier, plus proof that old truncated artifacts cannot be mistaken for this cutover.

- [ ] **Step 1: Choose and persist one non-secret cutover ID on GreenCloud.**

  Connect to GreenCloud as root, choose an identifier that is unique for this rehearsal or final freeze, and create an empty directory. The ID contains only letters, digits, dots, underscores, and hyphens.

  ```bash
  export CUTOVER_ID="$(date -u +%Y%m%dT%H%M%SZ)-greencloud"
  case "$CUTOVER_ID" in
    (*[!A-Za-z0-9._-]*|'') exit 1 ;;
  esac
  sudo install -d -m 0700 -o root -g root "/srv/new-api/exports/$CUTOVER_ID"
  printf '%s\n' "$CUTOVER_ID" | sudo tee /etc/new-api-migration/cutover-id >/dev/null
  sudo chmod 0600 /etc/new-api-migration/cutover-id
  sudo stat -c '%a %U:%G %n' "/srv/new-api/exports/$CUTOVER_ID" /etc/new-api-migration/cutover-id
  ```

  Expected: exactly `700 root:root` for the export directory and `600 root:root` for the ID file.

- [ ] **Step 2: Confirm that the age identity is present without displaying it.**

  ```bash
  sudo test -s /etc/new-api-migration/age.key
  sudo test "$(stat -c '%a:%U:%G' /etc/new-api-migration/age.key)" = '600:root:root'
  sudo age-keygen -y /etc/new-api-migration/age.key >/dev/null
  ```

  Expected: all three commands exit zero. The public recipient is derived internally later; no age key material reaches the terminal.

- [ ] **Step 3: Remove incomplete artifacts from the abandoned transfer attempt, only after listing them.**

  On each machine, list candidate files by filename and size first. Do not use a broad recursive deletion.

  ```bash
  sudo find /srv/new-api/exports -maxdepth 2 -type f \( -name '*.partial.*' -o -name '*.dump' -o -name '*.rdb' -o -name '*.tar' -o -name '*.gz' \) -printf '%p %s bytes\n'
  find "$HOME" -maxdepth 3 -type f \( -name '*new-api*.dump*' -o -name '*cliproxy*.tar*' -o -name '*redis*.rdb*' \) -printf '%p %s bytes\n'
  ```

  Delete only files confirmed to be incomplete leftovers from the failed attempt, then rerun the two listings and record that no such file remains. Do not delete the new empty `/srv/new-api/exports/$CUTOVER_ID` directory or any age-encrypted completed artifact.

  Expected: this new cutover directory starts empty, and no plaintext dump/Redis/state archive is retained outside an active controlled restore.

## Task 2: Install the constrained GreenCloud receiver

**Files:**

- Create on GreenCloud only: `/usr/local/libexec/new-api-migration-receive`
- Create on GreenCloud only: `/home/migration/.ssh/authorized_keys`
- Create on GreenCloud only: system account `migration`

**Consumes:** Task 1's cutover ID and GreenCloud age identity.

**Produces:** A non-login account that can write only the seven encrypted artifacts through exact forced commands. It cannot obtain a terminal, forward traffic, or execute an arbitrary program.

- [ ] **Step 1: Install the receiver before authorizing any key.**

  On GreenCloud, install this root-owned script exactly. Its command whitelist and the `cutover-id` file prevent the source from choosing an arbitrary destination or filename.

  ```bash
  sudo install -d -m 0755 -o root -g root /usr/local/libexec
  sudo tee /usr/local/libexec/new-api-migration-receive >/dev/null <<'EOF'
  #!/usr/bin/env bash
  set -euo pipefail

  readonly AGE_IDENTITY=/etc/new-api-migration/age.key
  readonly CUTOVER_ID_FILE=/etc/new-api-migration/cutover-id
  readonly EXPORT_ROOT=/srv/new-api/exports

  cutover_id="$(tr -d '\n' < "$CUTOVER_ID_FILE")"
  case "$cutover_id" in
    (*[!A-Za-z0-9._-]*|'')
      echo 'invalid cutover ID' >&2
      exit 64
      ;;
  esac

  case "${SSH_ORIGINAL_COMMAND:-}" in
    receive-new-api-postgres) artifact='new-api-postgres.dump.age' ;;
    receive-new-api-redis) artifact='new-api-redis.rdb.tar.gz.age' ;;
    receive-new-api-config) artifact='new-api-config.tar.age' ;;
    receive-cpa-postgres) artifact='cpa-postgres.dump.age' ;;
    receive-cpa-files) artifact='cpa-files.tar.gz.age' ;;
    receive-cpa-keeper) artifact='cpa-keeper.tar.gz.age' ;;
    receive-cpa-keeper-config) artifact='cpa-keeper-config.tar.age' ;;
    *)
      echo 'unsupported migration receiver command' >&2
      exit 64
      ;;
  esac

  destination_dir="$EXPORT_ROOT/$cutover_id"
  test -d "$destination_dir"
  test "$(stat -c '%a:%U:%G' "$destination_dir")" = '700:root:root'
  recipient="$(age-keygen -y "$AGE_IDENTITY")"
  stage="$(mktemp "$destination_dir/.${artifact}.partial.XXXXXXXX")"
  trap 'rm -f "$stage"' EXIT HUP INT TERM

  age -r "$recipient" -o "$stage"
  plain_sha256="$(age -d -i "$AGE_IDENTITY" "$stage" | sha256sum | awk '{print $1}')"
  encrypted_sha256="$(sha256sum "$stage" | awk '{print $1}')"
  mv -f -- "$stage" "$destination_dir/$artifact"
  printf '%s  %s\n' "$encrypted_sha256" "$destination_dir/$artifact" > "$destination_dir/$artifact.sha256"
  trap - EXIT HUP INT TERM
  printf 'artifact=%s\nplain_sha256=%s\nencrypted_sha256=%s\n' "$artifact" "$plain_sha256" "$encrypted_sha256"
  EOF
  sudo chown root:root /usr/local/libexec/new-api-migration-receive
  sudo chmod 0755 /usr/local/libexec/new-api-migration-receive
  sudo bash -n /usr/local/libexec/new-api-migration-receive
  ```

  Expected: `bash -n` exits zero. The script reads stdin only, encrypts before persistence, re-decrypts only through `sha256sum`, and atomically publishes a completed artifact only after authentication succeeds.

- [ ] **Step 2: Create the non-login account and its constrained SSH directory.**

  ```bash
  getent passwd migration >/dev/null || sudo useradd --system --create-home --home-dir /home/migration --shell /bin/bash migration
  sudo install -d -m 0700 -o migration -g migration /home/migration/.ssh
  sudo install -m 0600 -o migration -g migration /dev/null /home/migration/.ssh/authorized_keys
  sudo passwd -l migration
  sudo tee /etc/sudoers.d/new-api-migration >/dev/null <<'EOF'
  Defaults:migration env_keep += "SSH_ORIGINAL_COMMAND"
  migration ALL=(root) NOPASSWD: /usr/local/libexec/new-api-migration-receive
  EOF
  sudo chmod 0440 /etc/sudoers.d/new-api-migration
  sudo visudo -cf /etc/sudoers.d/new-api-migration
  sudo stat -c '%a %U:%G %n' /home/migration /home/migration/.ssh /home/migration/.ssh/authorized_keys
  ```

  Expected: the account has no usable SSH shell because the key in Task 3 forces every SSH invocation through one root-owned receiver, and `restrict` prevents a terminal or forwarding. `/bin/bash` is present solely because `sshd` needs an executable shell to launch a forced command. The authorized-key file is empty until Task 3.

- [ ] **Step 3: Record GreenCloud's SSH host fingerprint out of band.**

  ```bash
  sudo ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
  ```

  Record the displayed SHA256 fingerprint in the maintenance record. It will be compared on GCP before the direct stream starts.

## Task 3: Create, distribute, and prove the one-time key

**Files:**

- Create temporarily on the Mac: `$HOME/.ssh/greencloud-migration-<timestamp>` and `.pub`
- Create temporarily on GCP: `/home/zkl/.ssh/greencloud-migration`
- Create on GCP only: `/home/zkl/bin/send-to-greencloud`
- Modify temporarily on GreenCloud: `/home/migration/.ssh/authorized_keys`

**Consumes:** Task 2's receiver and host fingerprint.

**Produces:** One SSH key that can invoke only the receiver's fixed commands, plus a sender that computes a source SHA-256 while it forwards stdin. The key is removed in Task 8.

- [ ] **Step 1: Generate the key locally; it never leaves the Mac except for its private copy on GCP.**

  ```bash
  export MIGRATION_KEY="$HOME/.ssh/greencloud-migration-$(date -u +%Y%m%dT%H%M%SZ)"
  umask 077
  ssh-keygen -t ed25519 -a 64 -f "$MIGRATION_KEY" -N '' -C 'gcp-to-greencloud-one-time-migration'
  test "$(stat -f '%Lp' "$MIGRATION_KEY")" = 600
  ```

  Expected: the private key has mode `0600`; the public key is the only content copied to GreenCloud.

- [ ] **Step 2: Add the public key with forced-command restrictions on GreenCloud.**

  Copy the small public-key file to GreenCloud using the existing admin key. Then, in the already-open GreenCloud root shell, create the exact authorized-key entry from that file. The forced command invokes `sudo` only for the root-owned, argument-free receiver permitted by Task 2's sudoers rule; the receiver itself validates `SSH_ORIGINAL_COMMAND` against its seven-command whitelist.

  ```bash
  scp -i /Users/nemo/.ssh/greencloud_core_rsa2048 "$MIGRATION_KEY.pub" root@173.249.203.66:/root/greencloud-migration.pub
  sudo awk '{print "restrict,command=\"/usr/bin/sudo -n /usr/local/libexec/new-api-migration-receive\" " $0}' /root/greencloud-migration.pub |
    sudo install -m 0600 -o migration -g migration /dev/stdin /home/migration/.ssh/authorized_keys
  sudo rm -f /root/greencloud-migration.pub
  ```

  Expected: `/home/migration/.ssh/authorized_keys` contains one `restrict,command=` line. `restrict` disables PTY, X11, agent and port forwarding, and user rc files.

- [ ] **Step 3: Copy the small private key to GCP and pin the verified GreenCloud host key.**

  ```bash
  gcloud compute scp --project=stalwart-elixir-490811-q6 --zone=us-west1-b \
    "$MIGRATION_KEY" zkl@sub2api-prod:/home/zkl/.ssh/greencloud-migration
  gcloud compute ssh --project=stalwart-elixir-490811-q6 --zone=us-west1-b zkl@sub2api-prod --command='chmod 700 ~/.ssh; chmod 600 ~/.ssh/greencloud-migration; mkdir -p ~/.local/state/greencloud-migration ~/bin; chmod 700 ~/.local/state/greencloud-migration ~/bin; ssh-keyscan -t ed25519 173.249.203.66 > ~/.ssh/known_hosts-greencloud; ssh-keygen -lf ~/.ssh/known_hosts-greencloud'
  ```

  Compare the final fingerprint with Task 2, Step 3 before continuing. If it differs, delete `~/.ssh/known_hosts-greencloud` and stop; do not use `StrictHostKeyChecking=accept-new` for the production transfer.

- [ ] **Step 4: Install the GCP sender helper.**

  Open an interactive GCP SSH session and create `/home/zkl/bin/send-to-greencloud` with the following content. The helper receives the source bytes on stdin, sends them directly to GreenCloud, and stores only a source hash on GCP. It rejects any command not in the target receiver manifest.

  ```bash
  mkdir -p "$HOME/bin" "$HOME/.local/state/greencloud-migration"
  chmod 700 "$HOME/bin" "$HOME/.local/state/greencloud-migration"
  tee "$HOME/bin/send-to-greencloud" >/dev/null <<'EOF'
  #!/usr/bin/env bash
  set -euo pipefail

  test "$#" = 1
  receiver="$1"
  case "$receiver" in
    receive-new-api-postgres|receive-new-api-redis|receive-new-api-config|receive-cpa-postgres|receive-cpa-files|receive-cpa-keeper|receive-cpa-keeper-config) ;;
    *) echo 'unsupported receiver command' >&2; exit 64 ;;
  esac

  readonly state_dir="$HOME/.local/state/greencloud-migration"
  readonly ssh_key="$HOME/.ssh/greencloud-migration"
  readonly known_hosts="$HOME/.ssh/known_hosts-greencloud"
  readonly destination='migration@173.249.203.66'
  mkdir -p "$state_dir"
  chmod 700 "$state_dir"
  fifo="$(mktemp -u "$state_dir/.${receiver}.fifo.XXXXXXXX")"
  mkfifo -m 0600 "$fifo"
  trap 'rm -f "$fifo"' EXIT HUP INT TERM
  sha256sum < "$fifo" > "$state_dir/${receiver}.source.sha256" &
  hash_pid=$!
  result="$(tee "$fifo" | ssh -i "$ssh_key" -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile="$known_hosts" "$destination" "$receiver")"
  wait "$hash_pid"
  source_sha256="$(awk '{print $1}' "$state_dir/${receiver}.source.sha256")"
  target_sha256="$(awk -F= '/^plain_sha256=/{print $2}' <<< "$result")"
  test -n "$source_sha256"
  test "$source_sha256" = "$target_sha256"
  printf '%s\n' "$result"
  printf 'source_plain_sha256=%s\n' "$source_sha256"
  EOF
  chmod 700 "$HOME/bin/send-to-greencloud"
  bash -n "$HOME/bin/send-to-greencloud"
  ```

  Expected: `bash -n` exits zero. The script contains no credential value; the source-side state directory contains only SHA-256 manifest lines.

- [ ] **Step 5: Perform a harmless end-to-end receiver test.**

  From the GCP shell, stream a fixed short string through the receiver. The test exercises the exact network path but contains no production data.

  ```bash
  printf 'greencloud-direct-transfer-smoke\n' | "$HOME/bin/send-to-greencloud" receive-new-api-config
  ```

  On GreenCloud, verify the encrypted file and decrypt it only to the terminal as the known fixed test string. Then delete the smoke artifact and its checksum before transferring a real configuration archive.

  ```bash
  export CUTOVER_ID="$(sudo tr -d '\n' < /etc/new-api-migration/cutover-id)"
  sudo sha256sum --check "/srv/new-api/exports/$CUTOVER_ID/new-api-config.tar.age.sha256"
  sudo age -d -i /etc/new-api-migration/age.key "/srv/new-api/exports/$CUTOVER_ID/new-api-config.tar.age"
  sudo rm -f "/srv/new-api/exports/$CUTOVER_ID/new-api-config.tar.age" "/srv/new-api/exports/$CUTOVER_ID/new-api-config.tar.age.sha256"
  ```

  Expected: the two printed SHA-256 values from GCP match, GreenCloud prints exactly `greencloud-direct-transfer-smoke`, and no smoke file remains.

## Task 4: Rehearse every export while GCP remains authoritative

**Files:**

- Create on GCP only: hash manifests in `/home/zkl/.local/state/greencloud-migration/`
- Create on GreenCloud only: seven age-encrypted artifacts and `.sha256` files under the cutover directory

**Consumes:** The proven direct-transfer path from Task 3 and source access on GCP.

**Produces:** A complete encrypted rehearsal package that proves data movement and restore tooling without stopping public GCP traffic. It does not authorize a Cloudflare change.

- [ ] **Step 1: Inventory source container names and identify the CPA database without printing secrets.**

  In an interactive GCP shell, list only names/statuses and record the exact production services. Do not use the similarly named `new-api-test` services.

  ```bash
  cd /opt/new-api
  sudo docker compose ps
  sudo docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}'
  sudo -u postgres psql -Atc "select datname from pg_database where datistemplate = false order by datname"
  ```

  Set `NEW_API_PG_CONTAINER`, `NEW_API_REDIS_CONTAINER`, and `CPA_DB` in the GCP shell from that inventory. `CPA_DB` must be the database referenced by the already-known local CPA configuration; inspect that configuration only in a local editor and never echo its DSN. Stop if it cannot be identified confidently.

- [ ] **Step 2: Stream a consistent new-api PostgreSQL custom dump.**

  The following function reads only selected non-secret container environment fields. It does not print the full `docker inspect` environment.

  ```bash
  container_env() {
    sudo docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$1" |
      awk -F= -v key="$2" '$1 == key {print substr($0, length(key) + 2); exit}'
  }
  export NEW_API_PG_CONTAINER
  PG_USER="$(container_env "$NEW_API_PG_CONTAINER" POSTGRES_USER)"
  PG_DB="$(container_env "$NEW_API_PG_CONTAINER" POSTGRES_DB)"
  test -n "$PG_USER"
  test -n "$PG_DB"
  sudo docker exec "$NEW_API_PG_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DB" --format=custom --no-owner --no-privileges |
    "$HOME/bin/send-to-greencloud" receive-new-api-postgres
  ```

  Expected: GCP prints matching `plain_sha256` and `source_plain_sha256`; GreenCloud contains `new-api-postgres.dump.age` and its checksum. `pg_dump` is read-only and uses PostgreSQL's consistent-snapshot behavior.

- [ ] **Step 3: Produce a fresh Redis RDB and stream it as a compressed tar archive.**

  ```bash
  export NEW_API_REDIS_CONTAINER
  REDIS_PASSWORD="$(container_env "$NEW_API_REDIS_CONTAINER" REDIS_PASSWORD)"
  test -n "$REDIS_PASSWORD"
  sudo docker exec "$NEW_API_REDIS_CONTAINER" redis-cli --no-auth-warning -a "$REDIS_PASSWORD" BGSAVE
  while :; do
    persistence="$(sudo docker exec "$NEW_API_REDIS_CONTAINER" redis-cli --no-auth-warning -a "$REDIS_PASSWORD" INFO persistence)"
    grep -q '^rdb_bgsave_in_progress:0' <<< "$persistence" && grep -q '^rdb_last_bgsave_status:ok' <<< "$persistence" && break
    sleep 2
  done
  sudo docker cp "$NEW_API_REDIS_CONTAINER":/data/dump.rdb - |
    gzip -1 |
    "$HOME/bin/send-to-greencloud" receive-new-api-redis
  unset REDIS_PASSWORD persistence
  ```

  Expected: Redis remains online; the target receives an encrypted gzip-compressed tar stream. `docker cp ... -` is deliberately used because it emits a tar stream without creating a source RDB copy.

- [ ] **Step 4: Stream the private new-api configuration input as an encrypted archive.**

  The target configuration must preserve application cryptographic continuity but must change internal service hostnames. Transfer the source Compose and `.env` inputs for private review on GreenCloud; do not install them directly as the target Compose file.

  ```bash
  sudo tar -C /opt/new-api -cf - docker-compose.yml .env |
    "$HOME/bin/send-to-greencloud" receive-new-api-config
  ```

  Expected: only `new-api-config.tar.age` reaches GreenCloud. If the production source does not use `/opt/new-api/.env`, archive the exact private env file referenced by `/opt/new-api/docker-compose.yml` instead and record its source path privately; do not substitute the test stack.

- [ ] **Step 5: Stream CPA database and state.**

  `CPA_DB` is the database identified in Step 1. The GreenCloud PostgreSQL 16 service is dedicated to CPA, so its full logical database dump is restored into a fresh target database.

  ```bash
  export CPA_DB
  test -n "$CPA_DB"
  sudo -u postgres pg_dump -d "$CPA_DB" --format=custom --no-owner --no-privileges |
    "$HOME/bin/send-to-greencloud" receive-cpa-postgres
  sudo tar --numeric-owner --xattrs --acls -C /opt/cliproxy -czf - pgstore config.yaml .env |
    "$HOME/bin/send-to-greencloud" receive-cpa-files
  ```

  Expected: both CPA artifacts arrive as age-encrypted files. The running CPA remains available during this rehearsal; its state can advance afterward, so these rehearsal exports are never the final source of truth.

- [ ] **Step 6: Stream CPA keeper state and configuration.**

  ```bash
  sudo tar --numeric-owner --xattrs --acls -C /opt/cpacodexkeeper -czf - runtime |
    "$HOME/bin/send-to-greencloud" receive-cpa-keeper
  sudo tar -C /opt/cpacodexkeeper -cf - .env |
    "$HOME/bin/send-to-greencloud" receive-cpa-keeper-config
  ```

  Expected: the package includes runtime state and configuration separately. Logs are excluded because they are not a restore prerequisite and would inflate the transfer.

## Task 5: Verify encrypted artifacts and rehearse the target restore

**Files:**

- Read on GreenCloud only: `/srv/new-api/exports/<cutover-id>/*`
- Create temporarily on GreenCloud only: root-owned restore work under `/srv/new-api/restore-check/<cutover-id>/`
- Do not create: a plaintext dump/archive on the Mac or GCP

**Consumes:** The seven completed artifacts from Task 4 and already loaded GreenCloud images.

**Produces:** Integrity evidence, PostgreSQL archive listings, a Redis RDB structural check, and a restore rehearsal before final cutover.

- [ ] **Step 1: Verify every encrypted checksum and age authentication.**

  On GreenCloud:

  ```bash
  export CUTOVER_ID="$(sudo tr -d '\n' < /etc/new-api-migration/cutover-id)"
  export EXPORT_DIR="/srv/new-api/exports/$CUTOVER_ID"
  cd "$EXPORT_DIR"
  for checksum in *.sha256; do sudo sha256sum --check "$checksum"; done
  for artifact in *.age; do sudo age -d -i /etc/new-api-migration/age.key "$artifact" >/dev/null; done
  ```

  Expected: every command returns zero. An authentication failure, missing artifact, or hash mismatch is a hard stop; rerun only that source export after removing its failed encrypted artifact and checksum.

- [ ] **Step 2: Validate PostgreSQL dump structure without writing it as plaintext.**

  ```bash
  export NEW_API_POSTGRES_IMAGE="$(awk -F= '$1 == "NEW_API_POSTGRES_IMAGE" {print substr($0, length($1) + 2)}' /srv/new-api/env/images.env)"
  sudo age -d -i /etc/new-api-migration/age.key "$EXPORT_DIR/new-api-postgres.dump.age" |
    sudo docker run --rm -i --entrypoint pg_restore "$NEW_API_POSTGRES_IMAGE" --list >/dev/null
  sudo age -d -i /etc/new-api-migration/age.key "$EXPORT_DIR/cpa-postgres.dump.age" |
    sudo -u postgres pg_restore --list >/dev/null
  ```

  Expected: both `pg_restore --list` commands exit zero. The dump bytes exist only in process pipes.

- [ ] **Step 3: Validate the Redis RDB in a disposable container filesystem.**

  ```bash
  export REDIS_IMAGE="$(awk -F= '$1 == "REDIS_IMAGE" {print substr($0, length($1) + 2)}' /srv/new-api/env/images.env)"
  sudo age -d -i /etc/new-api-migration/age.key "$EXPORT_DIR/new-api-redis.rdb.tar.gz.age" |
    sudo docker run --rm -i --entrypoint sh "$REDIS_IMAGE" -ec 'f=$(mktemp); trap "rm -f $f" EXIT; gzip -dc | tar -xOf - dump.rdb > "$f"; redis-check-rdb "$f" >/dev/null'
  ```

  Expected: `redis-check-rdb` exits zero, and its temporary plaintext RDB disappears when the disposable container exits.

- [ ] **Step 4: Restore into a disposable/rehearsal target before using the real target data directories.**

  Stop before this step if any image, env file, or CPA database role is not ready. Create a separate rehearsal PostgreSQL database and a temporary CPA state directory, restore into those locations, and start no public listener. The exact service startup and acceptance checks are those in Tasks 4–5 of the main plan; they must prove existing administrator login, existing channel decryption, a CPA normal request, and CPA SSE through `host.docker.internal:8317`.

  Expected: the first end-to-end functional proof occurs while Cloudflare continues to route to GCP. Failure leaves GCP authoritative and requires no DNS rollback.

## Task 6: Perform the final freeze and direct final export

**Files:**

- Replace on GreenCloud only: the seven encrypted artifacts in a new final cutover directory
- Modify temporarily on GCP only: runtime service state during the approved maintenance window

**Consumes:** A successful rehearsal and the maintenance window. This task is externally service-affecting because it stops GCP writers.

**Produces:** One internally consistent final package from a quiesced source. It still does not change Cloudflare.

- [ ] **Step 1: Open the maintenance window and stop every GCP writer in dependency order.**

  Put the public service into the approved maintenance state, then in an interactive GCP shell stop the keeper, `new-api`, and CPA before taking final state archives. Do not stop PostgreSQL or Redis until their dumps have completed.

  ```bash
  cd /opt/cpacodexkeeper && sudo docker compose stop
  cd /opt/new-api && sudo docker compose stop new-api
  sudo systemctl stop cliproxyapi
  sudo systemctl is-active --quiet cliproxyapi && exit 1 || true
  ```

  Expected: no application, CPA, or keeper writer can advance the final data while the exports below run. Keep GCP disks, images, and configuration intact.

- [ ] **Step 2: Create a new final cutover ID and repeat Task 4 exactly.**

  On GreenCloud, repeat Task 1 Step 1 with a new final ID before sending. On GCP, repeat Task 4 Steps 2–6 to produce the final PostgreSQL, Redis, configuration, CPA, and keeper artifacts. Do not reuse the rehearsal artifacts.

  Expected: each final source hash equals the target plaintext hash reported by the receiver. The new directory contains exactly seven `.age` artifacts and seven `.sha256` files.

- [ ] **Step 3: Verify and restore the final package, then run GreenCloud's local acceptance matrix.**

  Repeat Task 5 Steps 1–3, then perform the real target restore and tests specified in the main plan. Do not start `cloudflared`; keep `new-api` loopback-only. For the restored CPA channel, use `http://host.docker.internal:8317`, never the container's own `127.0.0.1`.

  Expected: GreenCloud local `/api/status`, authenticated `/v1/models`, normal relay, SSE relay, keeper access, and billing/log writes pass before any public route change.

- [ ] **Step 4: Abort safely if any final verification fails.**

  Do not attempt cross-host merging. Restart the stopped GCP components and retain GreenCloud's encrypted package as a forensic artifact.

  ```bash
  sudo systemctl start cliproxyapi
  cd /opt/new-api && sudo docker compose start new-api
  cd /opt/cpacodexkeeper && sudo docker compose start
  ```

  Expected: GCP returns to its prior single-write-authority state before any repair. Cloudflare has not changed, so public rollback is not required.

## Task 7: Handoff to the Cloudflare cutover and keep a simple rollback path

**Files:**

- Modify only after separate approval: Cloudflare Tunnel/DNS configuration
- Preserve: GreenCloud final encrypted package and intact GCP runtime

**Consumes:** A successful final restore and the user's explicit approval of the exact hostname and maintenance window.

**Produces:** A public-route decision, not a data-transfer action.

- [ ] **Step 1: Hold the Cloudflare cutover until the full local acceptance matrix is green.**

  Follow Task 6 of the main migration plan for the named Tunnel, WAF/Access/Turnstile parity, public HTTPS, authenticated API, and long SSE tests. Only `new-api` is exposed through the Tunnel. CPA (`8317`), PostgreSQL (`5432`), Redis (`6379`), and keeper remain private.

  Expected: the user explicitly approves the externally visible route change after the data transfer has already passed.

- [ ] **Step 2: If post-cutover service fails, restore Cloudflare to GCP first.**

  Do not try to synchronize writes in both directions. Repoint the exact public hostname to the preserved GCP origin/Tunnel, verify `/api/status`, `/v1/models`, and SSE, then start the GCP services with the commands in Task 6 Step 4.

  Expected: one host is write authority at all times. GreenCloud's final state remains available for investigation but does not accept public traffic after rollback.

## Task 8: Revoke the temporary path and retain only encrypted evidence

**Files:**

- Delete on GCP after final verification: `/home/zkl/.ssh/greencloud-migration`, `/home/zkl/.ssh/known_hosts-greencloud`, `/home/zkl/bin/send-to-greencloud`, and `/home/zkl/.local/state/greencloud-migration/`
- Delete on GreenCloud after final verification: `/home/migration/.ssh/authorized_keys`, `/usr/local/libexec/new-api-migration-receive`, `/etc/sudoers.d/new-api-migration`, `/etc/new-api-migration/cutover-id`, and the `migration` account
- Delete on the Mac after final verification: the one-time `$MIGRATION_KEY` and `.pub`
- Retain on GreenCloud: only the age-encrypted completed package under `/srv/new-api/exports/<final-cutover-id>/`

**Consumes:** A complete verified final package and either a successful cutover or a recorded rollback.

**Produces:** No GCP-to-GreenCloud login path and no plaintext export; the encrypted package remains available for the rollback-window audit.

- [ ] **Step 1: Remove the source private key and sender state from GCP.**

  ```bash
  rm -f "$HOME/.ssh/greencloud-migration" "$HOME/.ssh/known_hosts-greencloud" "$HOME/bin/send-to-greencloud"
  rm -rf "$HOME/.local/state/greencloud-migration"
  test ! -e "$HOME/.ssh/greencloud-migration"
  ```

  Expected: GCP can no longer authenticate as `migration`.

- [ ] **Step 2: Remove the GreenCloud authorized key, receiver, and account.**

  ```bash
  sudo rm -f /home/migration/.ssh/authorized_keys /usr/local/libexec/new-api-migration-receive /etc/sudoers.d/new-api-migration /etc/new-api-migration/cutover-id
  sudo userdel --remove migration
  sudo test ! -e /usr/local/libexec/new-api-migration-receive
  sudo test ! -e /home/migration
  ```

  Expected: no temporary migration SSH identity, key, shell wrapper, or user remains. The `/srv/new-api/exports/<final-cutover-id>/` directory stays root-owned and age encrypted.

- [ ] **Step 3: Delete the Mac's one-time key only after checking the GCP and GreenCloud removals.**

  ```bash
  rm -f "$MIGRATION_KEY" "$MIGRATION_KEY.pub"
  test ! -e "$MIGRATION_KEY"
  ```

  Expected: the full direct-transfer privilege has been revoked from all three machines.

## Cutover decision gates

| Gate | Required proof | If false |
| --- | --- | --- |
| No local data transit | The Mac has no dump/archive; GCP sender output is only hash lines | Stop and remove the incomplete local artifact. |
| Receiver confinement | `migration` has `nologin`, `restrict`, one forced command, and a seven-command whitelist | Stop; do not give GCP a general GreenCloud shell. |
| Transport integrity | Source SHA-256 equals GreenCloud plaintext SHA-256; encrypted SHA and age decryption both pass | Delete only that failed artifact and re-export it. |
| Restore integrity | Both PostgreSQL TOCs list, Redis RDB check passes, and rehearsal restores | Keep GCP authoritative; repair the target. |
| Final consistency | GCP writers are stopped before the final seven artifacts | Do not cut over; a rehearsal package cannot be production state. |
| Public cutover | User approves exact Cloudflare hostname/window after GreenCloud local acceptance | Do not change Cloudflare. |
| Revocation | GCP key/sender and GreenCloud receiver/account are removed after verification | Treat the migration as not closed. |

## Self-review

- Scope coverage: the plan transfers `new-api` PostgreSQL, Redis, private configuration inputs, CPA PostgreSQL/state, CPA keeper runtime/configuration, and explicitly excludes Cloudflare credentials from the data path.
- Truncation coverage: every large byte stream originates and terminates remotely; `gcloud` carries only a short private key and command control, while the Mac never receives a dump.
- Security coverage: the temporary SSH identity is host-key pinned, forced-command restricted, key-scoped, non-interactive, encrypted at rest immediately, and revoked after use.
- Consistency coverage: rehearsal is read-only; final export stops all writers, compares source/target plaintext hashes, validates archive formats, and leaves Cloudflare untouched until an independent approval gate.
