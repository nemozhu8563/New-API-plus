---
title: Use Cloud Build and Artifact Registry when new-api must be pushed to GCloud without building on the target server
date: 2026-04-24
category: workflow-issues
module: gcloud image delivery
problem_type: workflow_issue
component: tooling
severity: medium
applies_when:
  - You need to prepare a new-api image for a GCE-hosted production update but must not build on the target VM
  - Local Docker Desktop is unhealthy or cannot talk to the Docker engine
  - The GCP project does not yet have Cloud Build or Artifact Registry prepared
  - You want the server rollout step to stay pull-only instead of source-upload-plus-build
tags: [new-api, gcp, cloud-build, artifact-registry, docker, sub2api-prod, image-push, remote-build]
symptoms:
  - `docker info` and `docker version` fail locally with `Bad response from Docker engine`
  - The operator explicitly forbids building the image on the production server
  - The initial `gcloud builds submit` run can fail on missing APIs, missing repository setup, service-account IAM, or `.gcloudignore` omissions
root_cause: missing_tooling
resolution_type: workflow_improvement
---

> 历史记录（已弃用，2026-07-31）：本文描述的 GCP Cloud Build / Artifact Registry 发布路径已完全停用。不得执行其中的 `gcloud`、`sub2api-prod` 或 GCP 镜像发布命令；当前部署请遵循 [GreenCloud 服务迁移 SOP](../../operations/greencloud-service-migration-sop.md)。

# Use Cloud Build and Artifact Registry when new-api must be pushed to GCloud without building on the target server

## Context
The goal was to package the current `new-api` service image locally from the checked-out repo and push it into GCloud so the production environment could be updated later. A hard constraint was that the image must **not** be built on the target server.

During execution, the workstation's Docker Desktop backend was unhealthy: `docker version` and `docker info` could reach the client but the engine returned `Bad response from Docker engine`, and Docker diagnostics also reported WSL-related filesystem issues. That ruled out a normal local `docker build`, but it still did not justify falling back to a server-side build because the deployment constraint was explicit.

## Guidance
When local Docker is broken but the image still must not be built on the target VM, use **GCP Cloud Build + Artifact Registry** as the build lane:

1. Keep the build source on the workstation, but submit it to Cloud Build instead of invoking local Docker.
2. Enable the required APIs in the target project:
   - `cloudbuild.googleapis.com`
   - `artifactregistry.googleapis.com`
3. Create a Docker-format Artifact Registry repository in the deployment region if it does not already exist.
4. Make sure the Cloud Build execution identity can:
   - read the Cloud Build staging bucket
   - push to the Artifact Registry repository
5. Be careful with `.gcloudignore`: by default it can inherit `.gitignore`, which may accidentally exclude build-critical files.
6. Submit the repo snapshot to Cloud Build with a versioned tag, then deploy later by pulling that immutable image on the server.

For this run, the successful image was:

- `us-west1-docker.pkg.dev/stalwart-elixir-490811-q6/new-api/new-api:prod-20260424-212750-8acb17f5cd84`
- digest: `sha256:c03884daf1d25a81d7b8806ea5f9b1e50d49af23a8491024759c73492264247f`
- Cloud Build ID: `b613f2e1-d08f-4446-ae8e-f9cba8b246d9`

The concrete prep steps that were needed in this project were:

- enable the two GCP APIs
- create Artifact Registry repo `new-api` in `us-west1`
- grant `roles/storage.objectViewer` on `gs://stalwart-elixir-490811-q6_cloudbuild` to `888780077871-compute@developer.gserviceaccount.com`
- grant `roles/artifactregistry.writer` on the `new-api` repo to the same service account
- add a custom `.gcloudignore` for the submitted temp source tree so `web/bun.lock` was not excluded via inherited `.gitignore`

## Why This Matters
This path preserves the important operational boundary: the server only needs to **pull and run** a finished image. It does not need the source tree, frontend toolchain, Go toolchain, or an ad hoc build workflow.

