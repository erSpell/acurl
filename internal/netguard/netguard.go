// Package netguard stops requests to private and internal networks.
//
// acurl serve and acurl mcp fetch URLs chosen by someone else (a browser
// tab, an AI assistant). Without a guard they could be pointed at
// 127.0.0.1, the cloud metadata service or the office network (SSRF).
// The check runs on the resolved IP at connect time, so DNS names that
// resolve to private addresses, including after a rebinding, are refused.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"time"
)

// ErrBlocked is returned when a connection to a non-public address is refused.
var ErrBlocked = errors.New("connection to a private or internal address blocked")

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),      // "this" network
	netip.MustParsePrefix("100.64.0.0/10"),  // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),   // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"),  // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),    // reserved
	netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64
	netip.MustParsePrefix("2001:db8::/32"),  // documentation
	netip.MustParsePrefix("fec0::/10"),      // deprecated site-local
}

// Blocked reports whether ip must not be connected to.
func Blocked(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// control is a net.Dialer Control hook that refuses blocked addresses.
func control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("netguard: %w", err)
	}
	if Blocked(ip) {
		return fmt.Errorf("%w: %s", ErrBlocked, ip)
	}
	return nil
}

// DialContext returns a dial function that refuses private addresses.
func DialContext(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second, Control: control}
	return d.DialContext
}
