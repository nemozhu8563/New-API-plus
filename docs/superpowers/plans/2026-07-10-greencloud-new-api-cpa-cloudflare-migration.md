# GreenCloud new-api, CPA, and Cloudflare Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the production `new-api` stack, the host-native CLIProxyAPI (CPA) and its keeper, and the public Cloudflare entry point from GCP to GreenCloud while keeping GCP available for a seven-day rollback window.

**Architecture:** GreenCloud runs CPA as a native systemd service bound only to `127.0.0.1:8317`, with a loopback-only host PostgreSQL 16 service for CPA's existing PG store. `new-api`, PostgreSQL 15, Redis, `cpacodexkeeper`, and `cloudflared` run as containers on an isolated Docker bridge; `new-api` reaches CPA through Docker's `host-gateway` mapping. Cloudflare exposes only `new-api` through a named Tunnel; CPA, PostgreSQL, and Redis have no public listener.

**Tech Stack:** Ubuntu 24.04 on GreenCloud (`nemo-Phoenix`, x86_64, 2 vCPU, 3.8 GiB RAM), Docker Compose, PostgreSQL 15 for `new-api`, host PostgreSQL 16 for CPA's PG store, Redis, native Linux AMD64 CLIProxyAPI 7.2.47, `cloudflared`, systemd, Cloudflare Tunnel, GCP `gcloud` for source-side export.

## Global Constraints

- This plan migrates CPA **to GreenCloud**. GCP is a source, validation target, and rollback environment only; it is not the CPA destination.
- The GreenCloud host never builds images and never clones this repository. Every container image is built or pulled on the local Mac, exported with `docker save`, checksummed, uploaded, verified, and then loaded with `docker load`.
- GCP database/state exports must never be routed through the local Codex/gcloud command channel. That channel has been observed truncating large streams. The mandatory GCP-to-GreenCloud direct-transfer runbook is [2026-07-11-greencloud-direct-data-transfer-runbook.md](2026-07-11-greencloud-direct-data-transfer-runbook.md); it uses a one-time, forced-command SSH receiver and keeps data bytes off the Mac.
- CPA remains host-native in the first migration. Do not containerize CLIProxyAPI in this change.
- Do not print, commit, paste into chat, or place in this document any database DSN, CPA API key, OAuth/auth file, Cloudflare Tunnel token, Feishu webhook/secret, `SESSION_SECRET`, or encryption key.
- Preserve the existing `SESSION_SECRET` and any encryption key used to decrypt `Channel` data, unless a deliberate global logout/key-rotation is approved. The exposed CPA public API keys have already been rotated and the two GCP `new-api` CPA channels were updated; do not rotate the distinct CPA remote-management/keeper secret without a separate reason.
- Keep GCP `sub2api-prod` running and retain the final encrypted source exports for seven days after production cutover. Do not delete GCP disks, images, Artifact Registry packages, snapshots, Tunnel routes, or DNS records during that period.
- `new-api` supports SQLite, MySQL, and PostgreSQL in the codebase; this production migration keeps PostgreSQL and does not change application database semantics.
- The GCP source also has `new-api-test` on `127.0.0.1:3001`. It is a separately deployed stack (`/opt/new-api-test`) and must be moved only after its SQL/Redis target is inventoried; it must not accidentally share the production database during this migration.

---

## Verified baseline (2026-07-10)

| Area | Evidence | Migration implication |
| --- | --- | --- |
| GCP production host | `sub2api-prod`, `us-west1-b`, `e2-medium`, 50 GB `pd-balanced`; public IP exists | Retain untouched for rollback. |
| GreenCloud target | `nemo-Phoenix`, x86_64, 2 vCPU, 3.8 GiB RAM; Docker, Compose, PostgreSQL 16, age, and UFW are installed, and host preflight passed | Capacity is sufficient for the current light stack. Application/data ports remain closed, no workload has started, and backup/monitoring are still cutover gates. |
| `new-api` source | Docker service in `/opt/new-api`; container binds only `127.0.0.1:3000`; PostgreSQL 15 and Redis are separate containers | Export the production PostgreSQL database from the container; keep all data services on the Docker bridge. |
| CPA source | `cliproxyapi.service`, root-owned native binary `/opt/cliproxy/bin/CLIProxyAPI`, version 7.2.47, config `/opt/cliproxy/config.yaml`, state under `/opt/cliproxy/pgstore` | Reproduce as a native service; export both the state tree and the referenced host PostgreSQL store. |
| CPA exposure | CPA listens on `*:8317`; host PostgreSQL listens only on loopback | Bind CPA to `127.0.0.1:8317` on GreenCloud and explicitly test Docker-to-host access. |
| CPA persistence | host PostgreSQL 15 cluster is online and its non-template `postgres` database is 247 MB; CPA environment names include `PGSTORE_DSN`, `PGSTORE_LOCAL_PATH`, and `PGSTORE_SCHEMA` | Treat the relevant CPA PostgreSQL schema plus `/opt/cliproxy/pgstore` as authoritative state; determine the exact schema without logging its DSN. |
| CPA support service | `cpacodexkeeper` is a Docker service in `/opt/cpacodexkeeper`, with persisted `logs` and `runtime`; it holds CPA, Feishu, alerting, refresh, and quota-monitor configuration | It is in migration scope and needs its own encrypted configuration/state transfer and functional alert test. |
| Cloudflare | No account or DNS changes were made during this audit | Use a named Tunnel and migrate the existing public hostname only after local and external smoke tests pass. |

