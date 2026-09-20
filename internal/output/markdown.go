package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tragent-io/acurl/pkg/types"
)

// WriteMarkdown renders result as a GitHub-flavored Markdown report.
func WriteMarkdown(w io.Writer, result types.ComparisonResult) error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# acurl report\n\n")
	fmt.Fprintf(&sb, "- **URL:** %s\n- **Checked:** %s\n\n", result.URL, result.Timestamp.Format(time.RFC3339))

	sb.WriteString("| Profile | Status | Time | Size | Result | Details |\n")
	sb.WriteString("|---|---|---|---|---|---|\n")
	for _, r := range result.Results {
		fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s | %s |\n",
			mdEscape(r.Profile), statusText(r), durationText(r.Duration), FormatSize(r.BodySize),
			r.Classification, mdEscape(details(r)))
	}

	s := result.Summary
	sb.WriteString("\n## Summary\n\n")
	fmt.Fprintf(&sb, "| Allowed | Blocked | Challenged | Rate limited | Cloaked | Robots disallowed | Errors |\n")
	fmt.Fprintf(&sb, "|---|---|---|---|---|---|---|\n")
	fmt.Fprintf(&sb, "| %d | %d | %d | %d | %d | %d | %d |\n",
		s.Allowed, s.Blocked, s.Challenged, s.RateLimited, s.Cloaked, s.RobotsDisallow, s.Errors)

	writeScoreMarkdown(&sb, result.Score)

	fmt.Fprintf(&sb, "\n> %s\n", Caveat)
	_, err := io.WriteString(w, sb.String())
	return err
}

func mdEscape(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}