That reduces rollout risk in three ways:

- it respects the "do not build on the server" constraint exactly
- it avoids turning a production VM into an emergency build box
- it produces an immutable registry artifact that can be pinned by tag or digest during rollout and rollback

It also surfaces a useful GCP-specific lesson: the first failure is often not application code but project plumbing — disabled APIs, missing repository creation, service-account IAM, or source filtering through `.gcloudignore`.

## When to Apply
- You are updating a GCE-hosted service and want the host to pull a ready-made image only
- Local Docker is failing, but GCP access is healthy
- You need a reproducible image reference before touching production
- You want a cleaner alternative to `git archive` + upload source + build on the VM

## Examples
### Validation before choosing the build lane

```bash
docker version
docker info
```

If the client works but the daemon returns engine errors, stop treating local Docker as trustworthy for release work.

### GCP preparation

```bash
gcloud services enable cloudbuild.googleapis.com artifactregistry.googleapis.com \
  --project stalwart-elixir-490811-q6

gcloud artifacts repositories create new-api \
  --repository-format=docker \
  --location=us-west1 \
  --description="new-api images" \
  --project stalwart-elixir-490811-q6
```

### Required IAM for the build identity used in this project

```bash
gcloud storage buckets add-iam-policy-binding gs://stalwart-elixir-490811-q6_cloudbuild \
  --member="serviceAccount:888780077871-compute@developer.gserviceaccount.com" \
  --role="roles/storage.objectViewer" \
  --project stalwart-elixir-490811-q6

gcloud artifacts repositories add-iam-policy-binding new-api \
  --location=us-west1 \
  --member="serviceAccount:888780077871-compute@developer.gserviceaccount.com" \
  --role="roles/artifactregistry.writer" \
  --project stalwart-elixir-490811-q6
```

### Source filtering fix

A local `.gitignore` in this repo excludes `web/bun.lock`. Because `gcloud builds submit` can derive ignores from `.gitignore`, the first remote build failed at:

```text
COPY web/bun.lock .
COPY failed: file not found in build context or excluded by .dockerignore
```

The fix was to put a minimal `.gcloudignore` into the temporary source tree used for submission:

```text
.gcloudignore
.git
.gitignore
.omx
```

That kept the repo metadata out of the upload while still allowing `web/bun.lock` into the Cloud Build context.

### Build and publish

```bash
gcloud builds submit <temp-source-dir> \
  --tag us-west1-docker.pkg.dev/stalwart-elixir-490811-q6/new-api/new-api:prod-20260424-212750-8acb17f5cd84 \
  --project stalwart-elixir-490811-q6 \
  --timeout=3600s
```

### Verification done for this run

```bash
go test ./...
bun run build
```

Both checks passed before the successful Cloud Build push.

### Cost notes

As checked against the official pricing pages on 2026-04-24:

- **Cloud Build** default-pool builds are billed per build-minute, with a monthly free tier before usage becomes billable.
- **Artifact Registry** cost is mainly storage plus any chargeable network egress; same-region pulls are the cheapest path.

For this specific run, the build duration was about **6 minutes 39 seconds**, so the direct Cloud Build cost is typically only a few US cents, and may still be covered by the monthly free tier. The steady-state Artifact Registry cost is usually negligible if you keep only a small number of recent images and pull them from resources in the same region.

That makes this lane operationally safer than server-side builds without materially changing deployment cost for low-frequency releases.

## Related
- `docs/solutions/workflow-issues/sub2api-prod-new-api-test-instance-deploy-2026-04-24.md` — related deployment lane on the same VM, but focused on parallel test-instance rollout and an older server-build fallback
- `docs/solutions/workflow-issues/stale-new-api-test-frontend-hides-tiered-billing-ui-2026-04-24.md` — related self-built image drift case on `sub2api-prod`
