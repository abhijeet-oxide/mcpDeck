# Vision

## One line

mcpDeck is a lightweight, local, controlled layer where agents and people turn
any API into reviewed, versioned, reusable tools. Agents use them over MCP or as
skills, credentials stay in a vault, and every call is traced and audited.

## The problem

1. **MCP servers are slow to build.** Each tool means code, a build and a
   redeploy, and every change means another one.
2. **Most systems have no MCP.** Legacy and internal systems have REST, SOAP, a
   spec, or nothing.
3. **Most MCP servers are badly designed.** One tool per endpoint, huge
   responses, several calls to do one job, wasted tokens.
4. **Code execution throws work away.** Agents write glue code that works, then
   disappears. The next session pays again in tokens, time and risk. The code is
   also different each run, so it's hard to review or audit.
5. **Credentials leak into agents.** Through env vars, prompts and generated
   code.
6. **There's no central control.** Nobody can say which agent can do what, on
   which system, or what it did last week.
7. **Tools don't improve.** Nothing measures which tools are slow, expensive or
   misused, so nothing gets better.

## The core idea

> Agents explore once. mcpDeck makes it repeatable.

Code execution is the discovery phase: the agent finds APIs and works out how
to use them. mcpDeck is the production phase: it captures what worked as a
tool, versions it, puts it behind access rules, and measures every call. Over
time agents write less code and call more tools, and those tools get cheaper,
faster and safer.

## Mental model

```
Application → APIs → Credentials → Examples
Project → Tools (one or more APIs, from one or more applications)
Agent → Access (which tools; read / read+write / needs approval)
Session → every call, with time and tokens → audit log
```

1. Create an **application** (Jira, an internal billing API, a SOAP service).
2. Add its **APIs** from OpenAPI, Postman, curl or HAR.
3. Add **credentials** (local vault or an external vault).
4. **Run** APIs and save good responses as **examples**.
5. Create a **project**. Tools live in projects and can use APIs from any
   application.
6. Create **tools**: in the no-code builder, or by asking a local agent
   (Claude Code, Copilot, Cursor) through the mcpDeck skill. A tool carries the
   context an agent needs: what it does, when to use it, when not to.
7. **Test** the tool locally and save. Each save is a new version.
8. Create **agents** and give them tools. Access follows the HTTP methods
   underneath: GET is read, POST/PUT/PATCH/DELETE is write, with overrides.
9. Open an agent and see every **session**: each call, its duration and tokens.
   Use that to merge, trim and fix tools.

## Principles

1. **Lightweight.** One binary, SQLite, running in under a minute.
2. **Agent-agnostic.** Claude Code, Copilot, Cursor and any MCP client.
3. **People set the limits, agents work inside them.** An agent can never widen
   its own access, approve a write tool or read a secret.
4. **Agents never see credentials.** Secrets are injected at the proxy.
5. **Least exposure.** Each agent sees only the tools it's granted.
6. **Never break a caller.** Tools are versioned; saving never changes what a
   pinned agent gets.
7. **Measured, not claimed.** Time, tokens and calls are recorded per session;
   every optimisation shows before and after.
8. **Tools are data.** Versioned, diffable, testable, exportable. Never locked
   inside a GUI.
9. **Simple UI.** Four places: Applications, Projects, Agents, Settings.

## Users

| User | Wants |
|---|---|
| Platform / AI engineer | Expose company systems to agents safely without writing an MCP server for each |
| Developer using an agent | An agent that can do real work across internal systems |
| Security / platform owner | Central control of credentials, access and audit |
| Team lead | Tools that improve over time and are shared across the team |
| The agent itself | Good tools with clear descriptions, small responses, and a way to build or request what's missing |

## Non-goals

- Not a hosted SaaS (self-hosted / local first).
- Not a workflow automation product (no cron, no triggers).
- Not an agent framework or chat UI.
- Not an API gateway for human traffic.
- Not a model host. Running tools never needs an LLM.

## Success measures

- Install to first agent tool call: under 5 minutes.
- Demo task: calls, tokens and time cut by at least 70% after a tool replaces raw calls.
- Share of agent calls going through tools rather than raw APIs rises week over week.
- Zero credentials ever returned to an agent (enforced by a CI test).

## Name

**mcpDeck.** A deck is a set of cards you deal from. Here the cards are tools:
each agent is dealt only the tools it needs, and a deck is also the control
surface where people manage agents, access and sessions. Keep the name in one
place (`internal/version`, the CLI root command, the UI title) so changing it
later stays cheap.
