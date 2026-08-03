# Core UI Redesign QA

## Scope

- Fixed the product UI language to the Anthropic-inspired warm palette, Public Sans typography, relaxed density, floating sidebar, and system light/dark mode.
- Replaced the public marketing homepage with authentication-aware entry routing.
- Rebuilt the authenticated overview page.
- Simplified API Keys actions to keep only CC Switch import.
- Added filtered CSV export for common, drawing, and task usage logs.

## Route checks

- Signed-out `/` redirects to `/sign-in`.
- Signed-in `/` redirects to `/dashboard/overview`.
- The authenticated shell no longer exposes a homepage navigation button.
- Sign-in and authenticated routes render inside the updated visual system.

## Responsive UI checks

Validated at 1272 x 868 and 390 x 844:

- Overview has no page-level horizontal overflow.
- Overview setup code samples scroll within their own container on narrow screens.
- API Keys has no page-level horizontal overflow.
- Usage logs have no page-level horizontal overflow.
- The CC Switch action is present with `aria-label="填入 CC Switch"`.
- The mobile usage-log export action is present with `aria-label="导出 CSV"`.

Screenshots are stored in `artifacts/ui-core-qa/`.

## CSV export checks

- Common, drawing, and task logs each downloaded successfully from the browser.
- Exports inherit the active log filters and respect administrator/current-user scope.
- Export is capped at 50,000 rows and streamed with a UTF-8 BOM.
- Spreadsheet formula injection is escaped.
- Non-admin exports apply log-field redaction.
- Task exports do not include `private_data`.

## Automated verification

- `GOCACHE=/tmp/new-api-go-build go test -count=1 ./controller ./model ./router`: passed.
- Frontend targeted tests: 11 passed.
- `bun run build:check`: passed.
- Targeted oxlint for changed frontend files: passed, with one pre-existing `no-danger` warning in `footer.tsx`.
- `git diff --check`: passed before final QA documentation.

## Known repository baseline issues

- Full `bun run lint` reports existing unrelated errors outside this change.
- `bun run format:check` reports only the unchanged `src/features/usage-logs/components/usage-logs-mobile-card.tsx`.

## Tryvalo Brand Asset QA

- Replaced the default Web, favicon, Electron application, Windows tray, and macOS template icons with the selected Tryvalo mark.
- Preserved the runtime `/api/status` branding contract; the browser QA supplied `system_name: "Tryvalo"` and `logo: "/logo.png"` through a local request mock instead of hard-coding product identity.
- Compared the selected source artwork with the exported 180 x 180 Web asset and the rendered sign-in page. The black `T`, central negative-space `V`, blue left plane, and terracotta right plane remain recognizable without the source artwork's excess canvas.
- The 180 x 180 Web source renders cleanly at 40.3125 px in the desktop and mobile brand positions. A separate 20 x 20 downsample retained distinct silhouette and color planes.
- The runtime favicon resolves to `/logo.png`; the static `favicon.ico` is available as the initial-load fallback.
- Validated the sign-in route at 1272 x 868 and 390 x 844. The logo stays aligned with the Tryvalo wordmark, does not overflow, and remains legible against the cream background.
- No new console error appeared after the mocked Tryvalo status response loaded. Earlier `/api/status` failures came from running the frontend without a local backend.
- Screenshots are stored in `artifacts/tryvalo-brand-qa/`; trailing empty viewport space was cropped from the saved captures.
- `bun run build:check`: passed.
- Built `dist/logo.png` and `dist/favicon.ico` matched their `public/` sources by SHA-256.

final result: passed
