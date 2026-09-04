package textdiff

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Op is the kind of a diff line.
type Op int

// Diff line kinds.
const (
	Equal Op = iota
	Insert
	Delete
	Skip // marks elided unchanged lines
)

// Line is one line of a line-based diff.
type Line struct {
	Op   Op
	Text string
}

// Lines diffs a and b line by line and keeps context unchanged lines around
// each change. Longer unchanged runs collapse into a single Skip line.
func Lines(a, b string, context int) []Line {
	dmp := diffmatchpatch.New()
	ca, cb, lines := dmp.DiffLinesToChars(a, b)
	diffs := dmp.DiffCharsToLines(dmp.DiffMain(ca, cb, false), lines)

	var all []Line
	for _, d := range diffs {
		op := Equal
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			op = Insert
		case diffmatchpatch.DiffDelete:
			op = Delete
		}
		for _, l := range strings.Split(strings.TrimSuffix(d.Text, "\n"), "\n") {
			all = append(all, Line{Op: op, Text: l})
		}
	}
	return withContext(all, context)
}

func withContext(all []Line, context int) []Line {
	keep := make([]bool, len(all))
	for i, l := range all {
		if l.Op == Equal {
			continue
		}
		for j := max(0, i-context); j <= min(len(all)-1, i+context); j++ {
			keep[j] = true
		}
	}
	var out []Line
	skipped := false
	for i, l := range all {
		if keep[i] {
			out = append(out, l)
			skipped = false
		} else if !skipped {
			out = append(out, Line{Op: Skip})
			skipped = true
		}
	}
	return out
}

// Changed reports whether lines contain any insertion or deletion.
func Changed(lines []Line) bool {
	for _, l := range lines {
		if l.Op == Insert || l.Op == Delete {
			return true
		}
	}
	return false
}
