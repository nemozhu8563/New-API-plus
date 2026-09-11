# Tryvalo frontend USD copy deployment status

## Outcome

The frontend copy change was deployed to the GreenCloud test environment first and then to production on 2026-08-30. Both environments are running the same immutable application image. The public subscription copy now presents included amounts in USD and no longer presents the customer-facing unit as Credit or Credits.

| Stage | State | Evidence |
| --- | --- | --- |
| Planned | Complete | Exact commit, image tags, target services, validation gates, and rollback locations were identified before mutation. |
| Test execution | Complete | Only `new-api-test` was recreated at `2026-08-30T03:03:06Z`. |
| Test validation | Complete | Container, local HTTP, public HTTP, browser copy, console, and performance checks passed. |
| Production execution | Complete | Only `new-api` was recreated at `2026-08-30T03:09:03Z`. |
| Production validation | Complete | Container, dependencies, local HTTP, public ingress, browser copy, system services, and private CPA connectivity checks passed. |
| Rollback | Not executed | Both previous configurations remain available at the paths below. |

## Release identity

- Application commit: `f96bf33b80dfeca9b025a94651fb68db492dc8a7`
- Commit intent: `Present public billing amounts in USD`
- Source state at build time: local `HEAD` and `origin/main` both pointed to the application commit.
- Build source: an exact `git archive HEAD` snapshot; unrelated dirty files were not included.
- Platform: `linux/amd64`
- OCI revision label: `f96bf33b80dfeca9b025a94651fb68db492dc8a7`
- Shared image ID: `sha256:032ba62c4df47fe53bb43e5b8894f0ecca0c9d23ae09d6c79e123aa42c820f1a`
- Test tag: `new-api:new-api-test-20260830T025223Z-f96bf33b80`
- Production tag: `new-api:new-api-release-20260830T025223Z-f96bf33b80`
- Transfer package: `new-api-f96bf33b80-20260830T025223Z.tar.gz`
- Transfer package size: `71,816,286` bytes
- Transfer package SHA-256: `019c053f85a5a2ad83fa082f743997e3c8808cb0677fa313d3774e977d653678`
- GreenCloud import directory: `/opt/new-api/import/new-api-release-20260830T025223Z-f96bf33b80`

The package passed both `sha256sum -c` and `gzip -t` on GreenCloud before `docker load`. Both remote tags resolved to the same image ID, platform, and revision label shown above.

## Change scope

- Localized the affected pricing, subscription, wallet, recharge, and tiered-pricing UI copy.
- Converted legacy customer-facing `Includes ... Credits per billing cycle` subtitles to USD amounts.
- Replaced public recharge and balance wording that described the unit as Credit or Credits.
- Kept internal quota and token accounting unchanged; this was a frontend copy and formatting release.
- Added or updated focused frontend regression tests and synchronized all seven frontend locale files.

The release did not contain database schema changes, payment-provider configuration changes, Stripe catalog changes, DNS changes, Caddy changes, CPA changes, or infrastructure changes.

## Pre-deploy verification

- Focused Vitest coverage: `20/20` passed.
- `bun run typecheck`: passed.
- `bun run i18n:sync`: all seven languages reported `0 missing`, `0 extras`, and `0 untranslated`.
- `bun run lint`: passed with no new release-related issue; only unrelated pre-existing warnings remained.
- `bun run format:check`: passed.
- `bun run build`: passed.
- `git diff --check` and staged secret review: passed.

## Test deployment

- Compose file: `/srv/new-api-test/compose.yaml`
- Service recreated: `new-api-test`
- Previous image: `new-api:new-api-test-20260828T091835Z-91e861f8c9`
- Current image: `new-api:new-api-test-20260830T025223Z-f96bf33b80`
- Current container image ID: `sha256:032ba62c4df47fe53bb43e5b8894f0ecca0c9d23ae09d6c79e123aa42c820f1a`
- Current state after deployment: `running`, `healthy`, restart count `0`
- Compose backup: `/srv/new-api-test/backups/compose.yaml.before-new-api-test-20260830T025223Z-f96bf33b80`

Only the application service was recreated:

```bash
docker compose -f /srv/new-api-test/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api-test
```

Test validation results:

- `http://127.0.0.1:3001/api/status`: HTTP `200`
- `http://127.0.0.1:3001/`: HTTP `200`
- `https://test.tryvalo.com/api/status`: HTTP `200`
- `https://test.tryvalo.com/`: HTTP `200`
- `https://test.tryvalo.com/wallet`: HTTP `200`
- `https://test.tryvalo.com/login`: HTTP `200`
- Startup critical log matches for panic, fatal, or migration failure: `0`
- Browser load time: `2.527s`
- Browser network: application assets and public APIs returned `200`; the unauthenticated token refresh returned the expected `401`.
- Simplified Chinese visible copy included `$290`, `$710`, and `$1,375` billing-cycle amounts.
- Visible Simplified Chinese page matches for `Credit` or `Credits`: `0`

