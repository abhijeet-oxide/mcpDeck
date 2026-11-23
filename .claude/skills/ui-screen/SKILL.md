---
name: ui-screen
description: Build or change a screen in the mcpDeck React UI so it matches the approved mockups and design tokens. Use for any work under web/src/routes or web/src/components.
---

# UI screen

## Reference

- Screenshots: `docs/screenshots/*.png`. Look at the relevant one before you start.
- Mockup source: `docs/mockups/*.dc.html`. Exact spacing, sizes and colours.
- Tokens and rules: `docs/design.md#ui`.

## Structure

- One route file per screen in `web/src/routes/`.
- Shared pieces in `web/src/components/`: `Glass`, `Sidebar`, `PillButton`,
  `MethodChip`, `StatusChip`, `StatTile`, `Timeline`, `StepList`, `CodeBlock`.
  Reuse them before making new ones.
- Data through TanStack Query hooks in `web/src/lib/`; never fetch in components.

## Look

- Page: `--bg` with three blurred glows; content sits in glass panels
  (`--glass`, `--glass-blur`, `--shadow`, 24px radius).
- Cards inside panels: white at 62%, 16px radius.
- Buttons are pills. Dark = main page action, blue = confirm, light = secondary,
  red tint = destructive (revoke, reject).
- HTTP methods: mono chips. GET green, POST blue, PUT/PATCH orange, DELETE red.
- Type: Inter / system UI; mono for names of tools, APIs and IDs.
- No purple, no emoji, no gradients on content, no onboarding wizards.

## Done means

- Matches the screenshot at 1440px and stays usable at 1024px.
- Keyboard: every action reachable, visible focus ring.
- Contrast AA (muted text is `#6E6E73` or darker).
- A Vitest test covers the screen's main state and one empty/error state.
- `make test-web && make lint` pass.
