package netguard

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowed(t *testing.T) {
	defer SetBlockPrivate(false)
	cases := []struct {
		ip             string
		always, strict bool // refused always / refused when the admin closed the local network
	}{
		{"169.254.169.254", true, true}, // cloud metadata
		{"fe80::1", true, true},
		{"127.0.0.1", false, true},
		{"::1", false, true},
		{"10.0.0.5", false, true},
		{"192.168.1.10", false, true},
		{"172.16.3.4", false, true},
		{"100.64.0.1", false, true},
		{"0.0.0.0", false, true},
		{"fd00::1", false, true},
		{"93.184.216.34", false, false},
		{"2606:4700::1111", false, false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		SetBlockPrivate(false)
		if got := Allowed(ip) != nil; got != c.always {
			t.Errorf("%s with the local network open: refused=%v, want %v", c.ip, got, c.always)
		}
		SetBlockPrivate(true)
		if got := Allowed(ip) != nil; got != c.strict {
			t.Errorf("%s with the local network closed: refused=%v, want %v", c.ip, got, c.strict)
		}
	}
}

// The rule is applied where the connection is opened, so it also holds for a
// redirect to a refused address. An admin's address is never checked.
func TestClientRefusesAtConnect(t *testing.T) {
	defer SetBlockPrivate(false)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("inside")) }))
	defer local.Close()

	get := func(c *http.Client, url string) error {
		resp, err := c.Get(url)
		if err == nil {
			resp.Body.Close()
		}
		return err
	}
	if err := get(Client(true), local.URL); err != nil {
		t.Fatalf("local network open, a user's address on this machine: %v", err)
	}
	Client(true).CloseIdleConnections() // a kept connection would skip the check below
	SetBlockPrivate(true)
	if err := get(Client(true), local.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("local network closed, a user's address on this machine: err = %v, want ErrBlocked", err)
	}
	if err := get(Client(false), local.URL); err != nil {
		t.Fatalf("an admin's address must always work: %v", err)
	}
}
