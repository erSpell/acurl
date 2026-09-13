package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/llmstxt"
	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/internal/robots"
)

func newRobotsCmd(a *app) *cobra.Command {
	var (
		fl     httpFlags
		format string
	)
	cmd := &cobra.Command{
		Use:   "robots <url>",
		Short: "Audit which agents robots.txt allows for a URL",
		Long: `robots fetches robots.txt once and reports, for every agent profile with a
robots.txt token, whether it may fetch the URL, along with Content-Signal
directives, sitemaps and the site's llms.txt files. The page itself is not
fetched.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runRobots(cmd.Context(), &fl, format, args[0])
		},
	}
	fl.register(cmd.Flags())
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table or json")
	return cmd
}

func (a *app) runRobots(ctx context.Context, fl *httpFlags, format, rawURL string) error {
	url, err := normalizeURL(rawURL)
	if err != nil {
		return err
	}
	ps, err := fl.loadProfiles()
	if err != nil {
		return err
	}
	f, err := fl.newFetcher()
	if err != nil {
		return err
	}
	cache := robots.NewCache(func(ctx context.Context, u string) (int, []byte, error) {
		return f.Get(ctx, u, a.userAgent(), robots.MaxSize)
	})
	audit := robots.Audit(ctx, cache, url, ps)
	audit.LLMsTxt, err = llmstxt.Check(ctx, func(ctx context.Context, u string) (int, []byte, error) {
		return f.Get(ctx, u, a.userAgent(), llmstxt.MaxSize)
	}, url)
	if err != nil {
		return err
	}
	if err := output.WriteRobotsAudit(a.stdout, format, audit, !fl.noColor); err != nil {
		return err
	}
	if audit.Error != "" {
		return exitError(output.ExitError)
	}
	return nil
}
