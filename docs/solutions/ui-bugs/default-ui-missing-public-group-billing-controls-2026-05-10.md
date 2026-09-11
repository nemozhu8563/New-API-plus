---
title: Restoring fork-specific public-group billing controls in the new default UI
date: 2026-05-10
category: ui-bugs
module: web/default system settings billing
problem_type: ui_bug
component: tooling
severity: medium
symptoms:
  - Switching the upstream-sync test branch to the new default UI removed the fork-specific public-group billing controls that existed in the classic UI
  - Admins could not edit public-group tag ratios or public-group model-tag overrides from System Settings -> Billing -> Group Pricing
  - The backend option keys and classic UI support still existed, so the missing controls looked like a frontend regression rather than a removed backend feature
root_cause: logic_error
resolution_type: code_fix
related_components:
  - web/default system settings
  - group ratio option persistence
  - public-group billing attribution
tags: [new-api, upstream-sync, web-default, billing, group-pricing, public-group, classic-ui, test-env]
---

# Restoring fork-specific public-group billing controls in the new default UI

## Problem
After the upstream sync introduced the new `web/default` admin UI, the Billing -> Group Pricing surface no longer exposed this fork's public-group billing controls. Operators could still configure ordinary group pricing, but they lost the visual controls for maintaining public-group tag ratios and public-group model-tag overrides.

The missing controls were not a backend removal. The fork-specific option keys still existed and the classic UI still represented the feature. The regression was that the new default UI had not been wired to the same fork-owned settings.

## Symptoms
- `web/default` Group Pricing did not show controls for `group_ratio_setting.public_group_tag_ratio`.
- `web/default` Group Pricing did not show controls for `group_ratio_setting.public_group_model_tag`.
- The backend still exported and refreshed these options, so saving the same keys through an editor would still affect pricing behavior.
- The test deployment could look healthy while still hiding the controls, which made it important to distinguish code absence from stale frontend assets.

## What Didn't Work
- Keeping this as a classic-only admin flow was rejected. Switching the test environment to the new frontend should not make operators return to the old UI for fork-specific billing maintenance.
- Treating upstream pricing or `tiered_expr` controls as a replacement was also rejected. Prior sync planning had already identified `web/default` as a migration boundary and public-group billing as a protected fork behavior (session history).
- Looking only at container health would not prove the fix. A related stale-frontend incident showed that `/api/status` can pass while the served UI bundle is old, so verification also had to inspect the deployed static bundle.

## Solution
Restore the missing settings as first-class controls in the new default UI while reusing the existing backend option keys.

Commit `bcd964718885` made three UI-side changes:

- added `PublicGroupBillingRulesEditor` and rendered it inside the new group-ratio form
- threaded both fork-owned settings through the new UI schema, defaults, section registry, and save mapping
- kept JSON fallback editing beside the visual editor, so operators can still paste or repair raw option payloads

The critical mapping is that the new form fields still save back to the existing backend option keys:

```ts
const apiKeyMap = {
  PublicGroupTagRatio: 'group_ratio_setting.public_group_tag_ratio',
  PublicGroupModelTag: 'group_ratio_setting.public_group_model_tag',
}
```

Key files to check when this surface changes again:

- `web/default/src/features/system-settings/models/public-group-billing-rules-editor.tsx`
- `web/default/src/features/system-settings/models/group-ratio-form.tsx`
- `web/default/src/features/system-settings/models/ratio-settings-card.tsx`
- `web/default/src/features/system-settings/billing/index.tsx`
- `web/default/src/features/system-settings/billing/section-registry.tsx`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/features/models/components/drawers/model-mutate-drawer.tsx`
- `web/default/src/i18n/locales/en.json`
- `web/default/src/i18n/locales/zh.json`

The new editor gives operators visual editing for:

- public group + channel tag -> ratio rules
- public group + model -> channel tag override rules

The form also keeps JSON fallback editing for both settings, so operators can still paste or repair raw option payloads if needed.

## Why This Works
The backend already understood the two option keys. The UI only needed to expose, load, validate, and save the same settings from the new frontend.

The fix works because it preserves the existing contract:

- `group_ratio_setting.public_group_tag_ratio` stores nested public-group to channel-tag ratios.
- `group_ratio_setting.public_group_model_tag` stores public-group and model-name to channel-tag overrides.
- The backend option path refreshes pricing state when these options change.
- This UI change does not alter the existing backend attribution path; it only restores the operator surface for editing the fork-owned settings.

This keeps upstream `web/default` usable without collapsing fork-specific attribution into upstream dynamic pricing.

## Prevention
- When upstream moves or replaces an admin surface, compare classic and default UI parity for fork-owned settings before treating the new UI as production-ready.
- For billing settings, verify three layers together: backend option keys, frontend load/save mappings, and the deployed static bundle.
- Do not mark `web/default` parity complete until fork-specific controls that affect real billing or attribution are visible in the new UI.
- Add a static regression check that fails if `group_ratio_setting.public_group_tag_ratio` or `group_ratio_setting.public_group_model_tag` disappears from the new UI schema/defaults, `section-registry.tsx`, or `ratio-settings-card.tsx` load/save mapping.
- Keep public-group billing controls under Billing -> Group Pricing so operators can find them next to the rest of the group ratio settings.

Verification performed for the fix:

- `cd web/default && bun run typecheck`
- `cd web/default && bun run build`
- `cd web/default && bunx eslint <changed frontend files>`
- Cloud Build image `upstream-test-ui-20260510-121533-bcd964718885` was deployed to `new-api-test`.
- The deployed JS bundle served from `127.0.0.1:3001` contained the labels `Public group tag ratios` and `Public group model tag overrides`.

## Related Issues
- `docs/solutions/best-practices/fork-public-group-billing-vs-upstream-tiered-expr-maintenance-boundary-2026-04-24.md` - explains why public-group attribution remains separate from upstream `tiered_expr`.
- `docs/solutions/best-practices/public-group-based-billing-split-requires-code-changes-2026-04-22.md` - explains the underlying public-group to internal billing attribution architecture.
- `docs/solutions/workflow-issues/stale-new-api-test-frontend-hides-tiered-billing-ui-2026-04-24.md` - related missing-UI symptom, but with stale deployed frontend assets as the root cause.
- `docs/solutions/runtime-errors/stale-billing-mode-map-after-hot-config-update-2026-04-28.md` - related billing UI/runtime mismatch pattern with stale runtime config as the root cause.
