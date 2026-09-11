---
title: Uptime Kuma service availability fields configure dashboard monitoring, not model availability
date: 2026-05-09
category: documentation-gaps
module: dashboard uptime kuma settings
problem_type: documentation_gap
component: documentation
severity: low
applies_when:
  - Operators ask what the 服务可用性 fields mean in new-api
  - The dashboard shows no monitoring data and says to configure Uptime
  - Someone confuses the 服务可用性 card with model, channel, or group availability controls
tags: [new-api, dashboard, uptime-kuma, service-availability, settings]
symptoms:
  - The admin UI asks for 分类名称, Uptime Kuma地址, and 状态页面Slug without making their relationship obvious
  - Operators may paste a full Uptime Kuma status-page URL into the address field
  - Operators may expect this setting to enable or disable models or channels
root_cause: inadequate_documentation
resolution_type: documentation_update
---

# Uptime Kuma service availability fields configure dashboard monitoring, not model availability

## Context
The `服务可用性` card on the new-api dashboard is a public status monitor panel backed by Uptime Kuma. It is not part of model routing, channel selection, billing groups, or model availability.

The card is rendered by `web/src/components/dashboard/UptimePanel.jsx`. Its data is loaded from `/api/uptime/status`, and that endpoint reads `console_setting.uptime_kuma_groups` from the console settings.

## Guidance
Explain the three fields as one address puzzle:

| Field | Plain meaning | Example |
| --- | --- | --- |
| 分类名称 | The display name shown in the new-api dashboard tab/card | `AI 服务` |
| Uptime Kuma地址 | The base URL of the Uptime Kuma instance, without `/status/<slug>` | `https://status.example.com` |
| 状态页面Slug | The slug of the public Uptime Kuma status page | `api` |

If the Uptime Kuma public status page URL is:

```text
https://status.example.com/status/api
```

then configure:

```text
分类名称: AI 服务
Uptime Kuma地址: https://status.example.com
状态页面Slug: api
```

new-api then builds these two Uptime Kuma API requests internally:

```text
https://status.example.com/api/status-page/api
https://status.example.com/api/status-page/heartbeat/api
```

## Why This Matters
The wording `服务可用性` can sound like a switch for whether a service, model, channel, or group can be used. In this codebase it means only "show service uptime data on the dashboard."

The actual controls are:

- `console_setting.uptime_kuma_enabled`: whether to show the Uptime Kuma panel
- `console_setting.uptime_kuma_groups`: JSON array of configured monitor categories

The backend validates each group has:

- `categoryName`
- `url`
- `slug`

It also requires the slug to contain only letters, numbers, underscores, and hyphens.

## When to Apply
Use this explanation when helping an operator configure the dashboard, especially when they ask what the three fields mean or why the service availability panel has no data.

Do not send them to model pricing, channel settings, or group settings unless their actual goal is model/channel access control.

## Code References
- `web/src/pages/Setting/Dashboard/SettingsUptimeKuma.jsx` saves `console_setting.uptime_kuma_groups` and `console_setting.uptime_kuma_enabled`
- `setting/console_setting/config.go` defines `UptimeKumaGroups` and `UptimeKumaEnabled`
- `setting/console_setting/validation.go` validates `categoryName`, `url`, and `slug`
- `controller/uptime_kuma.go` calls `/api/status-page/{slug}` and `/api/status-page/heartbeat/{slug}`
- `web/src/hooks/dashboard/useDashboardData.js` loads `/api/uptime/status`
