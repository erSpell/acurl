// Package server implements acurl serve: a local web UI and JSON API.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tragent-io/acurl/internal/llmstxt"
	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/internal/profiles"
	"github.com/tragent-io/acurl/internal/robots"
	"github.com/tragent-io/acurl/pkg/acurl"
	"github.com/tragent-io/acurl/pkg/types"
)

//go:embed static
var static embed.FS

// Options configures the server.
type Options struct {
	// Base options for every check. Profiles are filtered per request.
	Check acurl.Options
	// MaxConcurrent limits simultaneous checks. Default 4.
	MaxConcurrent int
	Version       string
}

// Server serves the UI and API.
type Server struct {
	opts Options
	sem  chan struct{}
	mux  *http.ServeMux
}

// New returns a Server. Callers should set Check.BlockPrivate unless the
// server is only reachable by trusted users.
func New(opts Options) (*Server, error) {
	if opts.Check.Profiles == nil {
		ps, err := profiles.Default()
		if err != nil {
			return nil, err
		}
		opts.Check.Profiles = ps
	}
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = 4
	}
	s := &Server{opts: opts, sem: make(chan struct{}, opts.MaxConcurrent), mux: http.NewServeMux()}
	ui, err := fs.Sub(static, "static")
	if err != nil {
		return nil, err
	}
	s.mux.Handle("GET /", http.FileServerFS(ui))
	s.mux.HandleFunc("GET /api/profiles", s.handleProfiles)
	s.mux.HandleFunc("POST /api/check", s.handleCheck)
	s.mux.HandleFunc("GET /api/robots", s.handleRobots)
	s.mux.HandleFunc("POST /api/report", s.handleReport)
	s.mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": opts.Version})
	})
	return s, nil
}

// ServeHTTP implements http.Handler with basic security headers.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' data:")
	s.mux.ServeHTTP(w, r)
}

// CheckRequest is the body of POST /api/check.
type CheckRequest struct {
	URL        string   `json:"url"`
	Categories []string `json:"categories,omitempty"`
	Agents     []string `json:"agents,omitempty"`
	Render     bool     `json:"render,omitempty"`
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleProfiles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.opts.Check.Profiles)
}

// checkURL validates a user-supplied URL.
func checkURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("enter a URL to check")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%q is not an http or https URL", raw)
	}
	return u.String(), nil
}

func (s *Server) acquire(ctx context.Context) bool {
	select {
	case s.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{"invalid JSON body"})
		return
	}
	target, err := checkURL(req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}
	opts := s.opts.Check
	opts.Profiles = profiles.FilterCategories(profiles.Filter(opts.Profiles, req.Agents), req.Categories)
	if len(opts.Profiles) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{"no profiles match the selected agents or categories"})
		return
	}
	opts.Render = req.Render && s.opts.Check.Render

	if !s.acquire(r.Context()) {
		return
	}
	defer func() { <-s.sem }()

	c, err := acurl.New(opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	rep, err := c.Check(ctx, target)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	target, err := checkURL(r.URL.Query().Get("url"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}
	if !s.acquire(r.Context()) {
		return
	}
	defer func() { <-s.sem }()
	c, err := acurl.New(s.opts.Check)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	cache := robots.NewCache(func(ctx context.Context, u string) (int, []byte, error) {
		return c.Get(ctx, u, robots.MaxSize)
	})
	audit := robots.Audit(r.Context(), cache, target, c.Profiles())
	audit.LLMsTxt, _ = llmstxt.Check(r.Context(), func(ctx context.Context, u string) (int, []byte, error) {
		return c.Get(ctx, u, llmstxt.MaxSize)
	}, target)
	writeJSON(w, http.StatusOK, audit)
}

// handleReport renders a JSON report (as returned by /api/check) in
// another format, for the UI's download buttons.
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	var rep types.ComparisonResult
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&rep); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{"invalid report JSON"})
		return
	}
	types := map[string]string{
		"html": "text/html; charset=utf-8", "markdown": "text/markdown; charset=utf-8",
		"csv": "text/csv; charset=utf-8", "sarif": "application/sarif+json", "junit": "application/xml",
	}
	ct, ok := types[format]
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"format must be html, markdown, csv, sarif or junit"})
		return
	}
	w.Header().Set("Content-Type", ct)
	if err := output.Write(w, format, rep, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
