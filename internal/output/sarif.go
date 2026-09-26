package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/tragent-io/acurl/pkg/types"
)

// SARIF 2.1.0, the subset GitHub code scanning reads.
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Version        string      `json:"version,omitempty"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	ShortDescription sarifText    `json:"shortDescription"`
	Help             sarifText    `json:"help"`
	Properties       sarifProps   `json:"properties"`
	DefaultConfig    sarifDefault `json:"defaultConfiguration"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifProps struct {
	Tags []string `json:"tags"`
}

type sarifDefault struct {
	Level string `json:"level"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
	} `json:"physicalLocation"`
}

type ruleDef struct {
	name, desc, level string
}

var sarifRules = map[string]ruleDef{
	"blocked":      {"AgentBlocked", "An AI agent or crawler received an error status instead of the page.", "error"},
	"challenged":   {"AgentChallenged", "An AI agent or crawler received a bot challenge or CAPTCHA page.", "error"},
	"rate-limited": {"AgentRateLimited", "An AI agent or crawler was rate limited.", "warning"},
	"cloaked":      {"AgentCloaked", "An AI agent saw substantially different content than a browser.", "warning"},
	"error":        {"FetchError", "The request failed at the network level.", "note"},
	"product":      {"ProductMismatch", "An AI agent saw different product price or availability than a browser.", "warning"},
	"score":        {"ReadinessCheckFailed", "An AI-readiness check failed.", "note"},
}

// WriteSARIF writes findings as SARIF 2.1.0 for GitHub code scanning and
// other SARIF viewers. Each problem result and failed readiness check
// becomes one finding located at the page URL.
func WriteSARIF(w io.Writer, version string, reports []*types.ComparisonResult) error {
	run := sarifRun{Tool: sarifTool{Driver: sarifDriver{
		Name:           "acurl",
		InformationURI: "https://github.com/tragent-io/acurl",
		Version:        version,
	}}}
	used := map[string]bool{}
	add := func(rule, uri, msg string) {
		used[rule] = true
		res := sarifResult{RuleID: sarifRules[rule].name, Level: sarifRules[rule].level, Message: sarifText{msg}}
		var loc sarifLocation
		loc.PhysicalLocation.ArtifactLocation.URI = uri
		res.Locations = []sarifLocation{loc}
		run.Results = append(run.Results, res)
	}
	for _, rep := range reports {
		for _, r := range rep.Results {
			if _, ok := sarifRules[r.Classification]; ok {
				add(r.Classification, rep.URL, fmt.Sprintf("%s (%s): %s", r.Profile, r.Classification, details(r)))
			}
			if r.ProductIssue != "" {
				add("product", rep.URL, r.Profile+": "+r.ProductIssue)
			}
		}
		if rep.Score != nil {
			for _, c := range rep.Score.Checks {
				if c.Applies && !c.Passed {
					add("score", rep.URL, fmt.Sprintf("%s: %s. %s", c.Title, c.Detail, c.Recommendation))
				}
			}
		}
	}
	for _, id := range []string{"blocked", "challenged", "rate-limited", "cloaked", "product", "error", "score"} {
		if !used[id] {
			continue
		}
		d := sarifRules[id]
		run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarifRule{
			ID: d.name, Name: d.name, ShortDescription: sarifText{d.desc},
			Help:          sarifText{d.desc + " See https://github.com/tragent-io/acurl/blob/main/docs/scoring.md"},
			Properties:    sarifProps{Tags: []string{"ai-agents", "accessibility"}},
			DefaultConfig: sarifDefault{Level: d.level},
		})
	}
	if run.Results == nil {
		run.Results = []sarifResult{}
	}
	if run.Tool.Driver.Rules == nil {
		run.Tool.Driver.Rules = []sarifRule{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs:    []sarifRun{run},
	})
}
