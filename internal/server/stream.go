package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tragent-io/acurl/internal/profiles"
	"github.com/tragent-io/acurl/pkg/acurl"
)

// handleStream runs a check and streams progress as server-sent events:
// "progress" after each agent request, then one "report" (or "error").
//
//	GET /api/check/stream?url=shop.example&categories=ai-assistant,ai-search
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, apiError{"streaming unsupported"})
		return
	}
	q := r.URL.Query()
	target, err := checkURL(q.Get("url"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}
	opts := s.opts.Check
	opts.Profiles = profiles.FilterCategories(profiles.Filter(opts.Profiles, split(q.Get("agents"))), split(q.Get("categories")))
	if len(opts.Profiles) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{"no profiles match the selected agents or categories"})
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")

	var mu sync.Mutex
	send := func(event string, v any) {
		data, _ := json.Marshal(v)
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	if !s.acquire(r.Context()) {
		return
	}
	defer func() { <-s.sem }()

	total := len(opts.Profiles)
	done := 0
	opts.Trace = func(e acurl.TraceEvent) {
		mu.Lock()
		done++
		n := done
		mu.Unlock()
		ev := map[string]any{"profile": e.Profile, "status": e.Status, "ms": e.Duration.Milliseconds(), "done": n, "total": total}
		if e.Err != nil {
			ev["error"] = e.Err.Error()
		}
		send("progress", ev)
	}
	c, err := acurl.New(opts)
	if err != nil {
		send("error", apiError{err.Error()})
		return
	}
	send("start", map[string]any{"url": target, "total": total})
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	rep, err := c.Check(ctx, target)
	if err != nil {
		send("error", apiError{err.Error()})
		return
	}
	send("report", rep)
}

func split(v string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}
