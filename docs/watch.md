# Watching for changes

AI agent access breaks quietly. A WAF rule update, a new bot-management default or a CDN change
can start challenging ChatGPT-User overnight. `acurl watch` re-checks pages and tells you when
anything changes.

```bash
acurl watch https://shop.example https://shop.example/products/best-seller --interval 1h
```

```
2026-10-04 09:00:12 https://shop.example: ChatGPT-User allowed -> challenged
2026-10-04 09:00:12 https://shop.example: AI readiness 92 (A) -> 74 (C)
```

The first run records a baseline. After that, acurl prints every change in a profile's
classification, and every score change of at least `--min-score-delta` points (default 5).

## Alerts

```bash
# Slack, Mattermost or any Slack-compatible incoming webhook
acurl watch https://shop.example --webhook https://hooks.slack.com/services/... --webhook-format slack

# Generic JSON for your own tooling
acurl watch https://shop.example --webhook https://ops.example/hooks/acurl
```

The JSON payload:

```json
{
  "event": "acurl.changes",
  "timestamp": "2026-10-04T03:00:12Z",
  "changes": [
    { "url": "https://shop.example", "profile": "ChatGPT-User", "from": "allowed", "to": "challenged", "worse": true }
  ]
}
```

## Running from cron or CI

With `--interval 0`, acurl checks once, compares the results with a state file, and exits:

```cron
0 * * * *  acurl watch https://shop.example --interval 0 --state /var/lib/acurl/state.json --webhook https://...
```

In a scheduled GitHub Actions workflow, keep the state file in the Actions cache between runs.

---

acurl is an open-source project by [tragent.io](https://tragent.io).