## Target release layout

```text
/srv/new-api/
  compose.yaml                     # container topology; no secrets
  env/new-api.env                  # mode 0600, production-only secrets
  env/cpacodexkeeper.env           # mode 0600, production-only secrets
  images/$RELEASE/                 # uploaded *.tar and SHA256SUMS
  exports/$CUTOVER_ID/             # encrypted, short-lived source imports
  logs/
  data/

/opt/cliproxy/
  bin/CLIProxyAPI                  # verified Linux AMD64 release binary
  pgstore/                         # restored CPA local state, 0700
  logs/                            # runtime logs

/etc/cliproxy/
  config.yaml                      # mode 0600, copied without exposing values
  cliproxy.env                     # mode 0600

/etc/systemd/system/cliproxyapi.service

/var/lib/postgresql/16/main/        # host PostgreSQL for CPA only; loopback listener
```

The implementation creates these repository-owned deployment files; none are created by this planning change:

| File | Responsibility |
| --- | --- |
| `ops/greencloud/compose.yaml` | Production container graph, loopback-only optional diagnostics port, health checks, and `host-gateway` mapping. |
| `ops/greencloud/env/new-api.env.example` | Variable names only for `new-api`; no values. |
| `ops/greencloud/env/cpacodexkeeper.env.example` | Variable names only for the keeper; no values. |
| `ops/greencloud/systemd/cliproxyapi.service` | Native CPA lifecycle and restart policy. |
| `ops/greencloud/scripts/build-release.sh` | Local-Mac-only image build/pull, digest capture, save, and checksum generation. |
| `ops/greencloud/scripts/verify-release.sh` | GreenCloud checksum and `docker load` verification. |
| `ops/greencloud/scripts/backup-restore-check.sh` | Restores encrypted database/state exports into a disposable validation directory and runs non-secret integrity checks. |

## Task 1: Close the source-inventory and secret-safety gates

**Files:**

- Create: `ops/greencloud/env/new-api.env.example`
- Create: `ops/greencloud/env/cpacodexkeeper.env.example`
- Create: `docs/reports/2026-07-10-greencloud-migration-source-inventory.md`

**Consumes:** The verified baseline above and read-only access to GCP.

**Produces:** A redacted manifest naming every data store, persistent path, public hostname, Cloudflare zone/Tunnel, and credential owner. It also produces a credential-rotation record that contains identifiers and timestamps, never secret values.

- [ ] **Step 1: Create the redacted source inventory without reading secret values.**

  Record these already verified paths and services: `/opt/new-api`, `/opt/new-api-test`, `/opt/cliproxy`, `/opt/cpacodexkeeper`, `cliproxyapi.service`, GCP Docker containers `new-api`, `new-api-test`, `postgres`, `redis`, and `cpacodexkeeper`. Run only metadata commands such as:

  ```bash
  gcloud compute ssh sub2api-prod --zone=us-west1-b \
    --project=stalwart-elixir-490811-q6 --quiet \
    --command='sudo docker ps --format "table {{.Names}}\\t{{.Image}}\\t{{.Ports}}\\t{{.Status}}"'
  ```

  Expected: container names/images/ports/status only; no environment-variable values.

- [ ] **Step 2: Inventory both production and test database targets by name, not by DSN.**

  Use a one-shot, local-only parser that receives each `SQL_DSN` and outputs only driver, hostname, port, and database name. Do not use `docker inspect` output directly in a terminal log. Record whether `new-api-test` has an independent PostgreSQL database and Redis database. Block the test migration if either one resolves to the production database/keyspace.

  Expected: `new-api` and `new-api-test` each have a named database/Redis isolation decision; no password, username, or full DSN appears in the report.

