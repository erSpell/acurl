// Command library shows how to use acurl as a Go library: check a list of
// pages with AI assistant profiles only and print the ones that fail.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tragent-io/acurl/pkg/acurl"
	"github.com/tragent-io/acurl/pkg/types"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./examples/library <url>...")
		os.Exit(2)
	}

	c, err := acurl.New(acurl.Options{
		Timeout:      20 * time.Second,
		BlockPrivate: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for _, u := range os.Args[1:] {
		rep, err := c.Check(ctx, u)
		if err != nil {
			log.Printf("%s: %v", u, err)
			continue
		}
		fmt.Printf("%s  score %d (%s)\n", rep.URL, rep.Score.Value, rep.Score.Grade)
		for _, r := range rep.Results {
			if r.Category == types.CategoryAIAssistant && r.Classification != "allowed" {
				fmt.Printf("  %-16s %s\n", r.Profile, r.Classification)
			}
		}
	}
}
