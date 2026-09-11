---
date: 2026-05-14
topic: gpt-image-2-size-tier-development-options
focus: TryValo gpt-image-2 size, ratio, edit, and billing support in new-api
---

# Ideation: gpt-image-2 Size Tier Development Options

## Codebase Context

- The OpenAI image relay surface is `POST /v1/images/generations` and `POST /v1/images/edits`.
- `dto.ImageRequest` carries `Size string`; there is no first-class `aspect_ratio` field for the OpenAI image path.
- For OpenAI-compatible channels, generation requests are passed through as JSON, and edit requests forward multipart non-file fields plus `image` / `mask` file fields.
- `GetAndValidOpenAIImageRequest` restricts sizes for `dall-e` and `dall-e-3`, but not for `gpt-image-*`.
- `ImageHelper` currently applies `n` through `OtherRatio("n")`; it does not add a 1K/2K/4K size multiplier for `gpt-image-2`.
- `ModelPriceHelper` already applies `TokenCountMeta.ImagePriceRatio` to model-price billing before pre-consume, so image-size billing can be implemented without changing the customer-facing request shape.
- `tiered_expr` can read JSON request bodies through `param("size")`, but multipart edit bodies are not available to expressions unless `BillingRequestInput` is explicitly built from the parsed `ImageRequest`.

## Verified Behavior

Live tests against `https://api.tryvalo.com` showed:

- `GET /v1/models` exposes `gpt-image-2`.
- Generation and edit both accept concrete `size` strings.
- The full verified matrix is:
  - 1K: `1024x1024`
  - 2K: `1536x1024`, `1024x1536`, `1792x1024`, `1024x1792`, `2048x2048`, `2048x1152`, `1152x2048`
  - 4K: `3840x2160`, `2160x3840`
- 4K 4:3 is not supported on this upstream:
  - `3840x2880` returns `Invalid size ... Requested resolution exceeds the current pixel budget`
  - `2880x3840` returns the same class of upstream 400 error
- A `1024x1024` input image can be edited into 2K and 4K outputs. For edits, `size` means output size, not input image size.
- Large edit tests need a long client timeout; PowerShell/.NET `HttpClient`'s default 100 second timeout can produce false failures.

## Topic Axes

- Customer request contract: which parameters and sizes customers should send.
- Billing and enforcement: how 1K/2K/4K should be charged and whether mismatches are blocked.
- Provider compatibility: how much logic should be specific to TryValo/sub2api vs reusable for future image upstreams.
- Operations and testing: how to verify upstream behavior without leaking keys or misreading timeouts.

## Ranked Ideas

### 0. Final direction: one OpenAI-compatible model, fixed size whitelist, size-derived billing

**Description:**
Expose a single public model, `gpt-image-2`, through the standard OpenAI Images endpoints. Customers keep sending OpenAI-compatible fields. The only resolution control is the existing `size` parameter with concrete `widthxheight` strings.

The gateway classifies `size` into a supported tier:

- `1k`
- `2k`
- `4k`

Unsupported or custom resolutions are rejected before routing to upstream. Billing is derived from that tier, not from model aliases.

**Why it matters:**
This matches the desired product contract: fully compatible with OpenAI Images, one model name, different billing by requested resolution, and no arbitrary custom resolution surface.

**Implementation shape:**

1. Add a shared `gpt-image-2` size classifier close to `dto.ImageRequest`.
2. Call it from `GetAndValidOpenAIImageRequest` for both JSON generation and multipart edit requests.
3. Set deterministic defaults for `gpt-image-2` when compatible clients omit optional fields:
   - empty `size` -> `1024x1024`
   - empty `quality` -> keep current upstream-compatible default behavior unless a provider-specific default is required
   - empty/zero `n` -> `1`
4. Reject any `gpt-image-2` size outside the verified whitelist.
5. Derive the billing multiplier from the same classifier in `ImageRequest.GetTokenCountMeta()` so pre-consume and post-consume use the same request-tier decision.
6. Include tier and size in log details so operators can audit why a request was charged as 1K, 2K, or 4K.

**Status:** Chosen direction for strict resale. Supersedes the alias-only pilot idea below.

### 1. Configuration-only launch with model aliases (fastest)

**Description:**
Expose three sellable model names and map them to the same upstream model:

- `gpt-image-2-1k` -> upstream `gpt-image-2`
- `gpt-image-2-2k` -> upstream `gpt-image-2`
- `gpt-image-2-4k` -> upstream `gpt-image-2`

Set different fixed model prices for those aliases in `new-api`. Customer docs say which `size` values are allowed for each alias.

