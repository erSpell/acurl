package textdiff

import "testing"

func FuzzVisibleText(f *testing.F) {
	f.Add("<html><body><p>hi</p></body></html>")
	f.Add("<script><script></script></script>text")
	f.Add("</svg></svg><title>t")
	f.Fuzz(func(t *testing.T, doc string) {
		text := VisibleText([]byte(doc))
		if s := Similarity(text, text); s != 1 {
			t.Errorf("Similarity(x, x) = %v", s)
		}
	})
}
