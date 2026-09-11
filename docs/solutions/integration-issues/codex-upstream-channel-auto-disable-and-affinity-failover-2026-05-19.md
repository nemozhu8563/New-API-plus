---
title: Codex upstream channel auto-disable and affinity failover do not trip when production upstream degrades
date: 2026-05-19
category: integration-issues
module: channel-routing
problem_type: integration_issue
component: service_object
symptoms:
  - "Codex usage logs showed channel 28 returning upstream 429, 500, and 502 errors while the app health check stayed green."
  - "Channel 28 kept receiving successful and failed traffic instead of being automatically removed from rotation."
  - "Production options had AutomaticDisableChannelEnabled=false even though channel 28 had auto_ban=1."
  - "Default Codex channel affinity can pin /v1/responses traffic to a preferred channel and skip retry on affinity failure."
root_cause: config_error
resolution_type: config_change
severity: high
tags: [new-api, sub2api-prod, codex, channel-routing, automatic-disable, channel-affinity, upstream-502, rate-limit]
---

# Codex upstream channel auto-disable and affinity failover do not trip when production upstream degrades

## Problem

On `sub2api-prod`, Codex requests routed through channel `28` (`ask-XToken-codex-周卡-1`) were seeing upstream failures such as:

```text
status_code=429, Upstream rate limit exceeded, please retry later
status_code=500, invalid character 'e' looking for beginning of value
status_code=502, Upstream service temporarily unavailable
status_code=502, Upstream request failed
```

The gateway process itself was healthy: the `new-api` container was up, `/api/status` returned successfully inside the container, and other Codex-capable channels were serving traffic. The failure was an upstream/channel-routing problem, not a local process outage.

## Symptoms

- Channel `28` produced a large 24-hour error cluster: mostly upstream `502`, plus `500` bad response bodies and `429` rate-limit responses.
- For the same Codex `gpt-5.4`/`gpt-5.5` lane, channel `28` had a high error share while channel `1` had successful traffic and no matching errors in the sampled window.
- The screenshot-time window still showed successes mixed with failures, which means the channel was not completely down; it was degraded.
- The channel had `auto_ban=1`, but the global production option `AutomaticDisableChannelEnabled` was `false`, so the automatic disable path was effectively off.

## What Didn't Work

- **Relying on `auto_ban=1` alone.** `auto_ban` is only checked after `service.ShouldDisableChannel()` returns true. If `AutomaticDisableChannelEnabled=false`, the disable decision short-circuits before the per-channel flag matters.
- **Expecting auto-test to fix live traffic.** The monitor auto-test can re-enable auto-disabled channels when configured, but it skips manually disabled channels and it does not replace live-request circuit breaking.
- **Only looking at container health.** `/api/status` proves the gateway is alive. It does not prove every upstream channel in a weighted pool is healthy.
- **Assuming priority and weight are dynamic load balancing.** Priority and weight choose among currently eligible channels. They do not automatically reduce traffic for a degraded channel unless another mechanism disables it or lowers its weight.

## Solution

Use three layers together: automatic disable for clear upstream failures, retry/failover for a single bad draw, and conservative channel weights for known-risk upstreams.

### 1. Turn on automatic disable for live channel errors

Production had this disabled:

```text
AutomaticDisableChannelEnabled=false
```

Set it to true for production if the goal is to stop sending traffic to failing upstream channels:

```text
AutomaticDisableChannelEnabled=true
```

Keep `auto_ban=1` on channels that are safe to auto-remove from rotation. A channel with `auto_ban=0` will still log the failure, but it should not be automatically disabled.

### 2. Include the upstream statuses that should trip the breaker

Production only had:

```text
AutomaticDisableStatusCodes=401,429
```

That catches authentication and rate-limit cases, but it does not catch the observed `502` upstream service failures. For this Codex lane, use:

```text
AutomaticDisableStatusCodes=401,429,502,503
```

Consider adding `500` only if the provider's `500` responses are consistently upstream-fatal for the channel. In this incident, the `500 invalid character 'e'...` rows looked like a malformed upstream response, but `500` can also be transient or provider-specific, so adding it is a stronger operational choice.

### 3. Allow retry/failover when channel affinity picks a bad channel

The default Codex CLI trace affinity rule for `/v1/responses` can pin traffic by `prompt_cache_key`. That helps cache locality, but the default `skip_retry_on_failure=true` means an affinity-selected bad channel can prevent the normal retry path.

