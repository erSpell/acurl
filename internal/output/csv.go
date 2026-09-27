package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tragent-io/acurl/pkg/types"
)

var csvHeader = []string{
	"url", "profile", "category", "status", "classification", "duration_ms", "body_size",
	"bot_challenge", "robots_disallowed", "similarity", "directives", "product_issue", "error", "score",
}

// WriteCSV writes one row per URL and profile, for spreadsheets.
func WriteCSV(w io.Writer, reports []*types.ComparisonResult) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, rep := range reports {
		score := ""
		if rep.Score != nil {
			score = strconv.Itoa(rep.Score.Value)
		}
		for _, r := range rep.Results {
			sim := ""
			if r.Similarity != nil {
				sim = fmt.Sprintf("%.3f", *r.Similarity)
			}
			status := strconv.Itoa(r.StatusCode)
			if r.RobotsOnly {
				status = ""
			}
			row := []string{
				rep.URL, r.Profile, r.Category, status, r.Classification,
				strconv.FormatInt(r.DurationMs, 10), strconv.Itoa(r.BodySize), r.BotChallenge,
				strconv.FormatBool(r.RobotsDisallowed), sim, strings.Join(r.Directives, " "),
				r.ProductIssue, r.Error, score,
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
