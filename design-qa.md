# Tryvalo four-week plans design QA

Date: 2026-08-20

## Comparison target

- Source visual truth: `/var/folders/37/2_349dss5js4j5ll3fqmcsf80000gn/T/codex-clipboard-58e0a3ef-f334-42c8-9748-844d457f5c15.png`
- Rendered implementation: `artifacts/design-qa/plans-desktop-final.png`
- Normalized implementation: `artifacts/design-qa/plans-desktop-content-normalized.png`
- Side-by-side comparison: `artifacts/design-qa/plans-comparison.png`
- Full-page desktop evidence: `artifacts/design-qa/landing-full-desktop.png`
- Mobile evidence: `artifacts/design-qa/plans-mobile-390x844.png`
- State: Simplified Chinese, anonymous visitor, plans loaded, no interaction overlay included in the normalized comparison.
- Browser viewport: 1440 x 900 CSS px at device scale factor 1.
- Source pixels: 1396 x 705.
- Raw implementation pixels: 1385 x 699.
- Normalized implementation pixels: 1396 x 705. The implementation crop was scaled to the source dimensions before side-by-side comparison.

## Full-view comparison

The implementation preserves the source hierarchy and proportions: centered plan heading, muted four-week explanation, four equal columns, a highlighted second plan, dark plan cards, and the orange enterprise card. The public product requirement intentionally replaces the source's discount and concurrency claims with the truthful seven-day credit refresh and four-week automatic-renewal terms.

## Focused comparison

The plan section is the conversion-critical focused region, so it was compared independently in `plans-comparison.png`. Prices, weekly credit labels, recommendation treatment, card spacing, CTA hierarchy, and enterprise contact treatment remain clearly legible at equal pixel dimensions. A separate focused crop was not needed because all critical card copy is readable in this normalized section comparison.

## Required fidelity surfaces

- Fonts and typography: Weight hierarchy, large plan prices, compact metadata, and line wrapping match the reference closely. The implementation uses the product's existing font stack rather than importing the reference site's font.
- Spacing and layout rhythm: Four-column desktop layout, card padding, divider placement, and highlighted-card treatment are consistent with the reference. Mobile cards stack without horizontal overflow.
- Colors and visual tokens: Near-black section, blue-violet recommended state, muted white copy, and warm orange enterprise treatment match the source direction while using existing product tokens/components.
- Image and icon fidelity: The section contains no raster product imagery. Existing Hugeicons provide the check, enterprise, loading, and arrow symbols; no placeholder or handcrafted SVG asset is used in this section.
- Copy and content: Only the CNY four-week plan total, weekly credit allowance, seven-day refresh, four-week automatic renewal, and enterprise benefits are shown. Model, token, request unit-price, and discount claims are absent for every public visitor.

## Findings

- No actionable P0, P1, or P2 visual mismatch remains.
- Accepted product deviation: the reference's API discount/unit-price row is intentionally removed rather than reproduced.
- Accepted implementation deviation: CTA copy reflects authentication state and sends anonymous visitors to sign in before Checkout.

## Interaction and responsive checks

- `View plans` resolves to `/#plans`.
- Anonymous plan CTAs resolve to `/sign-in?redirect=%2F%23plans`.
- Footer legal and contact links resolve to the intended pages and `contract@tryvalo.com`.
- Mobile evidence at a 390 x 844 viewport shows a readable single-column plan flow.
- The development-only TanStack launchers visible in raw full-page captures are not present in the normalized plan comparison or production build.

## Comparison history

1. Initial reference included public API discount/unit-price claims that conflict with the confirmed product scope.
2. Implementation removed those claims, added seven-day refresh and four-week renewal disclosures, and preserved the selected visual hierarchy.
3. Post-fix normalized comparison `artifacts/design-qa/plans-comparison.png` has no remaining P0/P1/P2 finding.

final result: passed
