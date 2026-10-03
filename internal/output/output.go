package output

import (
	"fmt"
	"io"

	"github.com/tragent-io/acurl/pkg/types"
)

// Formats lists the supported output formats.
var Formats = []string{"table", "json", "markdown", "csv", "junit", "sarif", "html"}

// Version is reported in machine-readable formats. The CLI sets it.
var Version = "dev"

// Write renders result in format to w.
func Write(w io.Writer, format string, result types.ComparisonResult, color bool) error {
	switch format {
	case "table", "":
		return WriteTable(w, result, color)
	case "json":
		return WriteJSON(w, result)
	case "markdown", "md":
		return WriteMarkdown(w, result)
	case "csv":
		return WriteCSV(w, []*types.ComparisonResult{&result})
	case "junit":
		return WriteJUnit(w, []*types.ComparisonResult{&result})
	case "sarif":
		return WriteSARIF(w, Version, []*types.ComparisonResult{&result})
	case "html":
		return WriteHTML(w, []*types.ComparisonResult{&result})
	default:
		return fmt.Errorf("unknown format %q (supported: %v)", format, Formats)
	}
}
