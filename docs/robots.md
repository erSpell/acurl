# robots.txt, Content Signals and llms.txt

`acurl robots <url>` answers "which agents may read this page?" without fetching the page:

```bash
acurl robots https://shop.example/products/1
acurl robots -f json https://shop.example/products/1
```

It reports:

- **Per-agent verdicts** from robots.txt, using Google's open-source matcher (RFC 9309)
- **Content Signals** (`Content-Signal: search=yes, ai-train=no`) that apply to each agent
- **Sitemaps** listed in robots.txt
- **llms.txt / llms-full.txt** presence, with format checks from [llmstxt.org](https://llmstxt.org)

## How robots.txt failures are handled

RFC 9309 defines what a crawler must do when robots.txt cannot be read:

| robots.txt response | Meaning |
|---|---|
| 2xx | Rules apply as written |
| 4xx (for example 404) | "Unavailable": every agent may crawl everything |
| 5xx or network error | "Unreachable": crawlers must assume **full disallow** |

acurl follows the same rules. An unreachable robots.txt makes `acurl robots` exit with code 2.

## Page-level directives

The main `acurl <url>` command also reads `X-Robots-Tag` headers and
`<meta name="robots">` tags, including agent-specific forms such as
`<meta name="GPTBot" content="noindex">` and `X-Robots-Tag: googlebot: nosnippet`.
Directives that limit indexing or AI use (`noindex`, `none`, `noai`, `noimageai`,
`nosnippet`) show in the Details column. JSON output always lists every directive.
