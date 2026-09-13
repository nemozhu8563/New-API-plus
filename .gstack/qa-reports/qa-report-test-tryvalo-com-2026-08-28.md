# Tryvalo Stripe Sandbox E2E QA Report

## Summary

| Item | Result |
| --- | --- |
| Date | 2026-08-28 |
| Execution window | 17:41-18:31 Asia/Shanghai |
| Target | `https://test.tryvalo.com` |
| Deployment | `new-api:new-api-test-20260828T091835Z-91e861f8c9` |
| Tier | Standard, release-focused |
| Scope | Authenticated customer pages and one real Stripe Sandbox top-up |
| Explicitly excluded | Production deployment; Creem, Epay, Waffo; administrator flows; real money |
| Browser pages | Sign-up/sign-in, Dashboard, Wallet, Profile, Usage Logs, `/subscriptions` access guard, Stripe Hosted Checkout |
| Screenshots | 10 |
| Final status | DONE |

The release-focused health score moved from **97/100 to 100/100** after the Dashboard API-key copy bug was fixed. The Stripe Sandbox top-up completed end to end: Hosted Checkout accepted one test payment, the real webhook settled the existing local order once, the customer balance became `$20`, Billing History showed one successful Stripe order, and Usage Logs showed one Top-up event.

## Scope and proof boundary

This run used an isolated synthetic account and Stripe's public test card data. It created exactly one Sandbox top-up order for CNY 20. It did not use production Stripe mode, production application state, a real person’s payment data, or any non-Stripe payment channel.

Browser return alone was not treated as proof. Completion required all four surfaces below to agree:

1. Stripe Hosted Checkout submitted successfully and returned to Tryvalo.
2. `top_ups` changed from one `pending` row to one `success` row.
3. A real `checkout.session.completed` event succeeded and the account was credited once.
4. Wallet, Billing History, and Usage Logs displayed the resulting customer outcome.

## Result matrix

| Surface | Before | After | Result |
| --- | --- | --- | --- |
| Customer quota | `0` | `10,000,000` quota units, displayed as `$20` | Pass |
| Billing debt | `0` | `0` | Pass |
| Top-up order | One `pending` order | One `success` order, no pending or duplicate user order | Pass |
| Charged amount snapshot | CNY 20 expected | `expected_amount_minor=2000`, `expected_currency=CNY`, `livemode=false` | Pass |
| Stripe references | Not fulfilled | Checkout Session, PaymentIntent, and Charge references present | Pass |
| Webhook | No event in the run window | Latest `checkout.session.completed` is `succeeded`, `attempts=1`, `last_error` empty, `livemode=false` | Pass |
| Top-up log | 0 | Exactly 1 Top-up log | Pass |
| Billing History | Empty | Exactly 1 Stripe order marked `Success` | Pass |
| Production application | Existing production image | Same image, start time, and healthy runtime | Pass, unchanged |

`logs.quota=0` for the Top-up log is expected in the current schema. `RecordTopupLog` records the event and formatted amount in its content; credited value is authoritative in `top_ups.credited_quota` and `users.quota`.

## Fixed issue

### ISSUE-001 - Dashboard copied the masked API-key list value

- Severity: High
- Category: Functional
- Fix status: Verified
- Commit: `e15486acc1859f701e4b1a6e531aeb5d1e52860d`
- Files changed:
  - `web/src/features/dashboard/components/overview/overview-dashboard.tsx`
  - `web/src/features/dashboard/components/overview/__tests__/overview-dashboard.test.tsx`

The Dashboard displayed a masked key, which is correct for presentation, but the Copy action previously copied that masked value. The fix fetches the full key by ID at click time, normalizes its `sk-` prefix, handles failure, and disables duplicate clicks while the fetch is in flight. The regression test proves the clipboard receives the full key rather than the masked list value.

Browser evidence: [Dashboard after fix](screenshots/dashboard-overview.png). The isolated QA account had no API key, so the live browser run verified the deployed Dashboard page but did not place a real key on the system clipboard; the exact copy contract is protected by the passing regression test.

## Release quality gates

Commit `91e861f8c970a0b5c1897eeabaaca17f07fa1613` brought the full frontend lint and formatting gates to zero findings. Final local validation passed:

- `bun run lint`: 0 errors and 0 warnings.
- `bun run typecheck`: passed.
- `bun run test`: 76 files, 309 tests passed.
- `bun run build`: passed.
- `bun run format:check`: passed.
- `go test ./...`: passed.
- `relaykit` with `GOWORK=off go build ./...`: passed.

## Browser evidence

- [Initial registration](screenshots/signup-initial.png)
- [Dashboard overview](screenshots/dashboard-overview.png)
- [Wallet before payment](screenshots/wallet-before-topup.png)
- [Profile](screenshots/profile.png)
- [Usage Logs before payment](screenshots/usage-logs.png)
- [`/subscriptions` access guard](screenshots/subscriptions-access.png)
- [Stripe Hosted Checkout before form submission](screenshots/stripe-checkout.png)
- [Return page with one Top-up log](screenshots/stripe-return-success.png)
- [Billing History with one successful Stripe order](screenshots/wallet-after-topup.png)
- [Wallet balance after settlement](screenshots/wallet-balance-after-topup.png)

The Stripe form used synthetic contact/address data, Link information saving was disabled, and the “I am an AI agent acting on behalf of someone else” disclosure was checked before payment. Form values are intentionally absent from this report.

## Console health

Dashboard, Wallet, Profile, and Usage Logs loaded without fresh application console errors. One earlier 401 came from restoring an expired browser session; after re-authentication and clearing the console, Wallet and the post-payment customer flow produced no console errors. Stripe emitted two third-party preload warnings during Checkout; they did not affect submission or settlement.

## Health score

| Category | Weight | Before | Final |
| --- | ---: | ---: | ---: |
| Console | 15% | 100 | 100 |
| Links | 10% | 100 | 100 |
| Visual | 10% | 100 | 100 |
| Functional | 20% | 85 | 100 |
| UX | 15% | 100 | 100 |
| Performance | 10% | 100 | 100 |
| Content | 5% | 100 | 100 |
| Accessibility | 15% | 100 | 100 |
| Weighted total | 100% | **97** | **100** |

This score applies only to the release-focused scope above; it is not a claim that every administrator page, language, provider, or production path has been exhaustively tested.

## Remaining release gates, not defects in this run

1. Production deployment remains a separate authorized action with its own backup, migration, canary, and rollback record.
2. The occupied OAuth-identity browser rejection message remains without a clean browser screenshot because Chrome intercepted that historical flow; server behavior is covered by tests.
3. Database restore rehearsal, MySQL migration verification, administrator flows, content-policy browser rejection, and paid upstream model requests remain outside this Stripe-focused run.

Creem, Epay, and Waffo were intentionally excluded. Their absence is not a Stripe release failure.

## PR summary

QA found 1 high functional issue, fixed and regression-tested it, restored all frontend quality gates, and completed one real Stripe Sandbox top-up E2E; release-focused health score improved from 97 to 100.
