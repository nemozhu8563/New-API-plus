---
date: 2026-05-15
topic: new-api-cpa-sub2api-migration-options
focus: Decide whether to migrate users and upstream account pools from new-api plus CPA to sub2api
---

# Ideation: new-api + CPA to sub2api Migration Options

## Grounding Context

- Current local architecture is `new-api + CPA`: new-api owns user accounts, user API keys, quota, groups, model limits, channel routing, billing logs, and admin dashboard operations.
- In new-api, `User` includes quota, used quota, request count, group, OAuth identities, and management access token. `Token` is user-bound and carries API key, remaining quota, unlimited quota, model limits, group, and cross-group retry. `Channel` owns upstream key/base URL/model/group/priority/weight/auto-ban and multi-key state.
- CPA is credential-pool oriented. Its local `Auth` model tracks provider, disabled/unavailable state, quota backoff, next recovery, and per-model state. It is very useful for OAuth/subscription account execution, but it is not the same thing as a multi-user commercial gateway.
- Sub2API publicly positions itself as a subscription quota distribution gateway. Its README says it handles platform-generated API keys, authentication, billing, load balancing, request forwarding, multi-account management, sticky sessions, user/account concurrency limits, request/token rate limits, payments, and an admin dashboard.
- CPA Management API docs currently emphasize config/auth-file management and per-request usage queue records. Legacy aggregate usage endpoints are documented as unavailable in favor of `/usage-queue` and Redis Usage Queue.

## Topic Axes

- User layer: whether existing users, balances, API keys, model limits, and groups can move cleanly.
- Upstream pool layer: whether CPA auth files and new-api channels map cleanly to sub2api accounts.
- Control ownership: which system should be the source of truth for billing, quota, routing, and observability.
- Cutover risk: how to avoid breaking existing clients and losing accounting history.
- Observability gain: whether the target shape makes active concurrency, account saturation, and failures clearer.

## Ranked Options

### 1. Keep new-api as user and billing front door, put sub2api underneath as the subscription pool

**Description:**
Run sub2api as an upstream provider behind new-api. In this shape, users keep using new-api keys and balances. new-api still owns user quota, payment, logs, and public API surface. sub2api owns the low-level subscription account pool, account selection, sticky sessions, and per-account concurrency limits.

**Why it matters:**
This gives the observability and scheduling win you actually want without forcing a risky user migration on day one. It also avoids running two competing customer-facing billing systems.

**Migration impact:**
Low to medium. You create one or more new-api channels that point at sub2api endpoints, then move only selected traffic groups/models onto those channels. Existing new-api users and keys do not need to change.

**What still needs validation:**
Sub2API must expose enough per-account/account-pool metrics for your real pressure questions: active requests, per-account concurrency, sticky-session hit, 429/502/503 by account/model/user, latency percentiles, and account cooldown/recovery.

**Status:** Recommended first move.

### 2. Full cutover: migrate users and upstream pools into sub2api

**Description:**
Make sub2api the public gateway. Users, API keys, balances, rate limits, payments, subscription accounts, and observability all live in sub2api. new-api and CPA are retired or kept only as legacy/internal tools.

**Why it matters:**
This is the cleanest final architecture if sub2api covers every feature you need. One system owns the whole control plane, so observability and throttling can line up with actual user and account usage.

**Migration impact:**
High. new-api users are not just names and emails; they have quota, used quota, groups, token-level quota, model limits, API keys, logs, top-up history, and route behavior. CPA auth records also carry provider-specific OAuth/runtime state, quota backoff, model state, labels, proxy settings, and metadata. Those are not obviously one-to-one with sub2api imports from public docs alone.

**What still needs validation:**
Before choosing this, inspect sub2api's actual DB schema/API surface and prove import paths for:

- users and passwords/OAuth identities
- existing API keys or key regeneration with client cutover
- balances and historical used quota
- groups/plans/model limits
- upstream OAuth accounts/API-key accounts
- sticky-session identifiers
- concurrency and rate-limit defaults

**Status:** Good north star, not safe as the first cut.

### 3. Keep new-api + sub2api permanently, retire CPA only for traffic that sub2api supports