- [ ] **Step 3: Inventory Cloudflare before touching it.**

  In the Cloudflare dashboard, record the exact production hostname, zone, current DNS record target, current proxy state, existing Tunnel name/UUID if any, Turnstile site/key configuration references, Access/WAF rules, and the owner of the API/token credential. Do not export tokens into the repository or shell history.

  Expected: one redacted record identifies the single hostname that can be switched and all policies that must remain effective.

- [x] **Step 4: Rotate the exposed CPA public API keys and atomically update dependents.**

  Completed before this runbook update: CPA's public API keys were rotated, GCP `new-api` channels `1` (`my-cpa-codex`) and `32` (`my-cpa-codex_生图专用`) were updated, and CPA returned to `active`. The CPA remote-management `secret-key` and the keeper `CPA_TOKEN` were verified to be a distinct shared management credential, so they were deliberately not rotated as part of the public-key incident.

  Expected: the old public keys are rejected; CPA and keeper are healthy with the replacement; the report records key labels/timestamps only.

- [ ] **Step 5: Create secret-free environment templates.**

  `ops/greencloud/env/new-api.env.example` must contain the variable names currently required by the production container, including `SQL_DSN`, `REDIS_CONN_STRING`, `SESSION_SECRET`, `TZ`, rate-limit settings, and error-monitor settings. `ops/greencloud/env/cpacodexkeeper.env.example` must contain its currently observed `CPA_*` and `FEISHU_*` variable names. Put comments next to sensitive names saying `store outside git`.

  Expected: `git grep -nE '(TOKEN|SECRET|PASSWORD|WEBHOOK).*=' ops/greencloud/env` finds only empty assignments or comments; no secret is staged.

- [ ] **Step 6: Commit the templates and redacted inventory.**

  ```bash
  git add ops/greencloud/env docs/reports/2026-07-10-greencloud-migration-source-inventory.md
  git commit -m "docs: record GreenCloud migration inventory and secret boundaries"
  ```

## Task 2: Make a local, immutable Linux AMD64 release bundle

**Files:**

- Create: `ops/greencloud/scripts/build-release.sh`
- Create: `ops/greencloud/scripts/verify-release.sh`
- Create: `ops/greencloud/README.md`

**Consumes:** A clean local checkout at the selected Git revision, Docker Buildx on the Mac, and approved upstream image digests.

**Produces:** A release directory containing `new-api`, PostgreSQL 15, Redis, `cloudflared`, and keeper images as tar files plus one `SHA256SUMS` manifest. No source checkout or build cache is needed on GreenCloud.

- [ ] **Step 1: Define the release identity from the actual commit.**

  ```bash
  export RELEASE="$(date +%Y%m%d-%H%M%S)-$(git rev-parse --short=12 HEAD)"
  export RELEASE_DIR="$PWD/dist/greencloud/$RELEASE"
  mkdir -p "$RELEASE_DIR/images"
  git status --short
  ```

  Expected: record the selected commit and any pre-existing local modifications in the release manifest. Do not build from an unexplained dirty tree.

- [ ] **Step 2: Build the application image locally for the target architecture.**

  ```bash
  docker buildx build --platform linux/amd64 --load \
    --tag "new-api:greencloud-$RELEASE" .
  docker image inspect "new-api:greencloud-$RELEASE" \
    --format '{{index .RepoDigests 0}} {{.Id}}' || true
  ```

  Expected: the image is locally available and reports Linux/amd64; the `RepoDigests` field may be empty for a locally built image, so retain the image ID and the `docker save` checksum as its immutable identity.

- [ ] **Step 3: Resolve and pull every non-application image locally by immutable Linux/AMD64 digest.**

  Choose explicit vendor release tags for PostgreSQL 15, Redis, and cloudflared in the local release script; never use `latest`. Resolve each selected tag's Linux/AMD64 manifest digest and write the exact `name@sha256:digest` reference to `release-manifest.json` before pulling it:

  ```bash
  resolve_linux_amd64_digest() {
    docker buildx imagetools inspect --raw "$1" |
      jq -r '.manifests[] | select(.platform.os == "linux" and .platform.architecture == "amd64") | .digest'
  }
  export POSTGRES_IMAGE="postgres@$(resolve_linux_amd64_digest "$POSTGRES_RELEASE_TAG")"
  export REDIS_IMAGE="redis@$(resolve_linux_amd64_digest "$REDIS_RELEASE_TAG")"
  export CLOUDFLARED_IMAGE="cloudflare/cloudflared@$(resolve_linux_amd64_digest "$CLOUDFLARED_RELEASE_TAG")"
  jq -n --arg postgres "$POSTGRES_IMAGE" --arg redis "$REDIS_IMAGE" \
    --arg cloudflared "$CLOUDFLARED_IMAGE" \
    '{postgres:$postgres,redis:$redis,cloudflared:$cloudflared}' > "$RELEASE_DIR/release-manifest.json"
  docker pull --platform linux/amd64 "$POSTGRES_IMAGE"
  docker pull --platform linux/amd64 "$REDIS_IMAGE"
  docker pull --platform linux/amd64 "$CLOUDFLARED_IMAGE"
  ```

  Expected: `POSTGRES_RELEASE_TAG`, `REDIS_RELEASE_TAG`, and `CLOUDFLARED_RELEASE_TAG` are explicit vendor release tags selected and reviewed at release time; the resulting manifest has one non-empty digest per image. No later GreenCloud `docker pull`, `docker build`, or Git clone is required.

