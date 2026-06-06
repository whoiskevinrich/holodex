# ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary)

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex's MCP server (Phase 2) must be reachable by AI assistant clients — Claude Desktop, VS Code extensions, custom agents. The MCP specification defines two transports: stdio (local process pipe) and Streamable HTTP (the `2025-03-26` spec, superseding the older SSE-only transport).

## Decision

Support **both transports** in the same server instance, with **HTTP/SSE as the primary target**:

- **HTTP/SSE** is the default and primary transport. Enabled when `MCP_ENABLED=true`.
- **stdio** is available as a secondary transport, enabled via `MCP_TRANSPORT=stdio` or `MCP_TRANSPORT=both`.
- Default port: `MCP_PORT=7801`.

## Rationale

- **HTTP/SSE is the ecosystem direction**: The MCP `2025-03-26` Streamable HTTP spec is where client implementations are converging. Claude Desktop supports it via the `url` config key — no `docker exec` required.
- **Network-friendly**: HTTP transport works across containers, remote machines, and reverse proxies without shell access to the Docker host. Consistent with the project's auth non-goal — the port can be wrapped with Authelia/Authentik at the proxy layer like the web UI.
- **stdio for local tooling**: `MCP_TRANSPORT=stdio` keeps compatibility with tools that expect a piped process (e.g. `claude mcp add holodex -- docker exec -i holodex holodex`). Low implementation cost since `mark3labs/mcp-go` handles both.
- **Single process**: Both transports share the same tool implementations and database connection — no duplication.

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `MCP_ENABLED` | `false` | Master switch; set `true` to start the MCP server |
| `MCP_TRANSPORT` | `http` | `http`, `stdio`, or `both` |
| `MCP_PORT` | `7801` | HTTP listener port (ignored when transport is `stdio`) |

## Client Configuration Examples

**Claude Desktop (HTTP)**
```json
{
  "mcpServers": {
    "holodex": {
      "url": "http://localhost:7801/mcp"
    }
  }
}
```

**Claude Desktop (stdio via docker exec)**
```json
{
  "mcpServers": {
    "holodex": {
      "command": "docker",
      "args": ["exec", "-i", "holodex", "holodex", "-mcp-transport", "stdio"]
    }
  }
}
```

## Consequences

- Port 7801 should be mapped in `docker-compose.yml` alongside port 7800 (web UI), but can be omitted if the user does not need networked MCP access.
- The HTTP endpoint path is `/mcp` (Streamable HTTP spec); SSE stream at `/mcp/sse` for older clients.
- Phase 2 security consideration: if `MCP_PORT` is exposed on a non-loopback interface, recommend placing it behind a reverse proxy with auth.
