package output

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/tragent-io/acurl/pkg/types"
)

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name      string      `xml:"name,attr"`
	Tests     int         `xml:"tests,attr"`
	Failures  int         `xml:"failures,attr"`
	Errors    int         `xml:"errors,attr"`
	Skipped   int         `xml:"skipped,attr"`
	Timestamp string      `xml:"timestamp,attr,omitempty"`
	Cases     []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitMessage `xml:"failure,omitempty"`
	Error     *junitMessage `xml:"error,omitempty"`
	Skipped   *junitMessage `xml:"skipped,omitempty"`
}

type junitMessage struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr,omitempty"`
	Body    string `xml:",chardata"`
}

// WriteJUnit writes a JUnit XML report: one test suite per URL and one test
// case per profile. Blocked, challenged, rate-limited and cloaked results
// are failures, network errors are errors and robots.txt-disallowed results
// are skipped (blocking an agent there is a choice, not a bug).
func WriteJUnit(w io.Writer, reports []*types.ComparisonResult) error {
	out := junitSuites{Name: "acurl"}
	for _, rep := range reports {
		s := junitSuite{Name: rep.URL, Timestamp: rep.Timestamp.Format("2006-01-02T15:04:05Z07:00")}
		for _, r := range rep.Results {
			c := junitCase{
				Name:      r.Profile,
				Classname: "acurl." + categoryOr(r.Category),
				Time:      fmt.Sprintf("%.3f", float64(r.DurationMs)/1000),
			}
			msg := &junitMessage{Message: r.Classification + ": " + details(r), Type: r.Classification, Body: details(r)}
			switch r.Classification {
			case "allowed":
			case "robots-disallowed":
				c.Skipped = msg
				s.Skipped++
			case "error":
				c.Error = msg
				s.Errors++
			default:
				c.Failure = msg
				s.Failures++
			}
			s.Cases = append(s.Cases, c)
		}
		s.Tests = len(s.Cases)
		out.Tests += s.Tests
		out.Failures += s.Failures
		out.Errors += s.Errors
		out.Suites = append(out.Suites, s)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func categoryOr(c string) string {
	if c == "" {
		return "other"
	}
	return c
}