- [ ] **Step 4: Save and checksum the full image set locally.**

  ```bash
  docker save -o "$RELEASE_DIR/images/new-api.tar" "new-api:greencloud-$RELEASE"
  docker save -o "$RELEASE_DIR/images/postgres.tar" "$POSTGRES_IMAGE"
  docker save -o "$RELEASE_DIR/images/redis.tar" "$REDIS_IMAGE"
  docker save -o "$RELEASE_DIR/images/cloudflared.tar" "$CLOUDFLARED_IMAGE"
  docker save -o "$RELEASE_DIR/images/cpacodexkeeper.tar" "$CPA_KEEPER_IMAGE"
  (cd "$RELEASE_DIR" && shasum -a 256 images/*.tar > SHA256SUMS)
  ```

  Expected: `shasum -a 256 -c SHA256SUMS` passes locally. `CPA_KEEPER_IMAGE` is set only by a local build of the keeper's source at its recorded revision. If that source/build procedure is unavailable, stop this task and record the blocker; do not silently copy the GCP image as the new supply chain.

- [ ] **Step 5: Obtain the CPA release on the Mac from its verified release source.**

  Download the Linux AMD64 binary/package corresponding to verified CPA version 7.2.47 (commit `00114bec`), verify the vendor-provided checksum/signature, and write the version, commit, URL, checksum, and retrieval time into the local release manifest. Do not use `/opt/cliproxy/bin/CLIProxyAPI` from GCP as a release artifact.

  Expected: `file CLIProxyAPI` reports `ELF 64-bit ... x86-64`, and the local manifest links the binary to an independently retrievable release.

- [ ] **Step 6: Commit only scripts, examples, and documentation.**

  ```bash
  git add ops/greencloud
  git commit -m "build: add offline GreenCloud release bundle tooling"
  ```

  `dist/greencloud/` remains ignored and is never committed.

## Task 3: Provision GreenCloud as an isolated production base

**Files:**

- Create: `ops/greencloud/compose.yaml`
- Create: `ops/greencloud/systemd/cliproxyapi.service`
- Create: `ops/greencloud/systemd/cliproxyapi.service.d/10-environment.conf`
- Create: `ops/greencloud/scripts/host-preflight.sh`

**Consumes:** The GreenCloud host, its dedicated SSH key, and the local release bundle from Task 2.

**Produces:** A host with Docker, a dedicated non-login `cliproxy` user, minimal firewall rules, log rotation, and no application port exposed to the Internet.

- [ ] **Step 1: Install and verify the host runtime.**

  On GreenCloud, install Ubuntu's supported `docker.io` and `docker-compose-v2` packages, plus `postgresql-16`, `redis-tools`, `age`, `ufw`, and `jq`. PostgreSQL 16 is deliberately a separate loopback-only host service for CPA: the source state is exported with logical `pg_dump -Fc`, so restore from source PostgreSQL 15 does not require a physical major-version match. `new-api` continues to use its separate PostgreSQL 15 container.

  ```bash
  docker --version
  docker compose version
  pg_lsclusters
  getent passwd cliproxy || sudo useradd --system --home /opt/cliproxy --shell /usr/sbin/nologin cliproxy
  sudo install -d -m 0750 -o root -g root /srv/new-api /srv/new-api/{env,exports,images,logs,data}
  sudo install -d -m 0700 -o cliproxy -g cliproxy /opt/cliproxy/{bin,pgstore,logs}
  ```

  Expected: the target remains reachable only by SSH during this stage; no `3000`, `5432`, `6379`, or `8317` listener exists.

