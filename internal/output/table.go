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
	if r.BotChallenge != "" {
		parts = append(parts, "challenge: "+r.BotChallenge)
	}
	if r.Similarity != nil && *r.Similarity < 0.95 {
		parts = append(parts, fmt.Sprintf("%.0f%% similar to browser", *r.Similarity*100))
	}
	if r.RobotsDisallowed {
		parts = append(parts, "robots.txt: disallowed")
	}
	if r.RateLimited {
		p := "rate limited"
		if r.RetryAfter != "" {
			p += " (retry after " + r.RetryAfter + ")"
		}
		parts = append(parts, p)
	}
	if n := len(r.RedirectChain); n > 0 {
		parts = append(parts, fmt.Sprintf("%d redirect(s) -> %s", n, truncate(r.FinalURL, 40)))
	}
	if r.Error != "" {
		parts = append(parts, "error: "+truncate(r.Error, 60))
	}
	if r.Title != "" {
		parts = append(parts, "title: "+truncate(r.Title, 40))
	}
	if len(parts) == 0 {
		return "OK"
	}
	return strings.Join(parts, "; ")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
