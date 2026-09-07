// Command acurl shows how AI agents see a URL compared to a real browser.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/tragent-io/acurl/internal/cli"
)

// Set by goreleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Execute(ctx, cli.BuildInfo{Version: version, Commit: commit, Date: date}, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
