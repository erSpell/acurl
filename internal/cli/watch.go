package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/watch"
	"github.com/tragent-io/acurl/pkg/acurl"
	"github.com/tragent-io/acurl/pkg/types"
)

type watchFlags struct {
	check         checkFlags
	interval      time.Duration
	state         string
	webhook       string
	webhookFormat string
	minScoreDelta int
}

func newWatchCmd(a *app) *cobra.Command {
	var fl watchFlags
	cmd := &cobra.Command{
		Use:   "watch <url>...",
		Short: "Re-check URLs and report when agent access changes",
		Long: `watch checks URLs repeatedly and prints every change in how an agent is
treated, or a score change of --min-score-delta points or more.

With --interval 0 it runs once and compares with the --state file, which suits cron.`,
		Example: `  acurl watch https://shop.example --interval 1h --webhook https://hooks.slack.com/... --webhook-format slack
  acurl watch https://shop.example --interval 0 --state acurl-state.json   # from cron`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runWatch(cmd.Context(), &fl, args)
		},
	}
	f := cmd.Flags()
	fl.check.http.register(f)
	f.BoolVar(&fl.check.noRobots, "no-robots", false, "skip robots.txt checks")
	f.DurationVar(&fl.interval, "interval", time.Hour, "time between checks (0 = check once)")
	f.StringVar(&fl.state, "state", "", "file that keeps the last results between runs")
	f.StringVar(&fl.webhook, "webhook", "", "POST changes to this URL")
	f.StringVar(&fl.webhookFormat, "webhook-format", "json", `webhook body: "json" or "slack"`)
	f.IntVar(&fl.minScoreDelta, "min-score-delta", 5, "report score changes of at least this many points")
	return cmd
}

func (a *app) runWatch(ctx context.Context, fl *watchFlags, args []string) error {
	urls := make([]string, 0, len(args))
	for _, raw := range args {
		u, err := normalizeURL(raw)
		if err != nil {
			return err
		}
		urls = append(urls, u)
	}
	if fl.interval > 0 && fl.interval < time.Minute {
		return fmt.Errorf("--interval must be at least 1m (or 0 to run once)")
	}
	opts, err := a.checkOptions(&fl.check)
	if err != nil {
		return err
	}
	c, err := acurl.New(opts)
	if err != nil {
		return err
	}
	prev, err := loadState(fl.state)
	if err != nil {
		return err
	}
	notifier := &watch.Notifier{URL: fl.webhook, Format: fl.webhookFormat}

	for {
		var changes []watch.Change
		for _, u := range urls {
			rep, err := c.Check(ctx, u)
			if err != nil {
				return err
			}
			if old, ok := prev[u]; ok {
				changes = append(changes, watch.Diff(old, rep, fl.minScoreDelta)...)
			} else {
				a.logf("%s %s: baseline recorded (score %s)\n", time.Now().Format(time.TimeOnly), u, scoreText(rep.Score))
			}
			prev[u] = rep
		}
		for _, ch := range changes {
			fmt.Fprintf(a.stdout, "%s %s\n", time.Now().Format(time.DateTime), ch)
		}
		if len(changes) == 0 {
			a.logf("%s no changes\n", time.Now().Format(time.TimeOnly))
		}
		if err := notifier.Notify(ctx, changes); err != nil {
			fmt.Fprintln(a.stderr, "acurl:", err)
		}
		if err := saveState(fl.state, prev); err != nil {
			return err
		}
		if fl.interval == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(fl.interval):
		}
	}
}

func scoreText(s *types.Score) string {
	if s == nil || s.Note != "" {
		return "n/a"
	}
	return fmt.Sprintf("%d/100", s.Value)
}

// state maps URL to its last report. Bodies are not stored.
func loadState(path string) (map[string]*types.ComparisonResult, error) {
	st := map[string]*types.ComparisonResult{}
	if path == "" {
		return st, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("state file %s: %w", path, err)
	}
	return st, nil
}

func saveState(path string, st map[string]*types.ComparisonResult) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