For production failover, set the Codex affinity rule to allow retry on failure:

```text
channel_affinity_setting.rules:
  - name: codex_cli_trace
    skip_retry_on_failure: false
```

Keep success recording enabled so a later successful fallback can become the preferred channel:

```text
channel_affinity_setting.switch_on_success=true
```

### 4. Configure at least one retry

Retry/failover only helps if the relay is allowed to try another eligible channel:

```text
RetryTimes=1
```

Use `1` as the first production step. Increase only if the upstream pool is broad enough and the added latency is acceptable.

### 5. Lower blast radius for unstable upstreams

Channel selection first filters by group/model/tag/status, then chooses a priority bucket, then applies weight within that bucket. In the inspected production state, channel `28` had the same high priority as channel `1` but a larger weight:

```text
channel 28: priority=10, weight=70
channel 1:  priority=10, weight=30
```

When a provider is showing intermittent upstream errors, reduce its weight or move it to a lower priority until it proves stable. Weight is not a health signal; it is only a traffic share signal inside an already eligible pool.

## Useful Production Checks

Check the options that decide disable/retry/affinity behavior:

```sql
select key, value
from options
where key in (
  'AutomaticDisableChannelEnabled',
  'AutomaticDisableStatusCodes',
  'AutomaticDisableKeywords',
  'AutomaticEnableChannelEnabled',
  'RetryTimes',
  'channel_affinity_setting.rules',
  'channel_affinity_setting.switch_on_success'
)
order by key;
```

Check the candidate channels for the affected Codex lane:

```sql
select id, name, status, auto_ban, priority, weight, "group", models, tag, base_url
from channels
where id in (1, 18, 28)
order by priority desc, weight desc, id;
```

Aggregate recent channel errors by status:

```sql
select channel_id,
       other::jsonb->>'status_code' as status_code,
       other::jsonb->>'code' as error_code,
       count(*) as errors
from logs
where type = 5
  and created_at >= extract(epoch from now() - interval '24 hours')::bigint
  and channel_id in (1, 18, 28)
group by channel_id, status_code, error_code
order by errors desc;
```

Compare errors and successes for the same group/model lane:

```sql
select channel_id,
       sum(case when type = 5 then 1 else 0 end) as errors,
       sum(case when type <> 5 then 1 else 0 end) as successes
from logs
where created_at >= extract(epoch from now() - interval '24 hours')::bigint
  and "group" = 'Codex'
  and model_name in ('gpt-5.4', 'gpt-5.5')
  and channel_id in (1, 18, 28)
group by channel_id
order by errors desc, successes desc;
```

## Why This Works

The relevant runtime flow is:

1. `controller/relay.go` logs channel errors and calls `service.ShouldDisableChannel(err)`.
2. `service/channel.go` returns false immediately when `AutomaticDisableChannelEnabled=false`.
3. If automatic disable is enabled, it checks channel error classification, configured status codes, status ranges, and keywords.
4. `controller/relay.go` only calls `service.DisableChannel()` if the error should disable the channel and the channel has `auto_ban=1`.
5. `controller/relay.go` retry logic can be bypassed when channel affinity says the preferred channel failure should not be retried.
6. `model/channel_cache.go` only applies priority and weight after filtering to currently enabled and eligible channels.

So the fix is configuration, not a restart: let live upstream failures disable eligible bad channels, let retry escape a bad affinity pick, and keep unstable channels from dominating the weighted pool.

## Prevention

- Treat `AutomaticDisableChannelEnabled=false` as a deliberate production risk for any paid or third-party upstream pool.
- Include upstream `502` and `503` in the disable list for providers that use them to mean account-pool or service unavailability.
- Keep manually disabled channels manual: the monitor auto-test skips them. Use auto-disabled status when the expected recovery path is automatic test and re-enable.
- For Codex `/v1/responses`, review affinity behavior whenever cache locality conflicts with failover reliability.
- Alert on channel-level error percentage, not only process health. A healthy gateway can still route too much traffic to a degraded upstream.

## Related Issues

- [Codex tag routing overrides channel priority when backup channels are untagged](../logic-errors/codex-tag-routing-overrides-channel-priority-2026-04-30.md)
- [Channel test returns 503 "No available accounts" from aigocode upstream](channel-test-503-no-available-accounts-2026-04-20.md)
