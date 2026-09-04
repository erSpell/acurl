package textdiff

import (
	"strings"
	"testing"
)

func TestVisibleText(t *testing.T) {
	doc := `<!doctype html><html><head><title>Red Shoes</title>
<style>body{color:red}</style><script>var nonce="abc123";</script></head>
<body><nav>Home</nav><main><h1>Red  Shoes</h1><p>Only <b>$49</b> today.</p>
<noscript>Enable JS</noscript><svg><text>icon</text></svg><ul><li>Size 9</li><li>Size 10</li></ul></main></body></html>`
	want := "Red Shoes\nHome\nRed Shoes\nOnly $49 today.\nSize 9\nSize 10"
	if got := VisibleText([]byte(doc)); got != want {
		t.Errorf("VisibleText:\n%q\nwant\n%q", got, want)
	}
}

func TestVisibleTextIgnoresDynamicMarkup(t *testing.T) {
	a := `<html><body data-session="1"><script>window.t=1</script><p>Same text here for all agents</p></body></html>`
	b := `<html><body data-session="2"><script>window.t=2</script><p>Same text   here for all agents</p></body></html>`
	if VisibleText([]byte(a)) != VisibleText([]byte(b)) {
		t.Error("pages that differ only in markup and scripts should have equal visible text")
	}
}

func TestSimilarity(t *testing.T) {
	long := strings.Repeat("the quick brown fox jumps over the lazy dog ", 20)
	tests := []struct {
		name     string
		a, b     string
		min, max float64
	}{
		{"identical", long, long, 1, 1},
		{"both empty", "", "", 1, 1},
		{"one empty", long, "", 0, 0},
		{"disjoint", "alpha beta gamma delta", "one two three four", 0, 0},
		{"small edit", long + "price 49", long + "price 59", 0.7, 0.99},
		{"case insensitive", "Hello Big World", "hello big world", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Similarity(tt.a, tt.b)
			if got < tt.min || got > tt.max {
				t.Errorf("Similarity = %.3f, want [%.2f, %.2f]", got, tt.min, tt.max)
			}
		})
	}
}

func TestLines(t *testing.T) {
	a := "title\nline 1\nline 2\nline 3\nline 4\nprice $49\nfooter"
	b := "title\nline 1\nline 2\nline 3\nline 4\nprice $59\nfooter"
	got := Lines(a, b, 1)
	want := []Line{{Op: Skip}, {Equal, "line 4"}, {Delete, "price $49"}, {Insert, "price $59"}, {Equal, "footer"}}
	if len(got) != len(want) {
		t.Fatalf("Lines = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if !Changed(got) || Changed(Lines(a, a, 1)) {
		t.Error("Changed() misreports")
	}
}
