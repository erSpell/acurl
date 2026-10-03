package output

import (
	"embed"
	"fmt"
	"html/template"
	"io"

	"github.com/tragent-io/acurl/internal/classifier"
	"github.com/tragent-io/acurl/internal/textdiff"
	"github.com/tragent-io/acurl/pkg/types"
)

//go:embed templates/report.html.tmpl
var templateFS embed.FS

var htmlTemplate = template.Must(template.New("report.html.tmpl").Funcs(template.FuncMap{
	"status":   statusText,
	"timeText": timeText,
	"sizeText": sizeText,
	"details":  details,
	"percent":  func(f float64) string { return fmt.Sprintf("%.0f%%", f*100) },
	"failed": func(cs []types.ScoreCheck) []types.ScoreCheck {
		var out []types.ScoreCheck
		for _, c := range cs {
			if c.Applies && !c.Passed {
				out = append(out, c)
			}
		}
		return out
	},
}).ParseFS(templateFS, "templates/report.html.tmpl"))

type htmlDiff struct {
	Profile, Baseline string
	Similarity        float64
	Lines             []textdiff.Line
}

type htmlPage struct {
	Title   string
	Reports []*types.ComparisonResult
	Diffs   [][]htmlDiff
	Caveat  string
	Version string
}

// WriteHTML renders a self-contained HTML report (no external assets). When
// response bodies are available, it includes visible-text diffs for every
// agent whose content differs noticeably from the browser.
func WriteHTML(w io.Writer, reports []*types.ComparisonResult) error {
	page := htmlPage{Reports: reports, Caveat: Caveat, Version: Version}
	switch len(reports) {
	case 0:
		page.Title = "no results"
	case 1:
		page.Title = reports[0].URL
	default:
		page.Title = fmt.Sprintf("%d URLs", len(reports))
	}
	for _, rep := range reports {
		page.Diffs = append(page.Diffs, diffsFor(rep))
	}
	return htmlTemplate.Execute(w, page)
}

// diffsFor returns diffs for results that differ from the baseline.
func diffsFor(rep *types.ComparisonResult) []htmlDiff {
	b := classifier.BaselineIndex(rep.Results)
	if b < 0 || len(rep.Results[b].Body) == 0 {
		return nil
	}
	base := rep.Results[b]
	baseText := textdiff.VisibleText(base.Body)
	var out []htmlDiff
	for i, r := range rep.Results {
		if i == b || r.Similarity == nil || *r.Similarity >= 0.95 || len(r.Body) == 0 {
			continue
		}
		out = append(out, htmlDiff{
			Profile:    r.Profile,
			Baseline:   base.Profile,
			Similarity: *r.Similarity,
			Lines:      textdiff.Lines(baseText, textdiff.VisibleText(r.Body), 2),
		})
	}
	return out
}
