// Package cli implements the acurl command line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/output"
)

// BuildInfo describes the running binary.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// app holds what every command needs.
type app struct {
	info   BuildInfo
	stdout io.Writer
	stderr io.Writer
}

// userAgent identifies acurl itself when fetching auxiliary files.
func (a *app) userAgent() string {
	return "acurl/" + a.info.Version + " (+https://github.com/tragent-io/acurl)"
}

// Execute runs acurl with args and returns the process exit code.
func Execute(ctx context.Context, info BuildInfo, args []string, stdout, stderr io.Writer) int {
	a := &app{info: info, stdout: stdout, stderr: stderr}
	root := newRootCmd(a)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.ExecuteContext(ctx)
	var code exitError
	switch {
	case err == nil:
		return output.ExitOK
	case errors.As(err, &code):
		return int(code)
	default:
		fmt.Fprintln(stderr, "acurl:", err)
		return output.ExitError
	}
}

func newRootCmd(a *app) *cobra.Command {
	cmd := newCheckCmd(a)
	cmd.Version = fmt.Sprintf("%s (commit %s, built %s)", a.info.Version, a.info.Commit, a.info.Date)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

// exitError carries a non-zero exit code without an error message.
type exitError int

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
