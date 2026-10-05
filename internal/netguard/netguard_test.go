package netguard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestBlocked(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "0.0.0.0",
		"100.64.0.1", "::1", "fd00::1", "fe80::1", "::ffff:127.0.0.1", "224.0.0.1",
	}
	public := []string{"93.184.215.14", "1.1.1.1", "2606:4700:4700::1111", "8.8.8.8"}
	for _, s := range blocked {
		if !Blocked(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range public {
		if Blocked(netip.MustParseAddr(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestDialRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	client := &http.Client{Transport: &http.Transport{DialContext: DialContext(time.Second)}}
	_, err := client.Get(srv.URL)
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("err = %v, want ErrBlocked", err)
	}
	if _, err := DialContext(time.Second)(context.Background(), "tcp", "localhost:1"); !errors.Is(err, ErrBlocked) {
		t.Errorf("localhost by name: err = %v", err)
	}
}
