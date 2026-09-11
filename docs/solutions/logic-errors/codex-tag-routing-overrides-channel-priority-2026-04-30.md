---
title: Codex tag routing overrides channel priority when backup channels are untagged
date: 2026-04-30
category: logic-errors
module: channel-routing
problem_type: logic_error
component: service_object
symptoms:
  - "Codex traffic used channel 1 only, despite channel 14 having higher priority and weight."
  - "Usage logs showed selected_public_group=Codex and matched_tag=my-cpa-codex."
  - "Usage logs showed use_channel=[\"1\"], with channel 14 absent from the selected path."
  - "Production DB showed channels 1 and 14 were both enabled and shared the same groups and models."
  - "Production DB showed channel 1 tag=my-cpa-codex while channel 14 tag was empty."
root_cause: config_error
resolution_type: config_change
severity: medium
tags: [new-api, channel-routing, codex, channel-tag, priority, weight, abilities]
---

# Codex tag routing overrides channel priority when backup channels are untagged

## Problem

A production Codex route was expected to prefer channel `14` (`cpa-backup`) because it had higher priority and weight than channel `1` (`my-cpa-codex`). Instead, usage logs continued to show only channel `1`, including records without a channel-affinity marker.

The issue was not weight distribution and not primarily channel affinity. The channel selector had already narrowed the candidate pool to the resolved tag `my-cpa-codex`, and channel `14` was untagged.

## Symptoms

- The channels page showed channel `14` enabled with higher priority/weight than channel `1`.
- Consume logs for group `Codex` kept showing channel `1`.
- Logs with `affinity_rule = null` still selected channel `1`.
- Production log metadata showed:

  ```text
  selected_public_group = Codex
  matched_tag = my-cpa-codex
  use_channel = ["1"]
  ```

- Production DB inspection showed both channels were otherwise eligible by group and model:

  ```text
  channel 1: group=default,Codex,y, tag=my-cpa-codex, priority=0, weight=0
  channel 14: group=default,Codex,y, tag='', priority=2, weight=1
  ```

## What Didn't Work

- **Treating weight as the deciding factor.** Weight only matters after the selector has already filtered by group, model, enabled status, tag, and priority. A channel outside the resolved tag pool receives no weight draw.
- **Looking only at channel affinity.** Affinity explained some rows, but rows without affinity still had `matched_tag=my-cpa-codex`, which proved the route tag itself was narrowing selection.
- **Comparing channel priority on the channels page.** Priority is only compared among channels that survive the earlier tag filter. Channel `14` had higher priority but was not in the tagged candidate pool.
- **Relying on the channel test result.** A successful channel test proves the upstream can answer the test request; it does not prove the channel is included in the runtime route pool for a specific group/model/tag combination.

## Solution

Put the channels that should compete or fail over together into the same tag pool.

For this incident, if channel `14` should be the primary/backup for the same Codex lane as channel `1`, configure:

```text
channel 14 tag = my-cpa-codex
channel 1 tag  = my-cpa-codex
channel 14 priority > channel 1 priority
```

Then refresh the channel runtime data so `abilities` and in-memory channel cache reflect the new configuration.

Useful production checks:

```sql
select id, name, "group", status, priority, weight, tag, models
from channels
where id in (1, 14)
order by id;

select channel_id, "group", enabled, priority, weight, coalesce(tag, '') as tag,
       count(*) as model_count,
       string_agg(model, ', ' order by model) as models
from abilities
where channel_id in (1, 14)
group by channel_id, "group", enabled, priority, weight, tag
order by channel_id, "group", priority desc;
```

Useful log check:

```sql
select to_char(to_timestamp(created_at) at time zone 'Asia/Hong_Kong',
               'YYYY-MM-DD HH24:MI:SS') as hk_time,
       id, type, channel_id, "group", model_name, token_name,
       case when other <> '' then (other::jsonb #>> '{admin_info,use_channel}') end as use_channel,
       case when other <> '' then (other::jsonb #>> '{admin_info,channel_affinity,rule_name}') end as affinity_rule,
       case when other <> '' then (other::jsonb #>> '{selected_public_group}') end as selected_public_group,
       case when other <> '' then (other::jsonb #>> '{matched_channel_tag}') end as matched_tag
from logs
where created_at >= extract(epoch from now() - interval '2 hours')::bigint
  and "group" = 'Codex'
order by created_at desc
limit 80;
```

## Why This Works

Channel selection is staged. Priority and weight are late-stage controls, not global controls.

The relevant flow is:

1. `service.ResolveGroupBilling()` checks enabled tags for the public group and model.
2. If exactly one enabled tag exists and there is no explicit override, it sets `RouteTag`/`MatchedTag` to that tag.
3. `service.GetRandomSatisfiedChannelByResolution()` prefers `model.GetRandomSatisfiedChannel(group, model, routeTag, retry)` when a route tag candidate exists.
4. `model.GetRandomSatisfiedChannel()` resolves the channel list through `getChannelsByGroupModelTagUnlocked()`.
5. Only after that does it choose the target priority and then apply weight inside that priority.

So with this production state:

```text
tagged pool my-cpa-codex: channel 1
untagged pool: channel 14
```

the runtime candidate pool for `matched_tag=my-cpa-codex` contained only channel `1`. Channel `14` was filtered out before the selector reached priority or weight.

Aligning the tags makes the candidate pool:

```text
tagged pool my-cpa-codex: channel 14, channel 1
```

At that point priority can do what the operator expected: choose channel `14` first, and only fall back to lower-priority channels on retry or failure.

## Prevention

- When configuring a backup channel for a tagged route, copy the same tag onto every channel that should participate in that route.
- Use `other.admin_info.use_channel`, `selected_public_group`, and `matched_channel_tag` in logs before assuming affinity, weight, or priority is responsible.
- Remember the selection order: **group/model/tag first, priority second, weight third**.
- After changing channel tag/group/model/priority/weight, refresh or rebuild channel abilities/cache before judging routing behavior.
- If the intended behavior is “tag is only billing attribution, but untagged fallback should remain reachable,” revisit the route-tag design rather than relying on channel weights.

## Related Issues

- `docs/solutions/best-practices/public-group-based-billing-split-requires-code-changes-2026-04-22.md` — broader design warning that automatic tag inference can make untagged fallback channels unreachable.
- `docs/solutions/best-practices/fork-public-group-billing-vs-upstream-tiered-expr-maintenance-boundary-2026-04-24.md` — explains why public-group tag attribution, routing, billing, and logs must stay aligned.
- Relevant code path:
  - `service/group_tag_resolver.go`
  - `service/channel_select.go`
  - `model/channel_cache.go`
  - `model/ability.go`
  - `middleware/distributor.go`
