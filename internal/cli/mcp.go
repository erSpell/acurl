package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/mcp"
)

func newMCPCmd(a *app) *cobra.Command {
	var (
		fl           httpFlags
		allowPrivate bool
	)
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run acurl as an MCP server for AI assistants (stdio)",
		Long: `mcp runs a Model Context Protocol server on stdin/stdout so assistants such as
Claude Desktop, Claude Code and Cursor can call acurl's checks as tools:
check_url, readiness_score, robots_audit and list_profiles.

Add it to Claude Code with:
  claude mcp add acurl -- acurl mcp`,
		Example: `  claude mcp add acurl -- acurl mcp
  acurl mcp --timeout 20s`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.quiet = true // stdout belongs to the protocol
			opts, err := fl.options(a)
			if err != nil {
				return err
			}
			opts.BlockPrivate = !allowPrivate
			srv, err := mcp.New(opts, a.info.Version)
			if err != nil {
				return err
			}
			return srv.Serve(cmd.Context(), os.Stdin, a.stdout)
		},
	}
	fl.register(cmd.Flags())
	cmd.Flags().BoolVar(&allowPrivate, "allow-private", false, "allow checking private and internal addresses")
	return cmd
}
