# UI design

How every screen works, and why. The clickable mockup is
[`mockups/index.html`](mockups/index.html): open it in a browser, or regenerate
the screenshots with `node docs/mockups/shoot.mjs`. It runs on the demo
and every control in it does something. The mockup is the reference for
layout and behaviour; it is never shown as the product (DOC-1).

## The mental model is the sidebar

A first-time user should understand mcpDeck by reading the sidebar from top to
bottom. Each item is one step of the same pipeline, in order:

| Sidebar | What lives there | The one way to... |
|---|---|---|
| **Home** | What needs you (reviews, tool requests, suggestions) and today's numbers. Until set up: the Getting started list | See what to do next |
| **Applications** | One application per system (your CRM, Jira, a SOAP service). Inside it: requests from any number of imports, environments, credentials | Create an application, add requests to it, store its credential |
| **Toolkits** | Toolkits (one MCP server each) and, under *All tools*, the tool library | Create a toolkit, create or edit a tool, set approval rules |
| **Agents** | One row per client (Claude Code on this laptop, Cursor...) and what each may do | Connect, change access, revoke |
| **Activity** | Calls, usage and the audit log | See who called what, what it cost, what changed |
| About | What mcpDeck does and how it works, the demo and its tour, security, and the GitHub repository: star, docs, releases, report a bug | Understand and find the project |
| Settings | Theme, port, timeouts, call history, setup guide, security, data | Change how mcpDeck runs |