- [ ] **Step 2: Install a default-deny inbound firewall.**

  Allow the existing SSH administration path and deny public application/data ports:

  ```bash
  sudo ufw default deny incoming
  sudo ufw default allow outgoing
  sudo ufw allow 22/tcp
  sudo ufw deny 3000/tcp
  sudo ufw deny 3001/tcp
  sudo ufw deny 5432/tcp
  sudo ufw deny 6379/tcp
  sudo ufw deny 8317/tcp
  sudo ufw enable
  sudo ufw status numbered
  ```

  Expected: inbound SSH is explicitly confirmed from a second session before ending the first session. Cloudflare Tunnel needs only outbound connectivity.

- [ ] **Step 3: Define the production Compose topology.**

  `ops/greencloud/compose.yaml` contains `new-api`, `postgres`, `redis`, `cpacodexkeeper`, and `cloudflared`, all on one named internal bridge. It uses exact release image names, persistent named volumes/bind directories under `/srv/new-api`, restart policies, and health checks. The critical `new-api` section is:

  ```yaml
  services:
    new-api:
      image: ${NEW_API_IMAGE}
      env_file: /srv/new-api/env/new-api.env
      extra_hosts:
        - host.docker.internal:host-gateway
      ports:
        - 127.0.0.1:3000:3000
      networks: [backend]
  networks:
    backend:
      name: new-api-backend
      driver: bridge
  ```

  CPA channels in the restored `new-api` database must use `http://host.docker.internal:8317`, not `http://127.0.0.1:8317`; Docker's `127.0.0.1` is the container itself.

- [ ] **Step 4: Define the native CPA service.**

  Start with the source service contract—working directory, `.env` equivalent, config path, `Restart=always`, and `LimitNOFILE=65535`—but run as `cliproxy` after restoring ownership. The service must be structurally equivalent to:

  ```ini
  [Service]
  Type=simple
  User=cliproxy
  Group=cliproxy
  WorkingDirectory=/opt/cliproxy
  EnvironmentFile=/etc/cliproxy/cliproxy.env
  ExecStart=/opt/cliproxy/bin/CLIProxyAPI -config /etc/cliproxy/config.yaml
  Restart=always
  RestartSec=5
  LimitNOFILE=65535
  NoNewPrivileges=true
  ```

  Do not add stronger systemd sandbox directives until CPA starts, refreshes credentials, writes its configured state, and passes a streaming request under the `cliproxy` account. A failed non-root validation is a blocking compatibility finding, not a reason to revert automatically to root.

- [ ] **Step 5: Transfer the release without server-side builds.**

  ```bash
  rsync -avP -e 'ssh -i /Users/nemo/.ssh/greencloud_core_rsa2048' \
    "$RELEASE_DIR/" root@173.249.203.66:/srv/new-api/images/$RELEASE/
  ssh -i /Users/nemo/.ssh/greencloud_core_rsa2048 root@173.249.203.66 \
    "cd /srv/new-api/images/$RELEASE && shasum -a 256 -c SHA256SUMS"
  ```

  Expected: all checksums pass before `docker load -i` is run for each tarball. The host contains images but no application containers yet.

- [ ] **Step 6: Commit the deployment topology.**

  ```bash
  git add ops/greencloud/compose.yaml ops/greencloud/systemd ops/greencloud/scripts/host-preflight.sh
  git commit -m "ops: define isolated GreenCloud production topology"
  ```

## Task 4: Export, restore, and validate CPA before application cutover

**Files:**

- Create: `ops/greencloud/scripts/export-cpa-state.sh`
- Create: `ops/greencloud/scripts/restore-cpa-state.sh`
- Create: `docs/reports/2026-07-10-cpa-migration-validation.md`

**Consumes:** The rotated CPA credentials, the verified local CPA binary, source `/opt/cliproxy`, the relevant source PostgreSQL schema, and the GreenCloud host from Task 3.

**Produces:** A working, loopback-only CPA instance on GreenCloud plus a validation report; no public Cloudflare route is changed in this task.

- [ ] **Step 1: Make an encrypted, checksummed CPA state export on GCP.**

  Quiesce only the CPA writer for the short export window, create a PostgreSQL custom-format dump of the exact database/schema named by `PGSTORE_DSN`/`PGSTORE_SCHEMA`, and archive `/opt/cliproxy/pgstore`, `/opt/cliproxy/config.yaml`, and `.env` with ownership/mode metadata. Encrypt the archive before copying it off GCP. Exclude `logs`, old `backups`, `releases`, and `tmp` from the authoritative state archive.

  Expected: a manifest contains file paths, permissions, archive checksum, PostgreSQL dump checksum, source CPA version, and export time—never the file contents or DSN.

