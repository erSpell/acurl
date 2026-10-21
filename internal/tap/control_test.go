package tap

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPingReportsSettings(t *testing.T) {
	p, err := New(Options{MaxBody: 42, Version: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8899"+PingPath, nil))
	var info Info
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Name != "acurl tap" || info.Version != "1.2.3" || info.MaxBody != 42 || info.PID == 0 {
		t.Errorf("info = %+v", info)
	}
}

func TestEventsStreamExchangesUntilClose(t *testing.T) {
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}), Options{})
	p := h.proxy.Config.Handler.(*Proxy)

	resp, err := http.Get(h.proxy.URL + EventsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	waitFor(t, func() bool { return p.hub.count() == 1 })

	r, err := http.Get(h.proxy.URL + "/test/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()

	lines := bufio.NewScanner(resp.Body)
	if !lines.Scan() {
		t.Fatalf("no event: %v", lines.Err())
	}
	var ev Event
	if err := json.Unmarshal(lines.Bytes(), &ev); err != nil || ev.Exchange == nil || ev.Exchange.RespBody.Text != "ok" {
		t.Fatalf("event = %s (%v)", lines.Bytes(), err)
	}

	p.Close()
	if lines.Scan() {
		t.Errorf("stream continued after Close: %s", lines.Bytes())
	}
}

func TestHubCountsDropsForSlowViewers(t *testing.T) {
	var h hub
	s := h.subscribe()
	for range subscriberBuffer + 3 {
		h.publish(&Exchange{})
	}
	if got := s.dropped.Load(); got != 3 {
		t.Errorf("dropped = %d, want 3", got)
	}
	h.unsubscribe(s)
	h.unsubscribe(s) // a second call is harmless
	h.close()
	if _, ok := <-h.subscribe().ch; ok {
		t.Error("subscribe after close returned an open channel")
	}
}

func TestStop(t *testing.T) {
	stopped := make(chan struct{})
	p, err := New(Options{OnStop: func() { close(stopped) }})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8899"+StopPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET stop = %d, want 405", rec.Code)
	}
	rec = httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8899"+StopPath, nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("POST stop = %d, want 204", rec.Code)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("OnStop not called")
	}
}

func TestProxyPassesWebSocketsThrough(t *testing.T) {
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			t.Errorf("upstream Upgrade = %q", r.Header.Get("Upgrade"))
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = rw.Flush()
		line, _ := rw.ReadString('\n')
		_, _ = rw.WriteString("echo " + line)
		_ = rw.Flush()
	}), Options{})

	conn, err := net.Dial("tcp", strings.TrimPrefix(h.proxy.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, "GET /test/v1/responses HTTP/1.1\r\nHost: 127.0.0.1\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake = %v, %v", resp, err)
	}
	_, _ = io.WriteString(conn, "hello\n")
	if got, _ := br.ReadString('\n'); got != "echo hello\n" {
		t.Errorf("frame = %q, want the upstream echo", got)
	}
	if x := h.next(t); x.Status != http.StatusSwitchingProtocols || x.Upgrade != "websocket" {
		t.Errorf("exchange = status %d, upgrade %q", x.Status, x.Upgrade)
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
