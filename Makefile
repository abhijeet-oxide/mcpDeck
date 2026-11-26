# mcpDeck developer commands. `make help` lists them.
BIN      := bin/mcpdeck
GOFLAGS  := -trimpath
LDFLAGS  := -s -w

.PHONY: help dev dev-up dev-down build test test-go test-web test-skills scenarios e2e bench lint lint-docs claims lint-actions cover req-check sqlc clean next-version release-check

help: ## list targets
	@grep -E '^[a-z0-9-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*## "}{printf "  %-10s %s\n",$$1,$$2}'

dev: ## demo server (token mcpdeck-dev) + Vite with hot reload on :5173, proxied to it
	@trap 'kill 0' EXIT; go run ./cmd/mcpdeck dev --fixed-token --no-open & pnpm -C web dev

build: ## single binary with embedded UI
	pnpm -C web install --frozen-lockfile && pnpm -C web build
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/mcpdeck

test: test-go test-web test-skills ## all unit + integration tests

test-go: ## Go tests with race detector (includes the scenario suite)
	go test -race -shuffle=on ./...

test-skills: ## agent skill scripts (Node 18+)
	node --test skills/scripts.test.mjs

test-web: ## Vitest
	pnpm -C web test --run

scenarios: ## real-world scenario suite against mock systems, with a report
	cd e2e/scenarios && go test -count=1 -args -report ../../scenarios.json; status=$$?; cd ../.. && go run ./tools/scenarioreport scenarios.json; exit $$status

e2e: scenarios ## alias for scenarios

bench: ## rerun the token benchmark and rewrite bench/results.json
	cd e2e/scenarios && go test -count=1 -run 'TestScenarios/benchmark' -args -bench-out ../../bench/results.json
	@cat bench/results.json

dev-up: build ## mcpdeck dev with mock apps on 127.0.0.1:47800
	@mkdir -p .mcpdeck
	@$(BIN) dev --fixed-token --no-open --data .mcpdeck/dev > .mcpdeck/dev.log 2>&1 & echo $$! > .mcpdeck/dev.pid
	@for i in $$(seq 1 50); do curl -fs http://127.0.0.1:47800/healthz >/dev/null && exit 0; sleep 0.1; done; echo "mcpdeck dev did not start, see .mcpdeck/dev.log"; exit 1

dev-down: ## stop mcpdeck dev and delete its data
	@-kill $$(cat .mcpdeck/dev.pid 2>/dev/null) 2>/dev/null
	rm -rf .mcpdeck/dev .mcpdeck/dev.pid

lint: ## golangci-lint + Biome
	golangci-lint run
	pnpm -C web exec biome check .

lint-docs: ## regenerate docs/lint-rules.md from the rule catalog
	go run ./tools/lintdoc > docs/lint-rules.md

claims: ## every README claim names a test that exists
	go run ./tools/claimscheck

cover: ## unit coverage of internal/ with the floor CI enforces (45%)
	go test -coverprofile=cover.out ./internal/...
	@go tool cover -func=cover.out | awk '/^total:/{gsub("%","",$$3); if ($$3<45) {print "coverage "$$3"% < 45%"; exit 1} else print "coverage "$$3"%"}'

req-check: ## requirements with no REQ-tagged test yet
	@grep -oE '^\*\*[A-Z]+-[0-9]+' docs/requirements.md | tr -d '*' | sort -u | while read id; do \
	  grep -rqs "REQ: .*\b$$id\b" --include='*_test.go' --include='*.test.ts*' . || echo "$$id"; done

lint-actions: ## check workflow files (needs actionlint)
	actionlint

next-version: ## what merging to main would release (NFR-8)
	@go run ./tools/nextversion

release-check: ## build every release artifact locally and check it, like CI (needs goreleaser)
	goreleaser release --snapshot --clean --skip=publish,sbom
	go run ./tools/checkdist dist

sqlc: ## regenerate typed queries
	sqlc generate -f internal/store/sqlc.yaml

clean: ## remove build output and local dev data
	rm -rf bin dist cover.out scenarios.json .mcpdeck
