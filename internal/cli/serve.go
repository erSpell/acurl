package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/server"
)

func newServeCmd(a *app) *cobra.Command {
	var (
		fl           httpFlags
		addr         string
		allowPrivate bool
		render       bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the acurl web UI and JSON API locally",
		Long: `serve starts a local web UI for running checks in the browser, plus the JSON
API it uses (POST /api/check, GET /api/robots, GET /api/profiles).

Requests to private and internal addresses are blocked unless --allow-private is
set, because anyone who can reach the server can make it fetch URLs.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := fl.options(a)
			if err != nil {
				return err
			}
			opts.BlockPrivate = !allowPrivate
			opts.Render = render
			srv, err := server.New(server.Options{Check: opts, Version: a.info.Version})
			if err != nil {
				return err
			}
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}
			hs := &http.Server{Handler: srv, ReadHeaderTimeout: 10 * time.Second}
			fmt.Fprintf(a.stderr, "acurl web UI on http://%s (Ctrl+C to stop)\n", ln.Addr())
			go func() {
				<-cmd.Context().Done()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = hs.Shutdown(ctx)
			}()
			if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	fl.register(cmd.Flags())
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "address to listen on")
	cmd.Flags().BoolVar(&allowPrivate, "allow-private", false, "allow checking private and internal addresses")
	cmd.Flags().BoolVar(&render, "render", false, "allow headless Chrome rendering from the UI")
	return cmd
}
