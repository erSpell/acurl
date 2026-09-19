package output

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tragent-io/acurl/pkg/types"
)

func gradeStyle(g string) lipgloss.Style {
	switch g {
	case "A", "B":
		return okStyle
	case "C":
		return warnStyle
	default:
		return badStyle
	}
}

// writeScoreText renders the score for the terminal table.
func writeScoreText(sb *strings.Builder, s *types.Score, style func(lipgloss.Style, string) string) {
	if s == nil {
		return
	}
	fmt.Fprintf(sb, "\n%s %s\n", style(headerStyle, "AI readiness"),
		style(gradeStyle(s.Grade), fmt.Sprintf("%d/100 (%s)", s.Value, s.Grade)))
	for _, c := range s.Checks {
		if !c.Applies {
			continue
		}
		mark := style(okStyle, "✓")
		if !c.Passed {
			mark = style(badStyle, "✗")
		}
		fmt.Fprintf(sb, "  %s %-46s %4.1f/%d  %s\n", mark, c.Title, c.Points, c.Weight, style(mutedStyle, c.Detail))
	}
	var fixes []types.ScoreCheck
	for _, c := range s.Checks {
		if c.Applies && !c.Passed {
			fixes = append(fixes, c)
		}
	}
	if len(fixes) == 0 {
		return
	}
	sb.WriteString("\n" + style(headerStyle, "How to improve") + "\n")
	for i, c := range fixes {
		fmt.Fprintf(sb, "  %d. %s\n", i+1, style(lipgloss.NewStyle().Bold(true), c.Title))
		for _, line := range wrap(c.Recommendation, 88) {
			fmt.Fprintf(sb, "     %s\n", line)
		}
	}
}

// writeScoreMarkdown renders the score as a Markdown section.
func writeScoreMarkdown(sb *strings.Builder, s *types.Score) {
	if s == nil {
		return
	}
	fmt.Fprintf(sb, "\n## AI readiness: %d/100 (%s)\n\n", s.Value, s.Grade)
	sb.WriteString("| | Check | Points | Detail |\n|---|---|---|---|\n")
	for _, c := range s.Checks {
		if !c.Applies {
			continue
		}
		mark := "✅"
		if !c.Passed {
			mark = "❌"
		}
		fmt.Fprintf(sb, "| %s | %s | %.1f / %d | %s |\n", mark, c.Title, c.Points, c.Weight, mdEscape(c.Detail))
	}
	first := true
	for _, c := range s.Checks {
		if c.Applies && !c.Passed {
			if first {
				sb.WriteString("\n### How to improve\n\n")
				first = false
			}
			fmt.Fprintf(sb, "- **%s.** %s\n", c.Title, c.Recommendation)
		}
	}
}

// wrap splits s into lines of at most width runes on word boundaries.
func wrap(s string, width int) []string {
	var lines []string
	var cur strings.Builder
	for _, w := range strings.Fields(s) {
		if cur.Len() > 0 && cur.Len()+1+len(w) > width {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}