The sidebar runs the full height of the window, like VS Code, Linear or Vercel:
logo and a light/dark switch at the top, then search (`Ctrl+K`), the five
places, and at the bottom the repository card, About, Settings and the
address mcpDeck is running on. Icons are [Lucide](https://lucide.dev).

There is one place for each thing and one path to it:

- **Requests are always added inside an API.** "Add API" and "Import" are the
  same page; Import is opened from inside an API and is locked to it. That
  removes the ambiguity of importing a spec without saying where it goes
  (the CLI requires `--api` for the same reason).
- **Tools are always created inside a toolkit.** The tool lands in the
  library and in that toolkit. "Add from library" puts an existing tool in a
  second toolkit. There's no free-floating "new tool" with no home.
- **Agents are always connected to a toolkit.** "Connect agent" asks which
  client, which toolkit and what access, then shows how to add it. The same
  dialog adds a second toolkit to an existing agent and rotates a token.
- **Everything recorded is in Activity.** Agent and toolkit pages link into
  it filtered, instead of having their own call tables.

## Patterns we copy

People have used these products for years. We take the pattern as it is and
change as little as possible.

| mcpDeck screen | Copied from | What we keep |
|---|---|---|
| Add API / Import | Postman's Import (file, link or raw text; curl pasted as text), Insomnia and Scalar import | Source picked first, then its input, then a preview of what will be imported with a checkbox per request |
| API workspace | Postman and Bruno collections | Tree on the left with folders, open-request tabs, method + URL bar, Params / Auth / Headers / Body / Scripts, response with Pretty / Raw / Headers / Timing, environment picker top right |
| History → Create tool | Postman History | Newest first; we add multi-select and "Create tool from selected" |
| Script trust | Bruno safe mode | Imported scripts off until trusted |
| Credentials | Postman collection auth, GitHub/Vercel secrets | Auth type set once and inherited; secrets write-only (Replace, never Reveal) |
| Toolkit page | Vercel/Supabase project "Connect" panels | Endpoint and copy button at the top, then tabs |
| Connect agent | Each client's own install path: `claude mcp add` (Claude Code docs), `vscode:mcp/install` links (VS Code), `cursor://anysphere.cursor-deeplink/mcp/install` links (Cursor), `claude_desktop_config.json` | The client's own command first, a one-click link where the client has one, the config file as the fallback |
| Token shown once | GitHub personal access tokens, Vercel tokens | Shown once with a warning, then only a prefix |
| Tool editor | VS Code + Postman scripts | Code in the middle, request list on the left that inserts calls, test panel on the right, `Ctrl+S` / `Ctrl+Enter`, problems under the editor |
| Test run timeline | Chrome DevTools Network | One bar per API call on a shared time axis |
| Versions | GitHub file history, VS Code Timeline | Timeline on the left; the selected version against the one before it (or live, or latest) as a highlighted diff; "Restore this version" confirms, then makes a new version |
| Review a version | GitHub pull request with checks | Diff on the left, checks with pass/fail on the right, Approve / Reject |
| Needs you | Linear Inbox, GitHub notifications | One list of things waiting for a decision, each with its action inline |
| Calls | Chrome DevTools Network | Table with status, duration, size and a waterfall when one session is selected; detail pane with tabs |
| Usage | Vercel / Stripe dashboards | Range switch, a chart, then a sortable table |
| Command palette | VS Code, Linear, Raycast | `Ctrl+K` / `⌘K`, groups, actions as well as places |
| Settings | macOS System Settings | Sections on the left, plain rows on the right |
| Sidebar | VS Code, Linear, Vercel | Full height, icons with labels, secondary items at the bottom |
| About | macOS "About this app", open source project pages | What it is, where it lives, how to help |

Sources for the client install paths: [Claude Code MCP docs](https://code.claude.com/docs/en/mcp),
[VS Code MCP servers](https://code.visualstudio.com/docs/copilot/customization/mcp-servers),
[Cursor MCP install links](https://cursor.com/docs/mcp/install-links),
[Postman import](https://learning.postman.com/docs/getting-started/importing-and-exporting/importing-data/).
The VS Code `vscode:mcp/install` link format is used by many vendors but
isn't in VS Code's reference docs; we always show the `.vscode/mcp.json`
snippet next to it.

## Building blocks

New UI is made from the components that already exist. Don't add a widget
when one of these fits; if none does, use the established pattern named in
the table above and add it to `web/src/components/ui.tsx` with a test.

| Need | Use | Pattern |
|---|---|---|
| Primary and secondary actions | `Button` (primary on the right of a row or dialog footer) | macOS and GitHub dialogs |
| Page title, subtitle and page actions | `PageHead` | Vercel, Linear |
| Grouped content | `Panel` | GitHub settings boxes |
| Switch between views of one thing | `Tabs` (Radix Tabs) | Postman request tabs |
| Two to four exclusive options | `Seg` | macOS segmented control |
| Pick one of many | `Select` (Radix Select) | native select |
| Row or page actions that don't fit | `Menu` (Radix Dropdown Menu) | GitHub "..." menu |
| Decide or confirm | `Dialog`, `useConfirm` | macOS sheets |
| Edit beside a list | `Drawer` | Linear side panel |
| Nothing here yet | `EmptyState` with one action | GitHub empty repositories |
| Form row | `Field`, `Toggle`, `KVEditor` for headers and variables | Postman key-value tables |
| Unsaved changes | `SaveBar` | macOS System Settings |
| Status, method, access | `StatusChip`, `MethodChip`, `AccessChip` | Postman method colours |
| Lint and checks | `Findings` (block, warn, hint, with Why links) | VS Code Problems panel |
| Feedback after an action | `useToast` | GitHub flash messages |
| Copy a value | `CopyButton`, `CopyField` | GitHub clone box |
| Code | Monaco (`web/src/lib/monaco.ts`) | VS Code |
| Search everything | `Palette` (`Ctrl+K`) | VS Code, Linear |
| Hints on hover | `Tip` (Radix Tooltip) | native tooltips |

Rules: primary actions sit on the right; one primary action per view;
destructive actions ask with `useConfirm` and name what they delete; every
control works (UI-4); keyboard focus is visible; layouts hold from 1280x720
to 1440x900 and wider without horizontal scrolling.

## Built for working with agents

Most people will drive mcpDeck through an agent. The UI makes that the short
path.

1. **"Ask an agent" on every working page.** Home, an API, a toolkit and a
   tool each have the same button. It opens two or three prompts written for
   that page, using real names (`search_apis`, `save_tool`, `crm.getAccount`,
   the toolkit's name), and says which connected agent can run each one
   ("Paste into Claude Code: it can build in support"), or what's missing
   ("No agent can build in finance yet").
2. **Every request from an agent has a "Copy prompt".** A tool request on
   Home or a toolkit becomes a prompt for an agent that can build it, with
   `fulfils: <id>` so the request closes when the tool goes live.
3. **New tool starts with "Ask an agent".** Describe the tool in one line;
   the dialog writes the prompt. "From requests" and "Write it myself" are
   the other two choices.
4. **Connecting is one command or one click,** and finishes with a first
   prompt to try once the client connects.
5. **Errors are copyable for the agent.** A failed or blocked call has "Copy
   fix prompt", which includes the error, the input and what to do next.
6. **Suggestions can be handed to an agent.** Each suggestion has the prompt
   that makes the change, and the result still goes through approval rules.
7. **Skills install with the connection,** so agents also learn to report
   fields and ask for tools without being told.

Prompts are short, name exact tools and inputs, never contain secrets, and
always end with what to check (`test_tool` before `save_tool`).

## Screens

Screenshots are from the mockup (`docs/mockups/screens/`).

### First run, the tour, and a guide that goes away

The first time mcpDeck opens with no data, a **welcome page** explains the
three ideas (applications → toolkits → agents) and offers "Take the 2-minute
tour" or "Set up my own". It's shown once (`welcomeSeen`).

**The demo is kept apart** (UI-12). The tour opens the demo in a new tab: its
own instance with sample applications and temporary data, in the same theme.
A blue bar on every page says so, with "Tour" and "Leave demo". The tour is a
small card in the corner, eight steps (applications, a request, toolkits,
what an agent sees, agents, activity), Back/Next/close at every step, and ends
with "Leave demo and start", which returns to the user's own mcpDeck, still
signed in, on Home.

**Getting started** (UI-11) follows the journey: create an application,
send a request, save a response as an example, create a toolkit, build a
tool, connect an agent. Any order; a step that needs another says which. Home
shows the list; a small card at the bottom of the sidebar shows the next step
on every page. Pages suggest the next step in place: after a successful send,
"Save as example"; after saving one, "Create tool". Hide it from the card or
Home; bring it back in Settings.

The Demo button is on Home (top right) and About, always.

### Home

![Home](mockups/screens/10-home.jpg)

Four cards (Applications, Toolkits, Agents, Calls today) with counts restate
the mental model; each opens its page. **Needs you** lists, in order: versions waiting for review,
tool requests from agents, suggestions. **Today** shows calls, tokens returned,
errors and blocked writes, and the latest calls.

### New application

A page, not a dialog (UI-14): sources on the left, the input in the middle,
then every request found, each with a checkbox, its method, path, folder and
whether it reads or writes. The API name is shown as code because that's how
tools call it.

**Swagger or Redoc page.** Paste the docs page URL; mcpDeck finds the spec the
page loads.

![Swagger page](mockups/screens/02a-import-swagger.jpg)

**OpenAPI file or URL.**

![OpenAPI](mockups/screens/02b-import-openapi.jpg)

**Postman collection**, with its environment if you have one. Secret
environment values become encrypted credentials; scripts come in switched off.

![Postman](mockups/screens/02c-import-postman.jpg)

**curl.** The list updates as you paste. Credential headers and query keys are
moved into an encrypted credential.

![curl](mockups/screens/02d-import-curl.jpg)

**Start empty.** A name, a base URL and an auth type.

![Start empty](mockups/screens/02e-import-blank.jpg)

After importing, the API opens with a note of what was added and, if a
credential is needed, a button to add it:

![After import](mockups/screens/03-api-after-import.jpg)

### API

![API requests](mockups/screens/12-api-requests.jpg)

Postman's layout. The access row under the URL bar shows whether the request
reads or writes and why ("POST, marked read: it only searches"). Requests
that can't be classified (SOAP) say so and ask (API-1):

![Check access](mockups/screens/13-api-legacy-check-access.jpg)

The response bar shows size and tokens, so people see what an agent would
read. "Create tool" and "Copy as tool call" lead straight into the editor.

![Credentials](mockups/screens/14-api-credentials.jpg) ![Environments](mockups/screens/15-api-environments.jpg)

### Toolkits and tools

![Toolkits](mockups/screens/16-toolkits.jpg) ![All tools](mockups/screens/17-all-tools.jpg)

A toolkit card has its endpoint with a copy button, its agents and the
`tools/list` cost. *All tools* is the library.

![Toolkit](mockups/screens/18-toolkit.jpg)

Tabs: Tools, Agents (access and "May build" per agent, changed in place),
Requests and reviews, Settings (build list, approval rules, variables,
response hints).

![Toolkit agents](mockups/screens/19-toolkit-agents.jpg) ![Requests](mockups/screens/20-toolkit-requests.jpg) ![Settings](mockups/screens/21-toolkit-settings.jpg)

### New tool

![New tool](mockups/screens/34-new-tool-ask-agent.jpg)

### Tool editor

![Tool editor](mockups/screens/23-tool-test-run.jpg)

Requests on the left insert a call. Typing `crm.` lists that API's requests
with method and access:

![Autocomplete](mockups/screens/22b-tool-autocomplete.jpg)

The test panel shows the size of the API responses and of the output, the
saving, a timeline of the calls and the output.

![Versions](mockups/screens/24-tool-versions.jpg) ![Output fields](mockups/screens/25-tool-fields.jpg)

Output fields shows each field's share of the output, which agents use it and
which reported it unused. "Leave out for..." changes one agent's output only
(OUT-1).

### Review

![Review](mockups/screens/26-review.jpg)

Like a pull request: the code, the approval rules with the one that stopped
it, the requests it sends with their access, and who gets it when approved.

### Connect an agent

![Pick client](mockups/screens/06-connect-pick-client.jpg) ![Connected](mockups/screens/07-connect-done.jpg)

Client, toolkit, name, access, may build, install skills. Then the token,
shown once, inside the client's own command or link, and a live "waiting for
the client" state that turns into "Connected" and a first prompt to try.

### Agents

![Agents](mockups/screens/27-agents.jpg) ![Agent](mockups/screens/28-agent.jpg)

### Activity

![Calls](mockups/screens/29-activity-calls.jpg) ![Blocked call](mockups/screens/30-activity-blocked-call.jpg)
![Usage](mockups/screens/31-activity-usage.jpg) ![Audit](mockups/screens/32-activity-audit.jpg)

### Suggestions, prompts and the palette

![Suggestion](mockups/screens/36-suggestion.jpg) ![Ask an agent](mockups/screens/35-ask-an-agent.jpg) ![Palette](mockups/screens/37-command-palette.jpg)

### About

![About](mockups/screens/38-about.jpg)

What mcpDeck does in four lines, how keys are handled, and the repository:
Star on GitHub, Report a bug, Docs, Releases, Contribute, License (UI-13).

### Settings, dark mode, phone width

![Settings](mockups/screens/33-settings.jpg) ![Dark](mockups/screens/40-home-dark.jpg) ![Phone](mockups/screens/42-home-phone.jpg)

## Words on screen

- Say what happens: "Approve and publish", then "reassign_ticket is live for
  1 agent with read and write on support".
- Errors say what to do: "No credential for crm in Staging. Add it on the
  Auth and credentials tab."
- Numbers are labelled with their source: "tokens returned, counted by
  mcpDeck".
- Placeholders are real example values (`https://petstore3.swagger.io/`,
  `overdue_invoices`), never instructions.
- No words from the UI-5 list, no emoji, no "coming soon".

## Mockup and product

The screenshots above come from the mockup, the layout reference. The product
follows it; where they differ, the product and its tests are the truth, and
the README shows only what the product does.

## What the mockup fakes

The mockup has no server, so a few things are simulated. Everything else is
real logic over the demo.

- Network: URL imports only recognise the Petstore and demo-world URLs. curl
  parsing and JSON file imports (OpenAPI, Swagger, Postman collections and
  environments) are real; try the files in `docs/mockups/fixtures/`.
- Sending a request returns the demo's saved response.
- "Connected" appears a few seconds after you copy the command or write the
  config; the product waits for the client's real `initialize`.
- Tokens are bytes ÷ 4; the product uses a named encoder (SES-3).
- Problems under the editor are a few static checks; the product uses the
  TypeScript language service.
