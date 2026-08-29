package output

import "github.com/tragent-io/acurl/pkg/types"

// Exit codes returned by acurl.
const (
	ExitOK      = 0 // every profile was allowed
	ExitBlocked = 1 // at least one profile was blocked or challenged
	ExitError   = 2 // a request failed, or acurl itself failed
)

// ExitCode maps a summary to a process exit code.
func ExitCode(s types.Summary) int {
	switch {
	case s.Errors > 0:
		return ExitError
	case s.Blocked > 0 || s.Challenged > 0:
		return ExitBlocked
	default:
		return ExitOK
	}
}
