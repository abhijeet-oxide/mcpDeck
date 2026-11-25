# web

React + Vite + TypeScript UI, built into `dist/` and embedded into the Go binary
(`embed.go`). Match the mockup in `docs/mockups/` (see `docs/ui.md`) and the tokens in `docs/design.md#ui`.

```sh
pnpm -C web dev     # http://localhost:5173, proxies /api to :47800
pnpm -C web test
```
