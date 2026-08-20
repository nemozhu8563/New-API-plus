# Tryvalo public web brand specification

## Mode

- Public landing page: Redesign · Preserve.
- Wallet subscription billing: Extension.
- Preserve existing routes, runtime configuration, accessibility behavior, dark mode, and authenticated console journeys.

## Brand assets

- Primary logo: runtime `logo` returned by `/api/status` and rendered through the existing `HeaderLogo` component.
- Local fallback logo: `public/logo.png`.
- Product imagery: none required for the v0; the API request example is the product proof surface.
- Do not copy text, graphics, source, or identity assets from the reference site.

## Visual system

- Palette: existing semantic CSS tokens only (`background`, `foreground`, `primary`, `muted`, `accent`, `border`, and status tokens).
- Typography: Public Sans for display and body; the existing monospace stack for API examples.
- Spacing: 4px base with 8px primary rhythm; public content width capped at 1152px.
- Radius: existing `rounded-xl` and `rounded-2xl` system; `rounded-3xl` only for the primary pricing callout.
- Elevation: hairline rings and borders first; no decorative drop-shadow hierarchy.
- Motion: existing 150–200ms interaction feedback; navigation keeps its established transition. Honor reduced motion.

## Design calibration

- Visual variance: 5/10 — familiar developer landing structure with one asymmetric hero split.
- Motion intensity: 3/10 — state feedback and existing header behavior only.
- Information density: 6/10 — enough product, pricing, delivery, and legal context for developers and payment review.
- Asset dependence: 2/10 — typography, code, and real runtime branding carry the page.
- Brand fidelity: 9/10 for the landing page and 10/10 for wallet extensions.

## Content constraints

- Describe a unified AI API and public per-model usage pricing truthfully.
- Do not invent customer logos, testimonials, uptime figures, savings claims, model availability, or fixed SaaS tiers.
- Keep CNY top-up mechanics out of the landing-page hero; detailed purchase terms belong in the authenticated wallet and legal documents.
- Keep `contract@tryvalo.com`, Terms of Service, and Privacy Policy reachable from public pages.
