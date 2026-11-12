// Package netguard decides which network addresses the server may call on
// behalf of a user.
//
// Users can make Knitnox call any address they type in: a provider's base
// URL, a tool server, a picture link a provider sends back. Without a check,
// that lets a user reach things only the server can see: other machines on
// its network, services listening on the server itself, or a cloud host's
// metadata address, which hands out credentials.
//
// The check sits where the connection is opened, after the name was looked
// up, so a name that points somewhere else on its second look-up is caught
// too, and so is a redirect.
//
//   - Addresses in 169.254.0.0/16 and fe80::/10 (cloud metadata lives there)
//     are always refused for user-supplied addresses.
//   - This computer and the local network are refused when the admin turned
//     that on (Admin → Settings). It is off at first, because a tool server or
//     Ollama on the same machine is a normal thing to have.
//
// Addresses an admin set up (shared providers) are never checked.
package netguard

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"syscall"
	"time"
)

var blockPrivate atomic.Bool

// SetBlockPrivate turns the stricter rule on or off. It applies to the next connection.
func SetBlockPrivate(on bool) { blockPrivate.Store(on) }

// ErrBlocked is returned for an address users may not reach.
var ErrBlocked = errors.New("this address is not allowed")

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// Allowed reports whether a user-supplied address may be called.
func Allowed(ip net.IP) error {
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return fmt.Errorf("%w: %s is a link-local address (where cloud hosts keep their credentials)", ErrBlocked, ip)
	}
	if blockPrivate.Load() && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || cgnat.Contains(ip)) {
		return fmt.Errorf("%w: %s is on this server or its local network, and the admin has closed those to users' own providers and tools", ErrBlocked, ip)
	}
	return nil
}

func control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("%w: %q", ErrBlocked, host)
	}
	return Allowed(ip)
}

// Transport is for requests to addresses a user supplied.
var Transport http.RoundTripper = &http.Transport{
	Proxy: nil, // a proxy would hide the real address from the check
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   control,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          50,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   15 * time.Second,
	ExpectContinueTimeout: time.Second,
}

// Client returns the HTTP client for an address: the guarded one when a user
// supplied it, Go's default when an admin did.
func Client(userSupplied bool) *http.Client {
	if userSupplied {
		return guarded
	}
	return open
}

var (
	guarded = &http.Client{Transport: Transport}
	open    = &http.Client{}
)