- [ ] **Step 2: Restore CPA state with least privilege.**

  Restore the PostgreSQL dump into the target host PostgreSQL instance, restore the state tree to `/opt/cliproxy/pgstore`, install the version-verified native binary, install `/etc/cliproxy/config.yaml` and `/etc/cliproxy/cliproxy.env` with mode `0600`, and set CPA-owned runtime/state files to `cliproxy:cliproxy`.

  Expected: `systemctl enable --now cliproxyapi` is successful and `ss -ltnp | grep ':8317'` shows exactly `127.0.0.1:8317`, never `*:8317`.

- [ ] **Step 3: Migrate and validate `cpacodexkeeper`.**

  Restore only `/opt/cpacodexkeeper/runtime` needed by its state files. Rebuild/pull its image locally as Task 2 requires, load it on GreenCloud, create `/srv/new-api/env/cpacodexkeeper.env` with the rotated `CPA_TOKEN` and existing notification values outside Git, and start the container on the same Docker bridge. Set `CPA_ENDPOINT` to `http://host.docker.internal:8317` and give the keeper the same `extra_hosts` mapping.

  Expected: the keeper reaches CPA, a non-production test notification reaches the existing Feishu destination, and no notification secret appears in Docker inspection output, logs, or the report.

- [ ] **Step 4: Run the CPA acceptance suite.**

  From the GreenCloud host, test service restart, config/auth-state read, one provider token refresh where safe, one normal request, one SSE streaming request, and keeper quota collection. Run Docker-origin testing from the `new-api` network namespace to prove `host.docker.internal:8317` works.

  Expected: all requests work after `systemctl restart cliproxyapi` and `docker compose restart cpacodexkeeper`; CPA is inaccessible from the public Internet.

- [ ] **Step 5: Commit only scripts and the redacted validation report.**

  ```bash
  git add ops/greencloud/scripts docs/reports/2026-07-10-cpa-migration-validation.md
  git commit -m "ops: document native CPA migration and validation"
  ```

## Task 5: Move new-api data and connect it to CPA

**Files:**

- Modify: `ops/greencloud/compose.yaml`
- Create: `ops/greencloud/scripts/export-new-api-data.sh`
- Create: `ops/greencloud/scripts/restore-new-api-data.sh`
- Create: `docs/reports/2026-07-10-new-api-data-restore-validation.md`

**Consumes:** The GreenCloud Compose base, CPA from Task 4, and the source `new-api` PostgreSQL/Redis containers.

**Produces:** A GreenCloud `new-api` production stack with restored user/quota/channel/Option data and a verified CPA channel endpoint. Redis starts empty unless the inventory proves a durable business-state dependency.

- [ ] **Step 1: Produce a rehearsal dump and restore it before final cutover.**

  On GCP, export the application database from the production PostgreSQL container without emitting credentials:

  ```bash
  sudo docker exec postgres sh -c \
    'exec env PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -U "$POSTGRES_USER" -Fc -d "$POSTGRES_DB"' \
    > new-api-rehearsal.dump
  pg_restore -l new-api-rehearsal.dump > new-api-rehearsal.toc
  shasum -a 256 new-api-rehearsal.dump new-api-rehearsal.toc
  ```

  Expected: the dump and table-of-contents are checksummed and encrypted before transfer. A Redis dump is deliberately omitted unless Task 1 records a business-critical persistent keyspace.

- [ ] **Step 2: Restore into a fresh target PostgreSQL volume.**

  Start only target PostgreSQL and Redis, then restore:

  ```bash
  docker compose -f /srv/new-api/compose.yaml up -d postgres redis
  docker compose -f /srv/new-api/compose.yaml exec -T postgres sh -c \
    'exec env PGPASSWORD="$POSTGRES_PASSWORD" pg_restore -U "$POSTGRES_USER" --clean --if-exists -d "$POSTGRES_DB"' \
    < new-api-rehearsal.dump
  ```

  Expected: `pg_restore` exits zero; database object counts and dump table of contents match the source export.

- [ ] **Step 3: Preserve cryptographic continuity.**

  Put the existing `SESSION_SECRET` and any verified application encryption secret into `/srv/new-api/env/new-api.env` with mode `0600`. Confirm that restored `Channel` records can be decrypted by the application and that the `Option` entries holding Turnstile configuration are present. Never export these values into the report.

  Expected: administrator login behavior matches the source, existing channel credentials decrypt, and Turnstile validation is exercised against the same site configuration after Cloudflare staging is available.

