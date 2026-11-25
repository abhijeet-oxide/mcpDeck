# Mockup

A clickable mockup of every mcpDeck screen, running on the demo world. Open
`index.html` in a browser; no build step and no server. The spec it follows,
screen by screen, is [../ui.md](../ui.md).

| File | What it is |
|---|---|
| `index.html` | Page shell and the tool files (TOOL-1) used by the editor |
| `mockup.css` | Tokens (light and dark), layout and components |
| `mockup.js` | Demo world data, every screen, dialogs and actions |
| `shoot.mjs` | Screenshots of every screen and a smoke test of the main flows |
| `screens/` | The screenshots |

Use the bar at the top to switch between **First run** (empty, to walk
through Add API → toolkit → connect → first tool) and **Demo world**.

```sh
node docs/mockups/shoot.mjs --check   # flows and console errors only
node docs/mockups/shoot.mjs           # rewrite screens/*.jpg
```

Both need Node and Playwright with Chromium. The mockup is a reference for
layout, wording and behaviour. It's never shown as the product (DOC-1); the
README gets real screenshots once the screens exist.
