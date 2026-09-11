package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/tragent-io/acurl/internal/profiles"
	"github.com/tragent-io/acurl/pkg/types"
)

func newProfilesCmd(a *app) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "List and inspect agent profiles",
	}
	cmd.PersistentFlags().StringVar(&file, "profiles", "", "custom profiles YAML file")

	var categories []string
	var format string
	list := &cobra.Command{
		Use:   "list",
		Short: "List the available agent profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ps, err := profiles.Load(file)
			if err != nil {
				return err
			}
			ps = profiles.FilterCategories(ps, categories)
			if format == "json" {
				enc := json.NewEncoder(a.stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(ps)
			}
			return writeProfileTable(a, ps)
		},
	}
	list.Flags().StringSliceVar(&categories, "category", nil, "only list these categories")
	list.Flags().StringVarP(&format, "format", "f", "table", "output format: table or json")

	show := &cobra.Command{
		Use:   "show <name>",
		Short: "Show one profile as YAML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ps, err := profiles.Load(file)
			if err != nil {
				return err
			}
			found := profiles.Filter(ps, args)
			if len(found) == 0 {
				return errNoProfiles(args)
			}
			out, err := yaml.Marshal(found[0])
			if err != nil {
				return err
			}
			_, err = a.stdout.Write(out)
			return err
		},
	}

	cmd.AddCommand(list, show)
	return cmd
}

func writeProfileTable(a *app, ps []types.Profile) error {
	t := tablewriter.NewTable(a.stdout)
	t.Header("Name", "Category", "Operator", "robots.txt token", "Notes")
	for _, p := range ps {
		var notes []string
		if p.Browser {
			notes = append(notes, "baseline")
		}
		if p.RobotsOnly {
			notes = append(notes, "robots.txt only")
		}
		key := p.RobotsKey
		if key == "" {
			key = "-"
		}
		if err := t.Append(p.Name, p.Category, p.Operator, key, strings.Join(notes, ", ")); err != nil {
			return err
		}
	}
	if err := t.Render(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(a.stdout, "%d profiles\n", len(ps))
	return err
}
