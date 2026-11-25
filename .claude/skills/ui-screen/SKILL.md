---
name: ui-screen
description: Build or change a screen in the mcpDeck React UI so it matches the approved mockups and design tokens. Use for any work under web/src/routes or web/src/components.
---

# UI screen

## Reference

- Screen spec: `docs/ui.md`. Read the section for the screen first.
- Clickable mockup: open `docs/mockups/index.html`. Screenshots are in
  `docs/mockups/screens/`; `mockup.css` has the exact spacing, sizes and colours,
  and `mockup.js` the behaviour and wording.
- Tokens and rules: `docs/design.md#ui`.

## Structure

- One route file per screen in `web/src/routes/`.
- Shared pieces in `web/src/components/ui.tsx` (`Button`, `Chip`,
  `MethodChip`, `StatusChip`, `AccessChip`, `Panel`, `PageHead`, `Tabs`, `Seg`,
  `Dialog`, `Drawer`, `CopyField`, `Toggle`, `Empty`, `EmptyState`, `Menu`,
  `SaveBar`, `Avatar`, `StatCard`, `useConfirm`, toasts) plus `Sidebar`,
  `Palette`, `ConnectDialog`, `AskAgent`, `KVEditor`, `Diff`. Reuse them
  before making new ones.
- Data through TanStack Query hooks in `web/src/lib/hooks.ts` and the client in `web/src/lib/api.ts`; never call `fetch` in components.

## Look

- Page: flat `--bg`; content in white `.panel`s with a 1px `--line` border,
  10px radius, no blur or glow. Compact: 13.5px text, 20px page titles,
  30px controls. A page answers its question in the first screen.
- Buttons: 8px radius. Ink = main page action, blue = confirm, light = secondary,
  red tint = destructive (revoke, reject).
- HTTP methods: mono chips. GET green, POST blue, PUT/PATCH orange, DELETE red.
- Type: Inter / system UI; mono for names of tools, APIs and IDs.
- No purple, no emoji, no gradients on content.
- Actions on the right; destructive ones in the "…" `Menu`, confirmed with
  `useConfirm`; Save via `SaveBar` (shown only when dirty) or a panel footer.
- Every empty list uses `EmptyState` (what it is + first action). Suggest the
  next step in place; onboarding lives in `components/Guide.tsx`
  (`GettingStarted`, `SetupGuide`, the demo `Tour`). Never block the page.

## Done means

- Matches the mockup's layout at 1440px with the tokens in `web/src/styles/tokens.css`, and stays usable at 1024px.
- Keyboard: every action reachable, visible focus ring.
- Contrast AA (muted text is `#6E6E73` or darker).
- A Vitest test covers the screen's main state and one empty/error state.
- `make test-web && make lint` pass.
