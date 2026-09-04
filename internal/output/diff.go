package output

import (
	"fmt"
	"io"

	"github.com/tragent-io/acurl/internal/textdiff"
)

// WriteDiff prints a line diff of the visible text two profiles received.
func WriteDiff(w io.Writer, baseName, otherName string, lines []textdiff.Line, color bool) error {
	style := func(s string, st func(...string) string) string {
		if color {
			return st(s)
		}
		return s
	}
	if _, err := fmt.Fprintf(w, "%s\n%s\n", style("--- "+baseName, badStyle.Render), style("+++ "+otherName, okStyle.Render)); err != nil {
		return err
	}
	if !textdiff.Changed(lines) {
		_, err := fmt.Fprintln(w, "(visible text is identical)")
		return err
	}
	for _, l := range lines {
		var s string
		switch l.Op {
		case textdiff.Insert:
			s = style("+ "+l.Text, okStyle.Render)
		case textdiff.Delete:
			s = style("- "+l.Text, badStyle.Render)
		case textdiff.Skip:
			s = style("  ...", mutedStyle.Render)
		default:
			s = "  " + l.Text
		}
		if _, err := fmt.Fprintln(w, s); err != nil {
			return err
		}
	}
	return nil
}
