package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/config"
)

const configTemplate = `# acurl configuration. Any command-line flag can be set here by its long
# name; flags and ACURL_* environment variables override these values.
# Docs: https://github.com/tragent-io/acurl

# Only run these agent categories (the browser baseline is always kept):
# category: [ai-assistant, ai-search, ai-training, search]

# Or pick profiles by name:
# agents: ["Chrome (Browser)", ChatGPT-User, Claude-User, GPTBot]

timeout: 30s
concurrency: 5
retries: 1

# Extra headers for every request:
# header:
#   - "Cookie: country=US"

# Output format for acurl and acurl batch: table, json or markdown.
# format: table

# Batch defaults.
# delay: 500ms
# limit: 50
`

func newInitCmd(a *app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a " + config.FileName + " config file in the current directory",
		Example: `  acurl init
  acurl init --force`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := os.Stat(config.FileName); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", config.FileName)
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := os.WriteFile(config.FileName, []byte(configTemplate), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "Wrote %s\n", config.FileName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	return cmd
}