- [ ] **Step 4: Start new-api and point its CPA channel at the host gateway.**

  Start `new-api`, leave its port loopback-only, and update only the CPA channel endpoint in the restored GreenCloud database/admin UI to `http://host.docker.internal:8317`. Do not alter the GCP channel during rehearsal.

  Expected: from GreenCloud, `/api/status` is healthy, `/v1/models` authenticates, a request reaches CPA, relay streaming works, and billing/error logs are written under `/srv/new-api/logs`.

- [ ] **Step 5: Exercise the source-compatible application acceptance suite.**

  Check administrator login, current user API keys, user quota/used quota, model/group permissions, channel selection, a non-stream request, an SSE request, request/billing logs, and a deliberately invalid key. Capture only request IDs and status codes in the report.

  Expected: all listed behaviors work against GreenCloud while Cloudflare still routes production traffic to GCP.

- [ ] **Step 6: Make the final data export only inside the cutover freeze.**

  At the approved maintenance window, block administrative writes/new top-ups on GCP, stop `new-api` cleanly, take a final custom-format PostgreSQL dump, checksum/encrypt/transfer/restore it using Steps 1–2, and record the source and target dump checksums. Restart GCP `new-api` only if the migration is aborted; otherwise keep it stopped but intact for rollback.

  Expected: no writes can diverge between final source dump and GreenCloud production. A failed checksum or restore is an immediate rollback to the still-running/previous GCP application, before any Cloudflare route change.

- [ ] **Step 7: Commit scripts and the redacted restore report.**

  ```bash
  git add ops/greencloud/compose.yaml ops/greencloud/scripts docs/reports/2026-07-10-new-api-data-restore-validation.md
  git commit -m "ops: add GreenCloud data migration validation"
  ```

## Task 6: Stage and switch the Cloudflare public entry point

**Files:**

- Modify: `ops/greencloud/compose.yaml`
- Create: `ops/greencloud/env/cloudflared.env.example`
- Create: `docs/reports/2026-07-10-cloudflare-cutover-runbook.md`

**Consumes:** A fully accepted GreenCloud application, the Cloudflare inventory from Task 1, and the exact production hostname recorded there.

**Produces:** A named Cloudflare Tunnel that routes the public hostname to the GreenCloud `new-api` container, with the former GCP route preserved for rollback.

- [ ] **Step 1: Load the prepackaged cloudflared image and start a named Tunnel connector.**

  The `cloudflared` Compose service uses the locally loaded digest-pinned image, has no published port, joins `new-api-backend`, and connects to `http://new-api:3000`. Store its token only in `/srv/new-api/env/cloudflared.env` (mode `0600`), and use `tunnel --no-autoupdate run --token ${TUNNEL_TOKEN}`.

  Expected: Cloudflare shows a connected replica, while `ss -ltn` on GreenCloud still has no public application listener.

- [ ] **Step 2: Validate Tunnel routing before DNS cutover.**

  Validate the ingress configuration and test the exact production hostname/rule order. The final ingress rule must reject unmatched traffic with `http_status:404`; no wildcard or path rule may accidentally route to CPA.

  Expected: Cloudflare can reach `new-api`; `/api/status`, authenticated API traffic, and SSE all work through the staging path.

- [ ] **Step 3: Reapply Cloudflare protections deliberately.**

  Compare source and target behavior for DNS proxying, TLS mode, WAF/rate-limit rules, Access policies, bot controls, cache rules, and Turnstile. Keep Turnstile site/key configuration unchanged unless the source inventory proves it is hostname-bound and needs an approved update.

  Expected: public behavior is equivalent or deliberately improved; CPA, PostgreSQL, Redis, SSH, and keeper endpoints have no public Cloudflare hostname.

- [ ] **Step 4: Perform the approved DNS/Tunnel cutover.**

  Make the externally visible Cloudflare hostname change only after Tasks 4–5 pass, final database export has restored successfully, and the user approves the exact hostname/maintenance window. Keep the old GCP origin/Tunnel record available but inactive rather than deleting it.

  Expected: production requests, including a long SSE request, reach GreenCloud; logs show the source container is no longer receiving new public traffic.

- [ ] **Step 5: Verify with a public acceptance matrix.**

  | Check | Required result |
  | --- | --- |
  | Tunnel state | Connected named Tunnel with an active GreenCloud replica |
  | HTTPS and API | `GET /api/status`, authenticated `/v1/models`, and a normal completion return expected success results |
  | Streaming | SSE remains connected through completion and emits expected events |
  | Turnstile | Login/registration path validates as it did before cutover |
  | Data isolation | `5432`, `6379`, and `8317` are not reachable from the public Internet |
  | CPA integration | A request selected through the CPA channel succeeds and appears in the expected application/CPA logs |

