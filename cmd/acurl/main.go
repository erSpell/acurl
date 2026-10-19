// Command acurl shows how AI agents see a URL compared to a real browser.
package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"

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
	code := cli.Execute(ctx, buildInfo(), os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// buildInfo prefers the release ldflags and otherwise reads what the Go
// toolchain embedded, so `go install ...@v1.1.0` reports v1.1.0 too.
func buildInfo() cli.BuildInfo {
	info := cli.BuildInfo{Version: version, Commit: commit, Date: date}
	if version != "dev" {
		return info
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		info.Version = strings.TrimPrefix(v, "v")
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				info.Commit = s.Value[:7]
			}
		case "vcs.time":
			info.Date = s.Value
		}
	}
	return info
}
