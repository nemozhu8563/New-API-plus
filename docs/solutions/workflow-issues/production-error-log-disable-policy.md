---
title: Tune automatic channel disable rules from production error-log evidence
date: 2026-06-01
category: workflow-issues
module: production channel operations
problem_type: workflow_issue
component: development_workflow
severity: medium
applies_when:
  - Reviewing whether upstream error status codes should automatically disable channels
  - Investigating user reports where a conversation only recovers after starting a new session
  - Deciding whether an upstream failure should disable, pause, cool down, retry, or only alert
tags: [new-api, production-ops, channel-disable, error-logs, upstream-routing, status-reason, channel-affinity]
---

# Tune automatic channel disable rules from production error-log evidence

## Context

Production operators saw user-facing failures where an existing Codex conversation became unusable while a new conversation worked. Code inspection showed one contributing path: channel affinity can pin a session to a channel, and when that channel is later disabled the rule can stop retrying instead of selecting a fresh channel.

The immediate question was whether the production automatic-disable configuration was too broad. At the time, the monitoring settings used:

```text
AutomaticDisableStatusCodes = 401,429,502-503
AutomaticDisableChannelEnabled = true
AutomaticEnableChannelEnabled = true
```

Session history found prior incidents with the same diagnostic shape: a screenshot or user report was only explainable after tracing the request into production `logs`, `channels.other_info.status_reason`, and container logs. One earlier 502 stream-end incident showed `channel_id=18` being auto-disabled after `Upstream stream ended without a terminal response event` (session history). Another 503 incident showed a single upstream channel outage rather than a whole-site failure (session history).

For the May 2026 review, production logs were queried for `2026-05-01 00:00:00` through `2026-06-01 00:00:00` in Asia/Shanghai time. The month contained 2169 error logs across 19 channels and 10 models.

Status-code distribution was:

| Status | Count | Channel count | Operational reading |
| --- | ---: | ---: | --- |
| 503 | 556 | 7 | Mostly transient upstream/provider availability or auth-pool exhaustion |
| 502 | 482 | 11 | Mostly upstream gateway/stream failures |
| 429 | 377 | 6 | Mostly rate, concurrency, cooling, or quota-window limits |
| 500 | 339 | 10 | Mixed provider/gateway errors |
| 400 | 277 | 9 | Request compatibility or client parameter errors |
| 524 | 85 | 6 | Cloudflare/proxy timeout |
| 408 | 35 | 13 | Monitor timeout threshold |
| 504 | 7 | 2 | Gateway timeout |
| 403 | 4 | 3 | Mixed: credential scope, account balance, safety rejection |
| 202 | 4 | 1 | Async task response misclassified as error |
| 401 | 1 | 1 | Invalidated authentication token |
| 404 | 1 | 1 | Route or compatibility issue |
| 507 | 1 | 1 | Retry buffer limit |

That evidence showed status-code-only disable rules were too blunt. Most May errors were not proof that a channel credential was permanently bad.

## Guidance

Use content-based classification before deciding whether to disable a channel. Treat broad gateway and load errors as transient unless repeated failures prove otherwise.

### Hard-disable signals

These indicate a credential, account, or route is likely unusable until an operator changes something:

```text
Your authentication token has been invalidated
Insufficient account balance
Your credit balance is too low
This organization has been disabled.
Your account is not authorized
Upstream access forbidden
auth_not_found: no auth available
auth_unavailable: no auth available
```

For the current production behavior, a conservative automatic-disable setting is:

```text
AutomaticDisableStatusCodes = 401
```

Keep hard-disable keywords focused on account, credential, or route-unavailable failures. The May evidence supports disabling for:

- `401, Your authentication token has been invalidated`
- `403, Insufficient account balance`
- `500/502, Upstream access forbidden, please contact administrator`
- `503, auth_not_found: no auth available`
- `503, auth_unavailable: no auth available`

### Pause or cool down instead of disabling

These indicate a temporary or windowed limit. They should not permanently disable a channel when the system has no automatic expiry for that state:

```text
daily usage limit exceeded
weekly usage limit exceeded
Upstream rate limit exceeded
All credentials for model ... are cooling down
Concurrency limit exceeded
Too many pending requests
The usage limit has been reached
```

In May, `429` appeared 377 times. Most of it was rate/concurrency/cooling behavior, not a dead key. If a provider returns a daily or weekly limit, prefer a timed pause that expires at the next quota window. If the application only supports enabled/disabled, do not include these phrases in permanent auto-disable keywords unless manual recovery is acceptable.

### Retry, switch channel, or degrade instead of disabling

Do not use these as direct disable status codes:

```text
408
500
502
503
504
507
524
```

In May, `502 + 503` accounted for 1038 errors. They included:

- `Upstream service temporarily unavailable`
- `Upstream request failed`
- `bad response status code 502`
- `Service temporarily unavailable`
- `The origin web server returned an invalid or incomplete response to Cloudflare`
- `The origin web server did not return a complete response within the 120-second Proxy Read Timeout window`
- `Upstream stream ended without a terminal response event`

These are operationally important, but they are usually availability signals. They should drive retry, fallback, alerting, short-term channel penalty, or repeated-failure thresholds. They should not automatically and permanently disable a channel on first occurrence.

### Do not disable for request compatibility errors

These errors usually belong to request transformation, model capability, client behavior, or an async-provider contract:

