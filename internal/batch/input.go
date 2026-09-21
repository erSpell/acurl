// Package batch runs acurl checks over many URLs.
package batch

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// ReadURLs reads one URL per line, skipping blank lines and # comments.
// When csvInput is true the input is CSV: the "url" column is used if the
// header has one, otherwise the first column.
func ReadURLs(r io.Reader, csvInput bool) ([]string, error) {
	if csvInput {
		return readCSV(r)
	}
	var urls []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	return urls, sc.Err()
}

func readCSV(r io.Reader) ([]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	col, start := 0, 0
	for i, h := range rows[0] {
		if strings.EqualFold(strings.TrimSpace(h), "url") {
			col, start = i, 1
			break
		}
	}
	var urls []string
	for _, row := range rows[start:] {
		if col < len(row) {
			if u := strings.TrimSpace(row[col]); u != "" && !strings.HasPrefix(u, "#") {
				urls = append(urls, u)
			}
		}
	}
	return urls, nil
}

// Dedupe removes repeated URLs, keeping the first occurrence.
func Dedupe(urls []string) []string {
	seen := make(map[string]bool, len(urls))
	out := urls[:0:0]
	for _, u := range urls {
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}
