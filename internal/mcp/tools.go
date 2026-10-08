package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tragent-io/acurl/internal/llmstxt"
	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/internal/profiles"
	"github.com/tragent-io/acurl/internal/robots"
	"github.com/tragent-io/acurl/pkg/acurl"
	"github.com/tragent-io/acurl/pkg/types"
)

const instructions = `acurl checks how AI agents (ChatGPT, Claude, Perplexity and others), search
crawlers and a real Chrome browser are treated by a web page: blocked, challenged by a WAF,
cloaked, disallowed in robots.txt, or allowed. It also scores how ready the page is for AI
agents and explains how to fix problems. Requests come from the user's machine, so results show
how the site responds to agent user agents, not guaranteed agent access.`

// New returns an MCP server exposing acurl tools. base holds the network
// options for every check; set base.BlockPrivate unless every caller is trusted.
func New(base acurl.Options, version string) (*Server, error) {
	if base.Profiles == nil {
		ps, err := profiles.Default()
		if err != nil {
			return nil, err
		}
		base.Profiles = ps
	}
	t := &tools{base: base}
	return &Server{
		Name:         "acurl",
		Version:      version,
		Instructions: instructions,
		tools: []Tool{
			{
				Name:  "check_url",
				Title: "Check how AI agents see a URL",
				Description: "Fetch a URL as a real browser and as AI agents and crawlers, classify how each was treated " +
					"(allowed, blocked, challenged, cloaked, rate-limited, robots-disallowed), and return the AI readiness score with fixes.",
				InputSchema: schema(map[string]any{
					"url":        prop("string", "The page to check, e.g. https://shop.example/products/1"),
					"categories": arrayProp("Only these agent categories: ai-assistant, ai-search, ai-training, search, other"),
					"agents":     arrayProp("Only these profile names, e.g. ChatGPT-User, GPTBot"),
				}, "url"),
				Annotations: readOnly(),
				handle:      t.check,
			},
			{
				Name:        "readiness_score",
				Title:       "AI readiness score",
				Description: "Return only the AI readiness score (0-100), the failed checks and how to fix them for a URL.",
				InputSchema: schema(map[string]any{"url": prop("string", "The page to score")}, "url"),
				Annotations: readOnly(),
				handle:      t.score,
			},
			{
				Name:        "robots_audit",
				Title:       "Audit robots.txt for AI agents",
				Description: "Report which AI agents robots.txt allows for a URL, Content-Signal directives, sitemaps and llms.txt status. Does not fetch the page itself.",
				InputSchema: schema(map[string]any{"url": prop("string", "Any URL on the site")}, "url"),
				Annotations: readOnly(),
				handle:      t.robots,
			},
			{
				Name:        "list_profiles",
				Title:       "List agent profiles",
				Description: "List the AI agent and crawler profiles acurl can check, with category and operator.",
				InputSchema: schema(map[string]any{"category": prop("string", "Optional category filter")}),
				Annotations: readOnly(),
				handle:      t.list,
			},
		},
	}, nil
}

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func prop(typ, desc string) map[string]any { return map[string]any{"type": typ, "description": desc} }

func arrayProp(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func readOnly() map[string]any {
	return map[string]any{"readOnlyHint": true, "openWorldHint": true}
}

type tools struct {
	base acurl.Options
}

type urlArgs struct {
	URL        string   `json:"url"`
	Categories []string `json:"categories"`
	Agents     []string `json:"agents"`
	Category   string   `json:"category"`
}

func parseArgs(raw json.RawMessage, needURL bool) (urlArgs, error) {
	var a urlArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return a, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if needURL {
		a.URL = strings.TrimSpace(a.URL)
		if a.URL == "" {
			return a, errors.New("url is required")
		}
		if !strings.Contains(a.URL, "://") {
			a.URL = "https://" + a.URL
		}
	}
	return a, nil
}

func (t *tools) run(ctx context.Context, a urlArgs) (*types.ComparisonResult, error) {
	opts := t.base
	opts.Profiles = profiles.FilterCategories(profiles.Filter(opts.Profiles, a.Agents), a.Categories)
	if len(opts.Profiles) == 0 {
		return nil, errors.New("no profiles match the given agents or categories; call list_profiles to see them")
	}
	c, err := acurl.New(opts)
	if err != nil {
		return nil, err
	}
	return c.Check(ctx, a.URL)
}

func (t *tools) check(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	a, err := parseArgs(raw, true)
	if err != nil {
		return nil, err
	}
	rep, err := t.run(ctx, a)
	if err != nil {
		return nil, err
	}
	var md bytes.Buffer
	if err := output.WriteMarkdown(&md, *rep); err != nil {
		return nil, err
	}
	return textResult(md.String(), rep), nil
}

func (t *tools) score(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	a, err := parseArgs(raw, true)
	if err != nil {
		return nil, err
	}
	rep, err := t.run(ctx, a)
	if err != nil {
		return nil, err
	}
	s := rep.Score
	if s == nil {
		return nil, errors.New("no score was computed")
	}
	if s.Note != "" {
		return textResult("Not scored: "+s.Note, s), nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "AI readiness for %s: %d/100 (%s)\n", rep.URL, s.Value, s.Grade)
	for _, c := range s.Checks {
		if c.Applies && !c.Passed {
			fmt.Fprintf(&sb, "\n- %s (%.1f/%d): %s\n  Fix: %s\n", c.Title, c.Points, c.Weight, c.Detail, c.Recommendation)
		}
	}
	return textResult(sb.String(), s), nil
}

func (t *tools) robots(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	a, err := parseArgs(raw, true)
	if err != nil {
		return nil, err
	}
	c, err := acurl.New(t.base)
	if err != nil {
		return nil, err
	}
	cache := robots.NewCache(func(ctx context.Context, u string) (int, []byte, error) {
		return c.Get(ctx, u, robots.MaxSize)
	})
	audit := robots.Audit(ctx, cache, a.URL, c.Profiles())
	audit.LLMsTxt, _ = llmstxt.Check(ctx, func(ctx context.Context, u string) (int, []byte, error) {
		return c.Get(ctx, u, llmstxt.MaxSize)
	}, a.URL)
	var buf bytes.Buffer
	if err := output.WriteRobotsAudit(&buf, "table", audit, false); err != nil {
		return nil, err
	}
	return textResult(buf.String(), audit), nil
}

func (t *tools) list(_ context.Context, raw json.RawMessage) (*ToolResult, error) {
	a, err := parseArgs(raw, false)
	if err != nil {
		return nil, err
	}
	ps := t.base.Profiles
	if a.Category != "" {
		ps = profiles.FilterCategories(ps, []string{a.Category})
	}
	var sb strings.Builder
	for _, p := range ps {
		fmt.Fprintf(&sb, "- %s (%s", p.Name, p.Category)
		if p.Operator != "" {
			fmt.Fprintf(&sb, ", %s", p.Operator)
		}
		if p.RobotsOnly {
			sb.WriteString(", robots.txt token only")
		}
		sb.WriteString(")\n")
	}
	return textResult(sb.String(), map[string]any{"profiles": ps}), nil
}
