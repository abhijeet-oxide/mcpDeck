# How mcpDeck compares

Checked November 2026 against each product's own documentation (links at
the end). Products change quickly; if something here is out of date, open an
issue. We only describe what others do from their docs, and describe
mcpDeck by what it's designed to do, not by what others lack.

## The short version

Several products now turn existing APIs into MCP tools. They differ in where
they run, what you start from, and who they're built for.

| | Starts from | Runs | Tools are |
|---|---|---|---|
| **mcpDeck** | Swagger page, OpenAPI, Postman, curl, or requests you add by hand | One binary on your machine (team server later) | TypeScript functions over one or more requests |
| **IBM ContextForge** | REST and gRPC services, existing MCP servers | A gateway you deploy | Tools from REST/gRPC endpoints and MCP servers, grouped into virtual servers |
| **Speakeasy Gram** | OpenAPI documents, plus Gram Functions in TypeScript | Speakeasy's platform | One per operation by default, curated into toolsets; functions for custom logic |
| **Azure API Management** | REST APIs managed in API Management | Azure (Basic, Standard, Premium tiers) | Selected API operations |
| **Amazon Bedrock AgentCore Gateway** | OpenAPI, Smithy, Lambda, MCP servers | AWS | Operations from those targets, with optional semantic tool search |
| **Cloudflare MCP server portals** | Existing MCP servers | Cloudflare | The servers' own tools behind one endpoint, with optional code mode |

## Product by product

**IBM ContextForge** is an open-source gateway and registry. It can turn
REST APIs into MCP tools with JSON Schema extracted automatically, translate
gRPC services, federate existing MCP servers, and bundle tools into virtual
servers, each with its own endpoint. It has an admin UI. This is the closest
in shape to mcpDeck: toolkits and virtual servers solve the same "give each
agent a small, separate set of tools" problem.
*Where mcpDeck differs:* mcpDeck centres on building and improving the tools:
Postman-style requests and environments, multi-call TypeScript tools with
autocomplete from your APIs, tests and versions, rule-based suggestions and
field reports from real usage, and agent-built tools behind approval rules.
It ships as one local binary, Windows included.

**Speakeasy Gram** builds MCP servers from OpenAPI documents and from Gram
Functions (TypeScript snippets that can call several APIs). Tools are curated
into toolsets, and each toolset is an MCP server. Gram's docs note that the
default one-tool-per-operation set is usually too many tools for an LLM in
production, and recommend curating.
*Where mcpDeck differs:* you can start from Postman, curl or requests you add
by hand and explore them in a request runner before writing a tool; it runs
on your machine with credentials stored locally; and read-only access is
enforced per agent at the proxy by HTTP method.

**Azure API Management** can expose a REST API it already manages as a
remote MCP server, with the operations you select as tools, and keeps its
gateway features for authentication, authorization and monitoring. It
supports tools, not MCP resources or prompts, for REST-based servers.
*Where mcpDeck differs:* no Azure subscription or API Management instance,
and tools can combine calls across several APIs instead of mapping one
operation each.

**Amazon Bedrock AgentCore Gateway** turns OpenAPI and Smithy targets, Lambda
functions and MCP servers into tools behind one gateway, with an optional
search tool so agents can find the right tool by description.
*Where mcpDeck differs:* runs outside AWS on a laptop, and tools combine
several calls and shape their output.

**Cloudflare MCP server portals** put several existing MCP servers behind
one endpoint with Cloudflare's access controls. Code mode replaces the
individual tool definitions with a code-execution tool to save context.
*Where mcpDeck differs:* mcpDeck creates the tools for systems that have no
MCP server. A portal fronts servers that already exist; the two can be used
together.

## When to pick something else

- Your APIs already live in Azure API Management or behind AWS, and you
  want one tool per operation with your existing cloud governance: use the
  gateway you already run.
- You want a hosted service and start from a good OpenAPI document: Gram is
  built for that.
- You already have MCP servers and need one controlled front door for them:
  a portal or gateway fits; mcpDeck will add this later (Roadmap).

## Sources

- IBM ContextForge: <https://github.com/IBM/mcp-context-forge>
- Speakeasy Gram, tool sources and toolsets: <https://www.speakeasy.com/docs/gram/concepts/tool-sources>, <https://www.speakeasy.com/docs/gram/build-mcp/create-default-toolset>
- Azure API Management, expose a REST API as an MCP server: <https://learn.microsoft.com/azure/api-management/export-rest-mcp-server>, <https://learn.microsoft.com/azure/api-management/mcp-server-overview>
- Amazon Bedrock AgentCore Gateway: <https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/gateway.html>, <https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/gateway-using-mcp-semantic-search.html>
- Cloudflare MCP server portals and code mode: <https://developers.cloudflare.com/cloudflare-one/access-controls/ai-controls/mcp-portals/>, <https://blog.cloudflare.com/code-mode-mcp/>