```text
400
404
202
```

May examples included unsupported parameters, invalid file data, model capability mismatch, image endpoint mismatch, and HTTP `202` async image task responses. Disabling a channel for these hides the real integration problem.

## Why This Matters

Over-broad automatic disable rules can turn transient upstream instability into customer-visible outages. The damaging path is:

1. A channel sees one `502` or `503`.
2. The status code rule disables the channel.
3. Channel affinity keeps existing sessions pinned to the disabled channel.
4. Retry is skipped by rule, so the user sees a hard 403-style failure.
5. Starting a new conversation works because it gets a new affinity key or a different route.

That failure mode makes the service look unreliable even when healthy alternate channels exist.

The durable operational distinction is:

- **Disable** only when the credential, account, or provider route is unusable without operator action.
- **Pause/cool down** when quota or concurrency may recover by time.
- **Retry/fallback/degrade** when the error is a gateway, network, timeout, stream, or provider availability issue.
- **Fix request mapping** when the error is a `400`, `404`, or provider-specific compatibility response.

## When to Apply

- A production screenshot includes a request id, a status code, or a `Please contact the administrator` message.
- A channel is auto-disabled and `channels.other_info.status_reason` shows `502`, `503`, `504`, `524`, or timeout text.
- A user reports that restarting a conversation fixes the problem.
- You are changing `AutomaticDisableStatusCodes` or `AutomaticDisableKeywords`.
- You are deciding whether to re-enable channels after an upstream incident.

## Examples

Use the production database first, then container logs only for confirmation. The most useful starting point is aggregate classification, not broad `docker logs`.

```sql
with may_logs as (
  select
    *,
    to_timestamp(created_at) at time zone 'Asia/Shanghai' as cst,
    coalesce(other::jsonb->>'status_code', '') as status_code,
    lower(content) as lc
  from logs
  where type = 5
    and created_at >= extract(epoch from timestamp with time zone '2026-05-01 00:00:00+08')::bigint
    and created_at < extract(epoch from timestamp with time zone '2026-06-01 00:00:00+08')::bigint
),
classified as (
  select
    *,
    case
      when lc like '%invalidated%'
        or lc like '%invalid token%'
        or lc like '%authentication token%' then 'hard_disable_auth'
      when lc like '%insufficient account balance%'
        or lc like '%credit balance is too low%'
        or lc like '%organization has been disabled%'
        or lc like '%account is not authorized%' then 'hard_disable_account'
      when lc like '%upstream access forbidden%'
        or lc like '%permission denied%'
        or lc like '%operation not allowed%' then 'hard_disable_forbidden'
      when lc like '%no auth available%'
        or lc like '%auth_not_found%'
        or lc like '%auth_unavailable%' then 'pause_or_disable_no_auth_available'
      when lc like '%daily_limit_exceeded%'
        or lc like '%weekly_limit_exceeded%'
        or lc like '%daily usage limit exceeded%'
        or lc like '%weekly usage limit exceeded%'
        or lc like '%7天限额已用完%' then 'pause_quota_window'
      when lc like '%rate limit%'
        or lc like '%cooling down%'
        or lc like '%concurrency limit%'
        or lc like '%too many pending%'
        or lc like '%usage limit has been reached%' then 'cooldown_only'
      when status_code in ('500','502','503','504','524','408','507') then 'transient_no_disable'
      when status_code in ('400','404','202') then 'request_or_compat_no_disable'
      else 'review'
    end as bucket
  from may_logs
)
select
  bucket,
  count(*) as n,
  count(distinct channel_id) as channels,
  string_agg(distinct status_code, ',' order by status_code) as codes,
  max(cst) as last_cst
from classified
group by bucket
order by n desc;
```

The May 2026 result:

| Bucket | Count | Meaning |
| --- | ---: | --- |
| `transient_no_disable` | 953 | Retry/fallback/degrade; do not hard-disable |
| `pause_or_disable_no_auth_available` | 520 | Credentials unavailable; disable only if this means operator action is required |
| `cooldown_only` | 301 | Rate/concurrency/cooling; do not hard-disable |
| `request_or_compat_no_disable` | 282 | Fix request mapping or provider contract |
| `pause_quota_window` | 76 | Pause until quota window resets |
| `hard_disable_forbidden` | 32 | Disable or route away until operator fixes access |
| `hard_disable_auth` | 1 | Disable until token is refreshed |
| `hard_disable_account` | 1 | Disable until balance/account is fixed |

After tuning settings, clear channel-affinity cache when old sessions may still be pinned to disabled channels:

```http
DELETE /api/option/channel_affinity_cache?rule_name=codex%20cli%20trace
```

or, if all affinity state is safe to reset:

```http
DELETE /api/option/channel_affinity_cache?all=true
```

Then recover disabled channels one by one. Do not bulk-enable channels that were disabled by `502` or `503` without testing them, because the upstream may still be unstable.

## Related

- `docs/solutions/integration-issues/channel-test-503-no-available-accounts-2026-04-20.md` - separates upstream account-pool errors from local new-api errors.
- `docs/solutions/logic-errors/codex-tag-routing-overrides-channel-priority-2026-04-30.md` - shows why logs should be checked for route selection facts before blaming priority, weight, or affinity.
- `docs/solutions/best-practices/header-based-cc-routing-with-single-key-2026-04-22.md` - explains that channel affinity is not a group-selection mechanism.
