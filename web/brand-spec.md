# UI design contract

## Design read

- Artifact: authenticated API gateway console and sign-in entry.
- Audience: developers, individual API users, operators, and administrators.
- Visual language: warm, restrained builder SaaS with an Anthropic-inspired
  cream-and-clay palette.
- Mode: redesign overhaul. Existing product, data, routes, permissions, and
  accessibility contracts remain authoritative.
- Dials: visual variance 5, motion 3, information density 7, asset dependence
  2, brand fidelity 7.

## Fixed system

- Theme: follow the operating-system light/dark preference.
- Palette: the existing `anthropic` preset is the sole active palette.
- Typography: Public Sans for headings and body copy.
- Radius: `0.75rem`.
- Density: spacious (`lg`) while preserving data-rich layouts.
- Navigation: floating sidebar; page-level navigation lives in the sidebar or
  page tabs, never in the global header.
- Motion: short state feedback only, with reduced-motion support.

## Assets and protected contracts

- The system logo and display name continue to come from `/api/status`; custom
  deployments keep their configured identity.
- Package metadata and runtime module paths remain unchanged unless a separate
  migration explicitly updates their consumers.
- Preserve authenticated routes, deep links, form behavior, API contracts,
  analytics hooks, i18n, keyboard behavior, and real data.

## Scope record

- Preserve: authentication methods, configured logo/name, sidebar information
  architecture, notifications, search, language selection, profile actions.
- Improve: hierarchy, spacing, responsive behavior, empty/error/loading states.
- Remove: the marketing home page, global page-navigation duplication, theme
  configurators, and decorative card/border noise.
- Highest-risk change: root and post-auth routing.
- Rollback: each P0 slice remains an isolated commit on
  `codex/ui-core-redesign`.
