# Changelog

Release notes for each version are generated from conventional commits and
published on [GitHub Releases](https://github.com/abhijeet-oxide/mcpDeck/releases).
This file collects user-visible changes before they ship; each release moves
them out of Unreleased.

## Unreleased

### Features

- Visual builder opens every shipped and generated tool. Steps reorder by
  drag (or keyboard) and the code follows; Arrange tidies the canvas; full
  screen; a legend explains the connectors; agent inputs are editable.
- Versions are a timeline with a highlighted diff against the previous,
  live or latest version, and Restore with confirmation.
- Requests: a timeout per request plus a default (30 s) and maximum (300 s)
  in Settings; Send shows the elapsed time and becomes Cancel; sends keep
  running when you leave the page, listed in a tray with jump-back and
  cancel; unsaved edits are kept until saved or reverted; saved responses
  are listed under their request; path variables are set on the Params tab.
- Proxy in Settings: system, none or manual HTTP/HTTPS with a no-proxy list
  and sign-in (password sealed, write-only); applied to every outbound call.
- Auth: requests that look like a sign-in are suggested first; captures are
  picked as From (body, header, cookie, custom) and a Field listed from the
  last response with sample values; "Use as bearer".
- Secrets show "Set · Encrypted" inline, and Settings → Security says where
  they and their key are stored.
- Build tabs explain themselves, and a first-run checklist includes sign-in.
- VS Code is listed first; search sits on the right of the top bar and finds
  requests; one light/dark toggle (system only until you choose); a gentle
  glass look that turns solid with reduced transparency.

### Fixes

- Switching auth to a login request no longer recreates an empty "token"
  credential or warns that requests will fail.
- Edits in Build were lost when opening Versions.
- The palette's "Settings: security" opened General.

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
