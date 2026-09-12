package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"

	"github.com/tragent-io/acurl/pkg/types"
)

// WriteRobotsAudit renders a robots.txt audit as a table or JSON.
func WriteRobotsAudit(w io.Writer, format string, a types.RobotsAudit, color bool) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(a)
	case "table", "":
	default:
		return fmt.Errorf("unknown format %q (supported: table, json)", format)
	}
	style := func(s string, st func(...string) string) string {
		if color {
			return st(s)
		}
		return s
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", style("robots.txt audit for "+a.URL, headerStyle.Render))
	switch {
	case a.Error != "":
		fmt.Fprintf(&sb, "%s: %s (RFC 9309: crawlers must treat this as full disallow)\n\n", a.RobotsURL, style(a.Error, badStyle.Render))
	case a.Status >= 400:
		fmt.Fprintf(&sb, "%s returned %d, so every agent is allowed\n\n", a.RobotsURL, a.Status)
	default:
		fmt.Fprintf(&sb, "%s (%d)\n\n", a.RobotsURL, a.Status)
	}

	t := tablewriter.NewTable(&sb)
	t.Header("Agent", "Token", "Category", "Allowed", "Content signals")
	for _, ag := range a.Agents {
		allowed := style("yes", okStyle.Render)
		if !ag.Allowed {
			allowed = style("no", badStyle.Render)
		}
		if err := t.Append(ag.Profile, ag.Token, ag.Category, allowed, formatSignals(ag.Signals)); err != nil {
			return err
		}
	}
	if err := t.Render(); err != nil {
		return err
	}
	if len(a.Sitemaps) > 0 {
		sb.WriteString("\nSitemaps:\n")
		for _, s := range a.Sitemaps {
			fmt.Fprintf(&sb, "  %s\n", s)
		}
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

// formatSignals renders signals in a stable order: the three standard
// signals first, then any others alphabetically.
func formatSignals(sig map[string]string) string {
	if len(sig) == 0 {
		return "-"
	}
	var parts []string
	seen := map[string]bool{}
	for _, k := range []string{"search", "ai-input", "ai-train"} {
		if v, ok := sig[k]; ok {
			parts = append(parts, k+"="+v)
			seen[k] = true
		}
	}
	var rest []string
	for k, v := range sig {
		if !seen[k] {
			rest = append(rest, k+"="+v)
		}
	}
	sort.Strings(rest)
	return strings.Join(append(parts, rest...), " ")
}
