package robots

import (
	"net/http"
	"reflect"
	"testing"
)

func TestDirectives(t *testing.T) {
	body := []byte(`<html><head>
<meta name="robots" content="max-image-preview:large, noai">
<meta name="GPTBot" content="noindex, nofollow">
<meta name="description" content="noindex is not a directive here">
</head><body><meta name="robots" content="ignored-after-body"></body></html>`)
	h := http.Header{}
	h.Add("X-Robots-Tag", "noimageai")
	h.Add("X-Robots-Tag", "googlebot: nosnippet")
	h.Add("X-Robots-Tag", "unavailable_after: 25 Jun 2030 15:00:00 PST")

	tests := []struct {
		agent string
		want  []string
	}{
		{"GPTBot", []string{"noimageai", "unavailable_after: 25 jun 2030 15:00:00 pst", "max-image-preview:large", "noai", "noindex", "nofollow"}},
		{"Googlebot", []string{"noimageai", "nosnippet", "unavailable_after: 25 jun 2030 15:00:00 pst", "max-image-preview:large", "noai"}},
	}
	for _, tt := range tests {
		got := Directives(h, body, tt.agent)
		if !sameSet(got, tt.want) {
			t.Errorf("Directives(%s) = %v, want %v", tt.agent, got, tt.want)
		}
	}
	if got := Directives(http.Header{}, []byte("<p>plain</p>"), "GPTBot"); len(got) != 0 {
		t.Errorf("plain page directives = %v", got)
	}
}

func sameSet(a, b []string) bool {
	m := func(s []string) map[string]bool {
		out := map[string]bool{}
		for _, v := range s {
			out[v] = true
		}
		return out
	}
	return len(a) == len(b) && reflect.DeepEqual(m(a), m(b))
}
