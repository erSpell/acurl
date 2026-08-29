package output

import (
	"encoding/json"
	"io"

	"github.com/tragent-io/acurl/pkg/types"
)

// WriteJSON renders result as indented JSON.
func WriteJSON(w io.Writer, result types.ComparisonResult) error {
	result.Caveat = Caveat
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}
