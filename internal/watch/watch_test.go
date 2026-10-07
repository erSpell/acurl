package watch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tragent-io/acurl/pkg/types"
)

func report(score int, classes map[string]string) *types.ComparisonResult {
	r := &types.ComparisonResult{URL: "https://shop.example", Score: &types.Score{Value: score, Grade: "B"}}
	for _, p := range []string{"Chrome", "ChatGPT-User", "GPTBot"} {
		if c, ok := classes[p]; ok {
			r.Results = append(r.Results, types.FetchResult{Profile: p, Classification: c})
		}
	}
	return r
}

func TestDiff(t *testing.T) {
	prev := report(90, map[string]string{"Chrome": "allowed", "ChatGPT-User": "allowed", "GPTBot": "blocked"})
	cur := report(70, map[string]string{"Chrome": "allowed", "ChatGPT-User": "challenged", "GPTBot": "allowed"})
	got := Diff(prev, cur, 5)
	if len(got) != 3 {
		t.Fatalf("Diff = %+v", got)
	}
	if got[0].Profile != "ChatGPT-User" || !got[0].Worse || got[1].Profile != "GPTBot" || got[1].Worse {
		t.Errorf("profile changes = %+v", got[:2])
	}
	if got[2].Profile != "" || !got[2].Worse || got[2].From != "90 (B)" {
		t.Errorf("score change = %+v", got[2])
	}
	if d := Diff(prev, report(88, map[string]string{"Chrome": "allowed", "ChatGPT-User": "allowed", "GPTBot": "blocked"}), 5); len(d) != 0 {
		t.Errorf("small score change should be ignored: %+v", d)
	}
}

func TestNotify(t *testing.T) {
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()
	changes := []Change{{URL: "https://shop.example", Profile: "ChatGPT-User", From: "allowed", To: "challenged", Worse: true}}

	if err := (&Notifier{URL: srv.URL}).Notify(context.Background(), changes); err != nil {
		t.Fatal(err)
	}
	var p Payload
	if err := json.Unmarshal(got, &p); err != nil || p.Event != "acurl.changes" || len(p.Changes) != 1 {
		t.Errorf("json payload = %s", got)
	}

	if err := (&Notifier{URL: srv.URL, Format: "slack"}).Notify(context.Background(), changes); err != nil {
		t.Fatal(err)
	}
	var slack struct{ Text string }
	if err := json.Unmarshal(got, &slack); err != nil || !strings.Contains(slack.Text, ":warning: https://shop.example: ChatGPT-User allowed -> challenged") {
		t.Errorf("slack payload = %s", got)
	}

	got = nil
	if err := (&Notifier{URL: srv.URL}).Notify(context.Background(), nil); err != nil || got != nil {
		t.Error("no changes should send nothing")
	}
}

func TestNotifyErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer srv.Close()
	ch := []Change{{URL: "u"}}
	if err := (&Notifier{URL: srv.URL}).Notify(context.Background(), ch); err == nil {
		t.Error("expected error on 500")
	}
	if err := (&Notifier{URL: srv.URL, Format: "teams"}).Notify(context.Background(), ch); err == nil {
		t.Error("expected error for unknown format")
	}
}
