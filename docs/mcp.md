# acurl for AI assistants (MCP)

`acurl mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio.
Claude, Cursor and other MCP clients can then run acurl checks while you work. Ask
*"can ChatGPT read our new product page?"* and the assistant fetches the page and answers.

## Tools

| Tool | What it does |
|---|---|
| `check_url` | Full check: every agent's classification, details, AI readiness score and fixes. Accepts `categories` and `agents` filters. |
| `readiness_score` | Just the score, the failed checks and how to fix them |
| `robots_audit` | robots.txt verdict per agent, Content Signals, sitemaps and llms.txt |
| `list_profiles` | The agent profiles acurl knows, optionally filtered by `category` |

All tools are read-only. Results come back as Markdown for the model, and as structured
JSON (`structuredContent`) for clients that use it.

## Setup

**Claude Code**

```bash
claude mcp add acurl -- acurl mcp
```

**Claude Desktop** (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "acurl": { "command": "acurl", "args": ["mcp"] }
  }
}
```

**Cursor** (`~/.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "acurl": { "command": "acurl", "args": ["mcp"] }
  }
}
```

If acurl is not on the PATH the client uses, give the full path to the binary
(`which acurl`).

## Options

Network flags work as on the command line, for example `acurl mcp --timeout 20s --proxy http://proxy:3128`.

For safety, requests to private, loopback and cloud-metadata addresses are blocked, so a
prompt-injected page cannot point acurl at your internal network. To check a local
development server, start the server with `acurl mcp --allow-private`.

---

acurl is an open-source project by [tragent.io](https://tragent.io).
