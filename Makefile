# mcpDeck. Targets that need code that isn't written yet will fail until that milestone lands.
BIN      := bin/mcpdeck
GOFLAGS  := -trimpath
LDFLAGS  := -s -w

.PHONY: help dev dev-up dev-down build test test-go test-web e2e lint cover req-check sqlc clean

help: ## list targets
	@grep -E '^[a-z0-9-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*## "}{printf "  %-10s %s\n",$$1,$$2}'

dev: ## Go server + Vite with hot reload
	@pnpm -C web dev & go run ./cmd/mcpdeck serve --dev-ui http://localhost:5173

build: ## single binary with embedded UI
	pnpm -C web install --frozen-lockfile && pnpm -C web build
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/mcpdeck

test: test-go test-web ## all unit + integration tests

test-go: ## Go tests with race detector
	go test -race -shuffle=on ./...

test-web: ## Vitest
	pnpm -C web test --run

e2e: dev-up ## Playwright + MCP client suite against mcpdeck dev
	go test ./e2e/... ; status=$$?; pnpm -C web exec playwright test --trace off; $(MAKE) dev-down; exit $$status

dev-up: build ## mcpdeck dev with mock apps on 127.0.0.1:47800
	@mkdir -p .mcpdeck
	@$(BIN) dev --data .mcpdeck/dev > .mcpdeck/dev.log 2>&1 & echo $$! > .mcpdeck/dev.pid
	@for i in $$(seq 1 50); do curl -fs http://127.0.0.1:47800/healthz >/dev/null && exit 0; sleep 0.1; done; echo "mcpdeck dev did not start, see .mcpdeck/dev.log"; exit 1

dev-down: ## stop mcpdeck dev and delete its data
	@-kill $$(cat .mcpdeck/dev.pid 2>/dev/null) 2>/dev/null
	rm -rf .mcpdeck/dev .mcpdeck/dev.pid

lint: ## golangci-lint + Biome
	golangci-lint run
	pnpm -C web exec biome check .

cover: ## coverage with the 80% gate on internal/
	go test -coverprofile=cover.out ./internal/...
	@go tool cover -func=cover.out | awk '/^total:/{gsub("%","",$$3); if ($$3<80) {print "coverage "$$3"% < 80%"; exit 1} else print "coverage "$$3"%"}'

req-check: ## requirements with no REQ-tagged test yet
	@grep -oE '^\*\*[A-Z]+-[0-9]+' docs/requirements.md | tr -d '*' | sort -u | while read id; do \
	  grep -rqs "REQ: .*\b$$id\b" --include='*_test.go' --include='*.test.ts*' . || echo "$$id"; done

sqlc: ## regenerate typed queries
	sqlc generate -f internal/store/sqlc.yaml

clean: ## remove build output and local dev data
	rm -rf bin web/dist cover.out .mcpdeck