**Why it matters:**
This can likely ship without touching code. It is enough if the current priority is to sell quickly and customers are trusted or controlled.

**Downside:**
It does not enforce size/model consistency. A customer could call `gpt-image-2-1k` with `3840x2160`, and the gateway would forward it unless upstream rejects it.

**Status:** Rejected for the strict resale contract. It is still useful only as a no-code emergency fallback.

### 2. Add gateway-side size tier validation for image aliases

**Description:**
Implement a small shared size classifier for image requests:

```text
1024x1024 -> 1k
1536x1024, 1024x1536, 1792x1024, 1024x1792,
2048x2048, 2048x1152, 1152x2048 -> 2k
3840x2160, 2160x3840 -> 4k
```

Then enforce alias compatibility:

- `gpt-image-2-1k` only accepts 1K sizes
- `gpt-image-2-2k` only accepts 2K sizes
- `gpt-image-2-4k` only accepts 4K sizes

Reject unsupported ratios such as `3840x2880` / `2880x3840` before sending upstream.

**Why it matters:**
This closes the biggest commercial hole in the alias approach: customers cannot underpay by choosing a cheap alias with an expensive output size.

**Downside:**
Requires code changes in request validation or relay setup, plus tests for both JSON generation and multipart edits.

**Status:** Recommended first code change if strict billing matters.

### 3. Add size-tier OtherRatio for image billing

**Description:**
Instead of relying on aliases, derive `image_size_tier` from `request.Size` inside `ImageHelper` or a helper called from it, then add a billing multiplier to `PriceData.OtherRatios`:

```text
image_size_tier=1k -> 1.0
image_size_tier=2k -> configured multiplier
image_size_tier=4k -> configured multiplier
```

Keep `n` as the existing separate multiplier.

**Why it matters:**
This lets one public model name, `gpt-image-2`, charge different prices by output size.

**Downside:**
The multipliers need a configuration surface. Hardcoding them would be fast but less maintainable. It also needs careful log display so admins can see why a request cost more.

**Status:** Useful if the site wants ratio-mode billing to support the same size multipliers. For the first implementation, `TokenCountMeta.ImagePriceRatio` is a lower-risk path for model-price billing because it runs before pre-consume.

### 4. Make tiered_expr request-aware for multipart image edits

**Description:**
For image requests, build `BillingRequestInput` from the parsed `dto.ImageRequest` rather than only reading raw JSON bodies. That makes expressions such as `param("size")` work for both generation JSON and edit multipart requests.

Then an admin could configure pricing like:

```text
param("size") == "1024x1024"
  ? tier("1k", 1.0)
  : (param("size") == "3840x2160" || param("size") == "2160x3840")
    ? tier("4k", 4.0)
    : tier("2k", 2.0)
```

**Why it matters:**
This reuses the existing dynamic billing framework instead of inventing a separate image-pricing system.

**Downside:**
The current expression system is token-price oriented. Per-call image pricing through expressions can work, but it must be tested carefully because image `usage` is often synthetic (`TotalTokens = 1` in this flow).

**Status:** Strong if the site already wants dynamic pricing rules in the admin UI.

### 5. Add an official image compatibility test harness

**Description:**
Promote the ad hoc PowerShell tests into durable scripts:

- generation matrix test
- edit matrix test
- dimension verification
- unsupported-size negative tests
- long timeout defaults

Keep the API key in environment variables only.

**Why it matters:**
The most expensive bugs here are false assumptions about upstream behavior. A repeatable harness lets operators validate a new upstream before changing code or pricing.

**Downside:**
Does not enforce production behavior by itself.

**Status:** Recommended alongside any code or config rollout.

## Recommended Development Plan

### Phase 1: Add strict OpenAI Images size validation

Implement:

- `ClassifyGPTImage2Size(size string) (tier, aspectRatio string, multiplier float64, ok bool)`
- validation for both `/v1/images/generations` JSON and `/v1/images/edits` multipart
- clear 400 errors for unsupported sizes such as `3840x2880`
- a deterministic default of `1024x1024` when `gpt-image-2` receives an empty `size`

Suggested test cases:

- generation accepts every verified `gpt-image-2` size
- edit accepts every verified `gpt-image-2` size
- empty `size` becomes `1024x1024`
- `3840x2880` and `2880x3840` are rejected
- arbitrary custom sizes such as `4096x4096`, `3000x2000`, `2048x1536`, and `512x512` are rejected for `gpt-image-2`
- multipart edit preserves `size` and validates it the same as JSON

