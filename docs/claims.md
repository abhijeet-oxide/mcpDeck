# Claims and their tests

Every factual claim in the README carries a marker such as `<!-- C4 -->`.
Each marker has one row here naming the tests that prove it.
`go run ./tools/claimscheck` runs in CI and fails when:

- a marker has no row,
- a row has no test, or names a test that doesn't exist,
- a row isn't used in the README.

Tests are written `scenario:<category>/<name>` (see
[e2e/scenarios](../e2e/scenarios)) or `go:<TestFunc>`.

**Adding a claim:**

1. Write the test first.
2. Add the row here.
3. Then write the sentence with its marker.

**Removing a feature:** remove its claim in the same change. Claims never
compare mcpDeck with other products, and every number comes from a test or
from committed raw output.

| ID | Claim | Tests |
|---|---|---|
| C1 | One tool chains several calls into one agent call | `scenario:chaining/shop login token feeds the orders call` `scenario:chaining/jfrog AQL to storage checksum to Xray summary` `scenario:chaining/kube crashing pods with the error lines of their logs` |
| C2 | Login chains, refresh, OAuth 2.0, HMAC, mTLS and a company CA work | `scenario:auth/shop token expiry triggers a fresh login` `scenario:auth/OAuth2 password grant with refresh token rotation` `scenario:auth/payments HMAC-signed GET` `scenario:auth/vault mTLS client certificate plus token` `scenario:parity/admin MCP onboards OAuth 2.0 behind a company CA` |
| C3 | Agents can do what the UI does; a person approves writes | `scenario:parity/admin MCP onboards a login API end to end with no UI or CLI` `scenario:parity/admin MCP write tool waits for a person; the agent can't approve it` `scenario:cli/help --json describes every command for agents` |
| C4 | A read agent can't run a write tool and the proxy refuses its writes | `scenario:access/read agent can't run a write tool` `scenario:access/read tool can't smuggle a write through call()` `go:TestDo_ReadGrantBlocksEveryWrite` `go:TestReadGrantCantWriteThroughBuilder` |
| C5 | The terminal quickstart works step by step | `scenario:cli/README terminal path: import, auth, toolkit, tool, connect, call` `scenario:onboarding/anchore Swagger 2.0 relative basePath resolves to the spec host` |
| C6 | connect prints the client's command or config, and the agent's call carries the key | `scenario:cli/README terminal path: import, auth, toolkit, tool, connect, call` `go:TestClaudeCode_KeepsExistingEntriesAndAddsBuilderSubagent` `go:TestVSCode_JSONCConfigLinkAndInstructions` `go:TestCursor_Deeplink` `go:TestClaudeDesktop_UsesStdioBridge` |
| C7 | Tools run sandboxed; every call goes through the proxy | `go:TestSandbox_EscapeSuite` `go:TestSandbox_PrototypePollutionStaysInRun` `go:TestDo_RefusesHostsNotAllowed` `scenario:secrets/builder run_script can't read the token` |
| C9 | The token benchmark reproduces from one command | `scenario:benchmark/token benchmark reproduces the committed numbers` |
| C10 | Auth is set once per API and tools never handle credentials | `scenario:builder/a builder can't see or change auth settings` `scenario:secrets/an echo endpoint can't reveal the API key` |
| C11 | Login captures are reused and renewed on expiry, 401 or a login redirect | `scenario:auth/shop login is reused across calls, not repeated` `scenario:auth/shop refresh token is used instead of the password` `scenario:auth/shop token expiry triggers a fresh login` `scenario:auth/legacy expired session redirect triggers a fresh login` `go:TestDo_LoginRequestKeepsCookie` |
| C12 | CSRF login forms with session cookies | `scenario:auth/legacy CSRF login page, form post, session cookie` `scenario:auth/legacy CSRF header on writes comes from the login` |
| C13 | OAuth 2.0 client credentials and password grants, secret in basic or body, rotation | `scenario:auth/OAuth2 client credentials token fetched and used` `scenario:auth/OAuth2 client secret sent in the body instead of basic` `scenario:auth/OAuth2 password grant with refresh token rotation` `scenario:auth/OAuth2 token renewed after expires_in` |
| C14 | API keys, bearer and basic from specs and curl | `scenario:auth/API key in a header` `scenario:auth/API key in the query string` `scenario:auth/anchore basic auth from the spec's security scheme` `scenario:onboarding/jfrog curl import moves the -u password into a secret` `go:TestCurl_MovesCredentialsOut` |
| C15 | HMAC signing with a configurable string to sign | `scenario:auth/payments HMAC-signed GET` `scenario:auth/payments HMAC signs the body of a POST` `scenario:auth/payments signs path parameters and query strings exactly` |
| C16 | Client certificates, extra CAs and skipped verification per API | `scenario:auth/vault mTLS client certificate plus token` `go:TestDo_TLSSettingsPerAPI` `scenario:parity/admin MCP onboards OAuth 2.0 behind a company CA` |
| C17 | GraphQL, SOAP, text bodies, downloads, 202 polling, pagination styles and retries | `scenario:graphql/GraphQL cursor pagination with endCursor` `scenario:payloads/legacy SOAP call parsed with xml()` `scenario:payloads/jfrog AQL is sent as text/plain` `scenario:payloads/PDF download reaches the tool as metadata` `scenario:async/export job: 202, poll Location, download CSV` `scenario:pagination/billing cursor pagination with starting_after and has_more` `scenario:pagination/shop page and per_page until total_pages` `scenario:pagination/jfrog AQL offset and limit until a short page` `scenario:pagination/github Link header rel=next followed with .raw()` `scenario:resilience/429 with Retry-After is retried` `scenario:resilience/503 on a GET is retried once` |
| C18 | The use endpoint's instructions and tools/list are small | `scenario:consumer/server instructions and tools/list stay small` |
| C19 | Over 25 tools, the top 15 are listed and the rest are found by search | `scenario:consumer/a 30-tool toolkit lists 15; search_tools and use_tool reach the rest` `go:TestLeastExposure_BigToolkitListsTopToolsAndUseTool` |
| C20 | request_tool records a need and grants nothing | `scenario:consumer/request_tool records the need but grants nothing` |
| C21 | Oversized results are cut to valid JSON with a note | `scenario:consumer/an output over the token budget is cut to fit, with a hint` `scenario:payloads/huge response reaches the agent as valid JSON under budget, with a note` `go:TestFitOutput` |
| C22 | Builder agents build and test tools; reads go live, writes wait | `scenario:builder/save_tool read goes live, write waits` `scenario:builder/test_tool runs against the live API` `scenario:builder/search_apis and describe_api find the right request` `go:TestBuilderSubagentBuildsToolForMainAgent` |
| C23 | The admin MCP server onboards an API, can't widen access or read secrets, is audited and small | `scenario:parity/admin MCP onboards a login API end to end with no UI or CLI` `scenario:parity/admin MCP can't widen access or see secret values` `scenario:parity/admin MCP actions are audited under the agent's name` `scenario:parity/admin MCP context an agent reads stays small` |
| C24 | Lint runs everywhere, cites sources, and blocks only clear violations | `scenario:lint/the editor lints an unsaved draft as you type` `scenario:lint/mcpdeck lint --json and --strict for CI` `scenario:lint/admin MCP save_tool returns lint findings to the agent` `scenario:lint/an invalid input schema is refused` `scenario:lint/saving a tool with a hard-coded token is refused, with the rule and fix` `scenario:lint/a write tool that claims readOnlyHint is refused` `go:TestRules_OnlyClearViolationsBlock` `go:TestRules_EachRuleHitsAndMisses` |
| C25 | A canary secret never reaches outputs or recorded calls | `go:TestDo_SecretNeverInResponseOrRecord` `go:TestBuilderSubagentBuildsToolForMainAgent` `scenario:secrets/an echo endpoint can't reveal the API key` `scenario:secrets/call log holds no credentials after the whole suite` |
| C26 | Secrets are encrypted with the key in the keychain, or a private file that is announced | `go:TestVault_KeyInKeychainNotOnDisk` `go:TestVault_FallsBackToKeyFile` `scenario:install/no-keychain fallback is announced` `scenario:install/database and key files are private (0600)` |
| C27 | Localhost only, Host and Origin checks, one-time launch link | `go:TestGuard_HostOriginCookieToken` `go:TestGuard_SessionCookiePerPort` |
| C28 | The audit log detects edited rows | `go:TestVerify_DetectsEditedRow` `go:TestStore_AuditRowsCannotBeChanged` |
| C29 | The installers verify checksums and install for the user | `go:TestInstaller` |
| C30 | Browser sign-in for web apps behind company SSO; cookies stay in the proxy | `scenario:webapp/an agent lists claims and submits a claim with a receipt from the folder once the person confirms` `go:TestBrowserSignIn_OpensOnceAndKeepsOnlyThePortalsCookies` `go:TestBrowserSignIn_ExpiryRetriesReadsButNeverResendsWrites` |
| C31 | Record a task once in the browser; the agent gets its pages and form fields, no cookies or values | `scenario:webapp/record_workflow turns one clicked-through task into requests with form fields, without cookies or values` |
| C32 | Tools read forms and tables, upload files from a folder the person picks, and wait for the person to confirm each submit | `scenario:webapp/an agent lists claims and submits a claim with a receipt from the folder once the person confirms` `scenario:webapp/a declined confirmation sends nothing` `go:TestUpload_SendsMultipartFromTheFilesFolderOnly` `go:TestParseForms` |
| C33 | A toolkit exports to one file without secrets; importing it leaves write tools waiting for approval | `scenario:webapp/the toolkit exports to a file without secrets and imports elsewhere with its write tool waiting for approval` |
