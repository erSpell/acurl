# Web UI and JSON API

```bash
acurl serve
# acurl web UI on http://127.0.0.1:8080
```

Open the address, enter a URL and pick which kinds of agents to check. Progress streams in as
each agent finishes. The result page shows the AI readiness score, every agent's verdict and
what to fix. Reports download as HTML, Markdown, CSV or JSON.

Link straight to a check with `http://127.0.0.1:8080/?url=shop.example/products/1`.

## Options

| Flag | Default | Meaning |
|---|---|---|
| `--addr` | `127.0.0.1:8080` | Listen address. Use `0.0.0.0:8080` to share it on your network. |
| `--allow-private` | off | Allow checking private and internal addresses |
| `--render` | off | Allow headless-Chrome rendering checks |

All network flags (`--timeout`, `--proxy`, `--tls`, ...) apply to every check.

## Security

Anyone who can reach the server can make it fetch URLs. Connections to private, loopback,
link-local and cloud-metadata addresses are therefore blocked at connect time, which also
covers DNS names that resolve to internal IPs. At most four checks run at once. Keep the default
`127.0.0.1` address unless you trust your network.

## API

| Endpoint | Description |
|---|---|
| `POST /api/check` | Body `{"url": "...", "categories": [...], "agents": [...]}`. Returns the JSON report. |
| `GET /api/check/stream?url=...&categories=a,b` | Server-sent events: `start`, then `progress` per agent, then `report` or `error` |
| `GET /api/robots?url=...` | robots.txt audit with Content Signals, sitemaps and llms.txt |
| `GET /api/profiles` | The profile catalog |
| `POST /api/report?format=html` | Renders a JSON report as `html`, `markdown`, `csv`, `sarif` or `junit` |

```bash
curl -s -X POST localhost:8080/api/check -d '{"url":"shop.example","categories":["ai-assistant"]}' | jq .score
```

---

acurl is an open-source project by [tragent.io](https://tragent.io).
