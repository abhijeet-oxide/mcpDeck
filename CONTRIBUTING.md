# Contributing

Thanks for helping. mcpDeck is maintained by agents and people under the
same rules, written down in [AGENTS.md](AGENTS.md). The short version:

## Set up

You need Go 1.26, Node 22 with pnpm (Corepack), and `make`.

```sh
make build          # ./bin/mcpdeck with the UI embedded
make dev            # demo server with mock APIs + Vite hot reload on :5173
```

## Make a change

1. **Find or add the requirement** in [docs/requirements.md](docs/requirements.md).
2. **Write the failing test first**, tagged `// REQ: <ID>`. For a bug, the
   test reproduces it.
3. **Add a scenario** in `e2e/scenarios` for anything a user or agent can see.
   Start from `e2e/scenarios/template_test.go.txt`; the guide is in
   [docs/testing.md](docs/testing.md).
4. **Make it pass**, then run the gates:
   ```sh
   make test && make lint && make claims
   ```
5. **Docs follow tests.** A README sentence needs a test and a row in
   [docs/claims.md](docs/claims.md). No comparisons with other products.
   Numbers come from committed raw output.
6. **Add a CHANGELOG line** under Unreleased if users will notice.
7. **Open a PR** with a conventional-commit title (`feat(auth): ...`,
   `fix(proxy): ...`) and fill in the template. The title decides the
   release: see [docs/releasing.md](docs/releasing.md).

## Good first areas

- A mock of a system you know, in `e2e/mockenv`, with scenarios for its
  auth, pagination and error shapes.
- A lint rule with a published source (`internal/lint/rules.go`).
- Import formats listed as not built in the README's Status section.

## Reporting security issues

See [SECURITY.md](SECURITY.md). Please don't open public issues for
vulnerabilities.
