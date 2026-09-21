package batch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tragent-io/acurl/pkg/types"
)

func TestReadURLs(t *testing.T) {
	got, err := ReadURLs(strings.NewReader("# products\nhttps://a.example/1\n\n  https://a.example/2  \n# end\n"), false)
	if err != nil || !reflect.DeepEqual(got, []string{"https://a.example/1", "https://a.example/2"}) {
		t.Errorf("ReadURLs = %v, %v", got, err)
	}
}

func TestReadURLsCSV(t *testing.T) {
	withHeader := "sku,url,title\n1,https://a.example/1,One\n2,https://a.example/2,\"Two, with comma\"\n"
	got, err := ReadURLs(strings.NewReader(withHeader), true)
	if err != nil || !reflect.DeepEqual(got, []string{"https://a.example/1", "https://a.example/2"}) {
		t.Errorf("CSV with header = %v, %v", got, err)
	}
	noHeader := "https://a.example/1,x\nhttps://a.example/2,y\n"
	got, err = ReadURLs(strings.NewReader(noHeader), true)
	if err != nil || len(got) != 2 || got[0] != "https://a.example/1" {
		t.Errorf("CSV without header = %v, %v", got, err)
	}
}

func TestDedupe(t *testing.T) {
	if got := Dedupe([]string{"a", "b", "a", "c", "b"}); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("Dedupe = %v", got)
	}
}

type fakeChecker map[string]int // url -> score; missing url fails

func (f fakeChecker) Check(_ context.Context, u string) (*types.ComparisonResult, error) {
	v, ok := f[u]
	if !ok {
		return nil, errors.New("boom")
	}
	return &types.ComparisonResult{
		URL:     u,
		Summary: types.Summary{Total: 2, Allowed: 1, Blocked: 1},
		Score:   &types.Score{Value: v},
	}, nil
}

func TestRun(t *testing.T) {
	urls := []string{"u1", "u2", "bad", "u3"}
	var progress int
	rep := Run(context.Background(), fakeChecker{"u1": 90, "u2": 60, "u3": 30}, urls, Options{Parallel: 3, Progress: func(done, total int, _ string) {
		progress = done
		if total != 4 {
			t.Errorf("total = %d", total)
		}
	}})
	for i, it := range rep.Items {
		if it.URL != urls[i] {
			t.Errorf("item %d url %q, want %q", i, it.URL, urls[i])
		}
	}
	s := rep.Summary
	if s.URLs != 4 || s.Failed != 1 || s.Scored != 3 || s.AverageScore != 60 || s.Results.Blocked != 3 || progress != 4 {
		t.Errorf("summary = %+v progress=%d", s, progress)
	}
	if rep.Items[2].Error != "boom" {
		t.Errorf("failed item = %+v", rep.Items[2])
	}
}
