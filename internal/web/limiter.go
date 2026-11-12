package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// loginLimiter slows brute-force attacks. Failed logins are counted under
// three kinds of key, each with its own limit inside the window; reaching a
// limit refuses further attempts for the lockout period:
//
//	user:NAME|IP  one address guessing one account's password         (10)
//	ip:IP         one address trying many accounts; higher, because an
//	              office or a phone carrier shares one address         (30)
//	name:NAME     many addresses guessing one account; high, so that a
//	              stranger cannot lock a known name out by failing a
//	              few times on purpose                                 (100)
//
// A successful login clears the first two for that person. The same type also
// counts sign-ups per address ("signup:IP").
type loginLimiter struct {
	mu       sync.Mutex
	failures map[string]*failureInfo
	limits   map[string]int // key prefix -> limit
	window   time.Duration
	lockout  time.Duration
}

func (l *loginLimiter) limitFor(key string) int {
	prefix, _, _ := strings.Cut(key, ":")
	if n, ok := l.limits[prefix]; ok {
		return n
	}
	return 10
}

type failureInfo struct {
	count     int
	first     time.Time
	lockedTil time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{failures: map[string]*failureInfo{},
		limits: map[string]int{"user": 10, "ip": 30, "name": 100, "signup": 10},
		window: 15 * time.Minute, lockout: 15 * time.Minute}
}

// loginKeys are the keys one login attempt counts under.
func loginKeys(ip, username string) []string {
	name := strings.ToLower(username)
	return []string{"user:" + name + "|" + ip, "ip:" + ip, "name:" + name}
}

// blocked reports whether any of the keys is currently locked and for how long.
func (l *loginLimiter) blocked(keys ...string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, k := range keys {
		if f := l.failures[k]; f != nil && f.lockedTil.After(now) {
			return true, f.lockedTil.Sub(now)
		}
	}
	return false, 0
}

func (l *loginLimiter) fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, k := range keys {
		f := l.failures[k]
		if f == nil || now.Sub(f.first) > l.window {
			f = &failureInfo{first: now}
			l.failures[k] = f
		}
		f.count++
		if f.count >= l.limitFor(k) {
			f.lockedTil = now.Add(l.lockout)
		}
	}
}

func (l *loginLimiter) reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.failures, k)
	}
}

// sweep drops stale entries (called by the janitor).
func (l *loginLimiter) sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, f := range l.failures {
		if now.Sub(f.first) > l.window && f.lockedTil.Before(now) {
			delete(l.failures, k)
		}
	}
}

// clientIP returns the caller's IP. X-Forwarded-For is trusted only when the
// connection comes from loopback (a reverse proxy on the same host), and then
// only its last entry: that is the one the proxy itself added. Anything before
// it was sent by the caller and can say whatever the caller likes.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
			return last
		}
	}
	return host
}
