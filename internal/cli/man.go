package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newManCmd generates man pages. It is hidden because it is used by the
// release build, not by people.
func newManCmd(a *app, root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "man <dir>",
		Short:  "Generate man pages into dir",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(args[0], 0o755); err != nil {
				return err
			}
			root.DisableAutoGenTag = true
			header := &doc.GenManHeader{Title: "ACURL", Section: "1", Source: "acurl " + a.info.Version, Manual: "acurl manual"}
			if err := doc.GenManTree(root, header, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "Man pages written to %s\n", args[0])
			return nil
		},
	}
}
