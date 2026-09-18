// Package output renders acurl reports in human and machine formats.
package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/olekukonko/tablewriter"

	"github.com/tragent-io/acurl/pkg/types"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	badStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4136"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
)

// Caveat explains what acurl results do and do not prove.
const Caveat = "acurl sends agent user-agent strings from your own IP address. Real AI agents " +
	"come from published IP ranges and may sign their requests, so these results show how a " +
	"site responds to user-agent strings, not guaranteed agent access."

// WriteTable renders result as a terminal table.
func WriteTable(w io.Writer, result types.ComparisonResult, color bool) error {
	style := func(s lipgloss.Style, text string) string {
		if !color {
			return text
		}
		return s.Render(text)
	}

	var sb strings.Builder
	sb.WriteString(style(headerStyle, "acurl results for "+result.URL) + "\n")
	sb.WriteString(style(mutedStyle, result.Timestamp.Format(time.RFC3339)) + "\n\n")

	table := tablewriter.NewTable(&sb)
	table.Header("Profile", "Status", "Time", "Size", "Result", "Details")
	for _, r := range result.Results {
		if err := table.Append(r.Profile, statusText(r), durationText(r.Duration), FormatSize(r.BodySize), style(classStyle(r.Classification), r.Classification), details(r)); err != nil {
			return err
		}
	}
	if err := table.Render(); err != nil {
		return err
	}

	sm := result.Summary
	sb.WriteString("\n" + style(headerStyle, "Summary") + "\n")
	line := func(label string, n int, s lipgloss.Style) {
		v := strconv.Itoa(n)
		if n > 0 {
			v = style(s, v)
		}
		fmt.Fprintf(&sb, "  %-18s %s\n", label, v)
	}
	line("Allowed", sm.Allowed, okStyle)
	line("Blocked", sm.Blocked, badStyle)
	line("Challenged", sm.Challenged, badStyle)
	line("Rate limited", sm.RateLimited, warnStyle)
	line("Cloaked", sm.Cloaked, warnStyle)
	line("Robots disallowed", sm.RobotsDisallow, warnStyle)
	line("Errors", sm.Errors, badStyle)

	if rr := result.Render; rr != nil {
		sb.WriteString("\n" + style(headerStyle, "JavaScript rendering") + "\n")
		switch {
		case rr.Error != "":
			sb.WriteString("  " + style(badStyle, rr.Error) + "\n")
		case rr.JSOnlyShare >= 0.2:
			fmt.Fprintf(&sb, "  %s of the visible text only appears after JavaScript runs (%d -> %d chars).\n  Agents that do not run JavaScript miss it.\n",
				style(badStyle, fmt.Sprintf("%.0f%%", rr.JSOnlyShare*100)), rr.RawTextLength, rr.RenderedTextLength)
		default:
			fmt.Fprintf(&sb, "  %s: the raw HTML already contains the page text (%d of %d chars).\n",
				style(okStyle, "OK"), rr.RawTextLength, rr.RenderedTextLength)
		}
	}

	sb.WriteString("\n" + style(mutedStyle, Caveat) + "\n")
	_, err := io.WriteString(w, sb.String())
	return err
}

func classStyle(c string) lipgloss.Style {
	switch c {
	case "allowed":
		return okStyle
	case "blocked", "challenged", "error":
		return badStyle
	default:
		return warnStyle
	}
}

func statusText(r types.FetchResult) string {
	if r.RobotsOnly {
		return "-"
	}
	if r.StatusCode == 0 {
		return "ERR"
	}
	return strconv.Itoa(r.StatusCode)
}

func durationText(d time.Duration) string {
	return fmt.Sprintf("%dms", d.Milliseconds())
}

// FormatSize renders a byte count as B, KB or MB.
func FormatSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
	}
}

func details(r types.FetchResult) string {
	var parts []string
	if r.RobotsOnly {
		parts = append(parts, "robots.txt token (not fetched)")
	}
	if r.BotChallenge != "" {
		parts = append(parts, "challenge: "+r.BotChallenge)
	}
	if r.Similarity != nil && *r.Similarity < 0.95 {
		parts = append(parts, fmt.Sprintf("%.0f%% similar to browser", *r.Similarity*100))
	}
	if r.JS != nil && r.JS.ClientRendered {
		parts = append(parts, "JS-only page"+frameworkSuffix(r.JS.Framework))
	}
	if r.ProductIssue != "" {
		parts = append(parts, "product: "+r.ProductIssue)
	}
	if r.RobotsDisallowed {
		parts = append(parts, "robots.txt: disallowed")
	}
	if d := blockingDirectives(r.Directives); d != "" {
		parts = append(parts, d)
	}
	if r.RateLimited {
		p := "rate limited"
		switch {
		case r.RetryAfterSeconds > 0:
			p += fmt.Sprintf(" (retry in %s)", time.Duration(r.RetryAfterSeconds)*time.Second)
		case r.RetryAfter != "":
			p += " (retry after " + r.RetryAfter + ")"
		}
		parts = append(parts, p)
	}
	if n := len(r.RedirectChain); n > 0 {
		parts = append(parts, fmt.Sprintf("%d redirect(s) -> %s", n, shortURL(r.FinalURL)))
	}
	if r.Error != "" {
		parts = append(parts, "error: "+truncate(r.Error, 50))
	}
	// The title is the least important detail; only show it when nothing
	// else needs saying.
	if len(parts) == 0 && r.Title != "" {
		parts = append(parts, truncate(r.Title, 40))
	}
	if len(parts) == 0 {
		return "OK"
	}
	return strings.Join(parts, "; ")
}

// shortURL drops the scheme and truncates for table display.
func shortURL(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	return truncate(u, 40)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// blockingDirectives keeps the directives that limit indexing or AI use.
func blockingDirectives(ds []string) string {
	var keep []string
	for _, d := range ds {
		switch d {
		case "noindex", "none", "noai", "noimageai", "nosnippet":
			keep = append(keep, d)
		}
	}
	return strings.Join(keep, ",")
}

func frameworkSuffix(f string) string {
	if f == "" {
		return ""
	}
	return " (" + f + ")"
}