- [ ] **Step 6: Commit the Tunnel topology and runbook without credentials.**

  ```bash
  git add ops/greencloud/compose.yaml ops/greencloud/env/cloudflared.env.example docs/reports/2026-07-10-cloudflare-cutover-runbook.md
  git commit -m "ops: document Cloudflare Tunnel cutover for GreenCloud"
  ```

## Task 7: Operate the rollback window, then retire GCP deliberately

**Files:**

- Create: `ops/greencloud/scripts/backup-restore-check.sh`
- Create: `docs/reports/2026-07-17-greencloud-migration-closure.md`

**Consumes:** A successful public cutover and the preserved GCP stack.

**Produces:** A tested backup/restore record and an explicit go/no-go decision for GCP retirement.

- [ ] **Step 1: Run GreenCloud backup and restore rehearsal within 24 hours.**

  Encrypt a PostgreSQL custom-format backup and CPA state archive, transfer a copy to the approved off-host backup destination, and restore both into a disposable location/database. Compare checksums, database object counts, and CPA service startup.

  Expected: restore succeeds without GCP access. The report records backup locations by identifier only, not path credentials.

- [ ] **Step 2: Monitor for seven full days.**

  Monitor GreenCloud CPU/RAM/disk, Docker container restart counts, CPA restart count/auth refresh failures, keeper alert delivery, Tunnel connected-replica state, API error rate, SSE completion rate, and data backup freshness. Treat VPS jitter, memory pressure, persistent Tunnel disconnects, or CPA credential refresh failures as rollback signals.

  Expected: no unresolved production regression for seven days; GCP remains untouched and restorable throughout.

- [ ] **Step 3: Execute rollback if a cutover gate fails.**

  Restore the Cloudflare hostname to the preserved GCP origin/Tunnel first, verify GCP `/api/status`, `/v1/models`, and SSE, then re-enable GCP write traffic. Do not attempt bidirectional database reconciliation during an incident; retain the GreenCloud database as a forensic snapshot and choose one write authority before resuming service.

  Expected: traffic returns to the known GCP stack before any repair work occurs.

- [ ] **Step 4: Retire GCP only after explicit approval.**

  After the seven-day window and a successful GreenCloud restore drill, present the GCP resource list, snapshots, Artifact Registry retention decision, and estimated savings for approval. Delete nothing until that approval is recorded.

  Expected: migration closure report names the approved resources and deletion order; it never assumes that a snapshot from 2026-05-19 is a current CPA backup.

- [ ] **Step 5: Commit the closure evidence.**

  ```bash
  git add ops/greencloud/scripts/backup-restore-check.sh docs/reports/2026-07-17-greencloud-migration-closure.md
  git commit -m "ops: record GreenCloud migration closure criteria"
  ```

## Cutover decision gates

| Gate | Must be true before proceeding | If false |
| --- | --- | --- |
| Source safety | CPA key rotated; source inventory is redacted; test database isolation is known | Stop. Do not export or copy a possibly compromised/ambiguous configuration. |
| Offline supply chain | Every target image and CPA binary has local provenance, architecture validation, and a matching checksum | Stop. Do not build or pull on GreenCloud. |
| Target hardening | Docker installed; UFW active; only SSH publicly reachable; data/CPA ports are closed | Stop. Fix host controls first. |
| CPA | Host-native service works as non-root where compatible, binds loopback only, normal/SSE/keeper tests pass | Stop. Do not expose a broken or public CPA. |
| Data | Rehearsal restore passes; crypto continuity proven; final dump checksums match | Stop. Keep GCP authoritative. |
| Cloudflare | Named Tunnel connected; exact hostname/policies known; public SSE and Turnstile tests pass | Stop. Do not change DNS. |
| Cutover | User has approved the exact hostname and maintenance window | Stop. No external route change. |
| Retirement | Seven days clean plus restore drill and explicit approval | Retain GCP. |

## Self-review

- Scope coverage: `new-api`, production PostgreSQL, Redis, host-native CPA, CPA PostgreSQL/local state, `cpacodexkeeper`, Cloudflare Tunnel/DNS/protections/Turnstile, local-only image supply chain, GCP rollback, and separate `new-api-test` isolation are covered by Tasks 1–7.
- Safety coverage: the accidental CPA credential exposure is treated as a mandatory rotation gate; no secret-bearing command writes values to the repository or report; CPA/PostgreSQL/Redis have explicit non-public acceptance checks.
- Execution coverage: each phase has a concrete owner boundary, source/target paths, commands or exact validation behavior, a success condition, and an abort/rollback boundary. Cloudflare cutover is explicitly held for the required user approval because it is externally visible.
