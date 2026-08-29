package output

import (
	"fmt"
	"io"

	"github.com/tragent-io/acurl/pkg/types"
)

// Formats lists the supported output formats.
var Formats = []string{"table", "json", "markdown"}

// Write renders result in format to w.
func Write(w io.Writer, format string, result types.ComparisonResult, color bool) error {
	switch format {
	case "table", "":
		return WriteTable(w, result, color)
	case "json":
		return WriteJSON(w, result)
	case "markdown", "md":
		return WriteMarkdown(w, result)
	default:
		return fmt.Errorf("unknown format %q (supported: %v)", format, Formats)
	}
}