**Description:**
Use new-api as the stable business/user layer. Move subscription-style upstream accounts from CPA into sub2api where possible. Keep CPA for provider/client cases where it has better native behavior or where migration is fragile.

**Why it matters:**
This matches the reality that CPA and sub2api overlap but are not identical. CPA is strong around CLIProxyAPI provider execution and local management; sub2api is stronger around subscription quota distribution and concurrency control.

**Migration impact:**
Medium. It creates a hybrid backend, but keeps user migration out of scope. The main work is channel mapping and operational dashboards.

**Downside:**
You still have multiple systems to observe. If you do this, you need one small cross-system dashboard or polling report, otherwise the blindness just moves from CPA to two backends.

**Status:** Most realistic if sub2api cannot import all CPA account types cleanly.

### 4. Keep CPA and add observability around it

**Description:**
Do not migrate yet. Enable CPA usage publishing, consume `/usage-queue`, and add runtime stats around active requests/streams/account selection. Keep new-api as-is.

**Why it matters:**
This is the smallest change and may be enough if the real pain is only "I cannot see pressure."

**Downside:**
It does not turn CPA into a user/subscription management system. You would still lack first-class user/account concurrency controls unless you build them or layer them elsewhere.

**Status:** Useful fallback, not the best strategic path if you already want sub2api's account-pool control.

## Rejected Ideas

- Directly importing new-api users into sub2api without a pilot: rejected because balances, keys, groups, token limits, and historical accounting have different ownership semantics.
- Running new-api and sub2api both as user-facing billing systems: rejected because double quota and double invoices make incident analysis harder, not easier.
- Replacing CPA with sub2api for every account type in one weekend: rejected because provider-specific OAuth/runtime state and client behavior need real traffic validation.

## Recommended Decision

Use `new-api -> sub2api -> subscription accounts` as the first production experiment.

Keep new-api as the user/key/balance front door. Let sub2api take over the bottom account-pool scheduling where it is strongest. Only consider full user migration after a pilot proves that sub2api can represent your current users, balances, API keys, group/model restrictions, upstream accounts, and logs without losing information.

## Pilot Checklist

1. Deploy sub2api separately with PostgreSQL and Redis.
2. Import or manually recreate 1-2 non-critical upstream subscription accounts.
3. Add sub2api as a new channel in new-api for one test group and one or two models.
4. Run real Codex/Claude/Gemini CLI traffic for 48-72 hours.
5. Compare new-api logs, sub2api account-level logs, and CPA behavior for the same traffic class.
6. Confirm whether sub2api can show or export active concurrency, account saturation, sticky session routing, cooldown, failure codes, and latency by account/model/user.
7. If the pilot passes, move more upstream accounts into sub2api. If it fails, keep CPA and add a usage queue dashboard.

## Migration Mapping

| Current object | Likely target | Migration difficulty | Notes |
| --- | --- | --- | --- |
| new-api user | sub2api user | High | Password/OAuth identity, group, balance, used quota, payment history, and role semantics need schema-level mapping. |
| new-api token/API key | sub2api API key | Medium to high | Existing clients may depend on stable keys; preserving old keys depends on sub2api import/API support. |
| new-api user quota | sub2api balance/quota | Medium | Units and rounding must match, especially if token-level billing differs. |
| new-api group/model limits | sub2api plan/rate/model rules | Medium | Needs policy mapping, not just data copy. |
| new-api channel | sub2api upstream account or provider | Medium | API-key channels may map; OAuth/subscription behavior needs provider-specific validation. |
| CPA auth file | sub2api upstream account | Medium to high | OAuth token format, refresh behavior, proxy settings, labels, and cooldown state may not be importable directly. |
| CPA usage queue | sub2api usage/account logs | Medium | Historical usage is usually better archived than imported unless sub2api has an explicit log import path. |

## Sources

- Sub2API README: https://github.com/Wei-Shaw/sub2api
- CLIProxyAPI Management API: https://help.router-for.me/management/api
- CLIProxyAPI Web UI: https://help.router-for.me/management/webui
- Local new-api models: `model/user.go`, `model/token.go`, `model/channel.go`
- Local CPA auth model: `sdk/cliproxy/auth/types.go`