### Phase 2: Add same-model size-tier billing

Use one public model name:

| Public model | Request parameter | Billing tier |
|---|---|---|
| `gpt-image-2` | `size=1024x1024` | 1K |
| `gpt-image-2` | any verified 2K `size` | 2K |
| `gpt-image-2` | any verified 4K `size` | 4K |

First implementation:

1. Treat configured `gpt-image-2` model price as the 1K base price.
2. Apply `ImagePriceRatio` from the size classifier before pre-consume:
   - 1K -> `1.0`
   - 2K -> configurable target if added, otherwise initial multiplier such as `2.0`
   - 4K -> configurable target if added, otherwise initial multiplier such as `4.0`
3. Keep existing `n` handling as a separate multiplier.
4. Add log details:
   - `大小 2048x1152`
   - `分辨率档位 2K`
   - `生成数量 1`

This keeps the customer interface compatible and makes the same model bill differently by request parameters.

### Phase 3: Make price multipliers configurable

If the business price is not exactly `1x / 2x / 4x`, add a small admin/config map rather than hardcoding:

```json
{
  "gpt-image-2": {
    "1k": 1.0,
    "2k": 2.0,
    "4k": 4.0
  }
}
```

Do not put this into the public request. It is an operator-side pricing rule.

### Phase 4: Make tiered_expr work for multipart image edits, if needed

If the site wants expression-based pricing instead of image-size multipliers, preload `BillingRequestInput` from the parsed `dto.ImageRequest` for all image requests. That makes `param("size")` reliable for both JSON generation and multipart edits.

This is optional after Phase 2. It is valuable only if operators need richer expressions than 1K/2K/4K multipliers.

## Final Interface Contract

### Generation

```http
POST /v1/images/generations
Content-Type: application/json
Authorization: Bearer <customer_key>
```

```json
{
  "model": "gpt-image-2",
  "prompt": "A clean studio product poster",
  "size": "2048x1152",
  "quality": "high",
  "n": 1,
  "response_format": "b64_json"
}
```

### Edit

```http
POST /v1/images/edits
Content-Type: multipart/form-data
Authorization: Bearer <customer_key>
```

Required form fields:

- `model=gpt-image-2`
- `image=<file>`
- `prompt=<edit instruction>`
- `size=<verified widthxheight>`

Optional form fields:

- `mask=<file>`
- `quality=high`
- `n=1`
- `response_format=b64_json`

### Allowed `gpt-image-2` sizes

| Tier | Ratio | size |
|---|---|---|
| 1K | 1:1 | `1024x1024` |
| 2K | 3:2 | `1536x1024` |
| 2K | 2:3 | `1024x1536` |
| 2K | 7:4 | `1792x1024` |
| 2K | 4:7 | `1024x1792` |
| 2K | 1:1 | `2048x2048` |
| 2K | 16:9 | `2048x1152` |
| 2K | 9:16 | `1152x2048` |
| 4K | 16:9 | `3840x2160` |
| 4K | 9:16 | `2160x3840` |

Do not expose a separate `aspect_ratio` parameter for OpenAI Images compatibility. Ratio is derived from the concrete `size`.

## Rejection Summary

| Idea | Reason Rejected |
|---|---|
| Advertise arbitrary ratios like `4K 4:3` | TryValo rejected `3840x2880` and `2880x3840` with pixel-budget errors |
| Let customers send `size: "16:9"` | The OpenAI-compatible image path expects concrete `widthxheight` strings and currently forwards `size` as-is |
| Depend only on upstream rejection | Customers can still select a cheaper alias with an expensive valid size; upstream will accept it and billing will be wrong |
| Use model aliases as the primary contract | The final requirement is one OpenAI-compatible model whose price changes by `size`, not separate public SKUs |
| Support custom `widthxheight` values by pixel threshold | This would make billing and upstream behavior ambiguous; the product contract should be a whitelist |
| Modify sub2api | The upstream is not under local control and the user explicitly prefers not to change it |
| Hardcode absolute 1K/2K/4K prices directly in request handling | Fast but brittle; use the configured model price as the base price and keep tier multipliers configurable |

## Session Log

- 2026-05-14: Verified TryValo `gpt-image-2` generation and edit size matrix.
- 2026-05-14: Verified 4K 4:3 and 3:4 fail with upstream pixel-budget errors.
- 2026-05-14: Initial development-option ideation documented.
- 2026-05-25: Finalized direction as one OpenAI-compatible `gpt-image-2` model, fixed `size` whitelist, and same-model size-tier billing.