## Production deployment

- Compose file: `/srv/new-api/compose.yaml`
- Image environment file: `/srv/new-api/env/images.env`
- Service recreated: `new-api`
- Previous image: `new-api:new-api-release-20260829T150415Z-79a0bba53`
- Current image: `new-api:new-api-release-20260830T025223Z-f96bf33b80`
- Current container image ID: `sha256:032ba62c4df47fe53bb43e5b8894f0ecca0c9d23ae09d6c79e123aa42c820f1a`
- Current state after deployment: `running`, `healthy`, restart count `0`
- Backup directory: `/srv/new-api/backups/new-api-release-20260830T025223Z-f96bf33b80`
- Validation completed: `2026-08-30 11:14:58 CST` (`2026-08-30T03:14:58Z`)

Only the application service was recreated:

```bash
docker compose --env-file /srv/new-api/env/images.env \
  -f /srv/new-api/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api
```

Production validation results:

- `http://127.0.0.1:3000/api/status`: HTTP `200`
- `http://127.0.0.1:3000/`: HTTP `200`
- `https://api.tryvalo.com/api/status`: HTTP `200`
- `https://new.tryvalo.com/api/status`: HTTP `200`
- `https://api.tryvalo.com/`: HTTP `200`
- `https://api.tryvalo.com/wallet`: HTTP `200`
- `https://api.tryvalo.com/login`: HTTP `200`
- `api.tryvalo.com` and `new.tryvalo.com` both returned `static/js/index.2b5858df53.js`.
- Browser load time on `api.tryvalo.com`: `2.390s`.
- Browser network: application assets and public APIs returned `200`; the unauthenticated token refresh returned the expected `401`.
- Simplified Chinese visible copy included `$290`, `$710`, and `$1,375` billing-cycle amounts.
- Visible Simplified Chinese page matches for `Credit` or `Credits`: `0`
- Startup critical log matches for panic, fatal, or migration failure: `0`
- `caddy`, `cliproxyapi`, and `cloudflared`: all `active`
- Private CPA root probe from the `new-api` container: HTTP `200`

## Dependency and topology invariants

The production database and cache were not recreated:

| Container | Container ID | Started at | State after release |
| --- | --- | --- | --- |
| `new-api-postgres` | `068ed5334fd00c628a10387cd53d66048f162b568785c282987996325d8bbefa` | `2026-07-11T01:00:44.13346078Z` | `running`, `healthy`, restart count `0` |
| `new-api-redis` | `792cc76235b965e3c3b85480b5185bfea9790d9e3141f8f8c96653522c152b4f` | `2026-07-12T00:26:12.673497206Z` | `running`, `healthy`, restart count `0` |

- `api.tryvalo.com` continued through the existing Zgo edge path.
- `new.tryvalo.com` continued directly to GreenCloud at `173.249.203.66`.
- No DNS, firewall, Caddy, Cloudflare, or private CPA routing setting changed.

## Known limits

- This release verified the public landing copy and unauthenticated routes. It did not perform a new authenticated wallet purchase, Stripe Checkout, subscription renewal, refund, dispute, or other payment-provider end-to-end transaction.
- Focused component tests cover the changed authenticated wallet and recharge copy.
- A successful health check proves the application and ingress surfaces only; it does not expand the payment acceptance evidence boundary.

## Rollback

Production application rollback restores the previous image configuration and recreates only `new-api`:

```bash
cp -p \
  /srv/new-api/backups/new-api-release-20260830T025223Z-f96bf33b80/images.env.before \
  /srv/new-api/env/images.env
docker compose --env-file /srv/new-api/env/images.env \
  -f /srv/new-api/compose.yaml config -q
docker compose --env-file /srv/new-api/env/images.env \
  -f /srv/new-api/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api
```

Test rollback restores the previous Compose file and recreates only `new-api-test`:

```bash
cp -p \
  /srv/new-api-test/backups/compose.yaml.before-new-api-test-20260830T025223Z-f96bf33b80 \
  /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml config -q
docker compose -f /srv/new-api-test/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api-test
```

Because this application commit contains no schema or production data mutation, the normal rollback is image-only. Any unrelated data-integrity incident must use a separately reviewed database recovery plan rather than treating the image rollback as a database restore.
