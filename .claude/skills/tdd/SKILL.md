---
name: tdd
description: Test-first workflow for mcpDeck. Use for any change that adds or alters behaviour in internal/, cmd/ or web/ — new features, bug fixes, refactors that could change output.
---

# Test-driven change

## 1. Pin the requirement

Find the requirement ID in `docs/requirements.md` (e.g. `ACC-2`). If none fits,
add one in the right section with acceptance criteria, then continue. For a bug,
the requirement is the behaviour the bug violates.

## 2. Red

Write the smallest failing test that describes the behaviour, next to the code:

```go
// REQ: ACC-2
func TestProxy_ReadGrantBlocksWriteMethods(t *testing.T) {
	tests := []struct{ method string; want int }{
		{"GET", 200}, {"POST", 403}, {"DELETE", 403},
	}
	// ...
}
```

Run only that test and confirm it fails for the expected reason (not a compile
error or a nil pointer):

```sh
go test ./internal/proxy -run TestProxy_ReadGrantBlocksWriteMethods -v
```

For the UI, use Vitest + Testing Library with MSW handlers in `web/src/test/`.

## 3. Green

Write the least code that makes it pass. Use the existing interfaces; inject a
clock and ID generator instead of calling `time.Now()` or random sources.

## 4. Refactor

Tidy names and duplication while green. Don't widen the change.

## 5. Prove it

```sh
make test && make lint
```

If the change crosses a boundary (proxy, MCP, sessions, UI), also:

```sh
make dev-up
# call the mcpdeck-dev MCP tools, or run: make e2e
make dev-down
```

## 6. Report

Say which requirement, which tests you added, the commands you ran and their
result. Mention anything you couldn't run.

Never delete, skip or loosen an existing test to get green. If a test is wrong,
fix it in a separate commit and explain why.
