# Changelog

Release notes for each version are generated from conventional commits and
published on [GitHub Releases](https://github.com/abhijeet-oxide/mcpDeck/releases).
This file collects user-visible changes before they ship; each release moves
them out of Unreleased.

## Unreleased

### Features

- Auth for real systems: login chains with captures, refresh and re-login on
  401 or a login redirect; OAuth 2.0 client credentials and password grants
  with refresh-token rotation; HMAC request signing; mutual TLS.
- Import reads OAuth 2.0 security schemes, relative server URLs, `text/plain`
  and XML bodies, and curl commands that mix auth styles.
- Tools get `.raw()` for headers and status, `sleep()` for polling, `xml()`
  for SOAP and XML, and a per-tool timeout.
- An agent can do every setup step without the UI: the admin MCP server
  (`mcpdeck mcp`) and CLI commands `api`, `auth set`, `request`, `secret`,
  `var`, `toolkit`, `tool generate|save|test|approve`, all with `--json`.
- Lint rules have severities (block, warn, hint) and cited sources; saves
  with a blocking finding are refused with the fix. `mcpdeck lint` exits 1
  on a block, and on warnings with `--strict`.
- Tool results over 20,000 tokens are cut to valid JSON with a note, instead
  of being cut mid-JSON by the client.
- `mcpdeck --version`, `mcpdeck stop` stops dev mode, `mcpdeck help --json`.

### Fixes

- Without an OS keychain, mcpDeck says the data key is in a private file.
- Database and key files are created readable only by you.
- `mcpdeck dev` uses a random agent token unless `--fixed-token` is given.
- `mcpdeck secret set` keeps multi-line values such as PEM keys.
- Generated tools take their description from the spec's summary.
