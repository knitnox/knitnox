package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// loginLimiter slows brute-force attacks: after too many failed logins for
// one username (maxPerUser) or from one IP (maxPerIP, higher because many
// people may share an office or carrier address) within the window, further
// attempts are refused for the lockout period. Successful logins clear the
// counters. Keys are prefixed "user:" or "ip:".
type loginLimiter struct {
	mu         sync.Mutex
	failures   map[string]*failureInfo
	maxPerUser int
	maxPerIP   int
	window     time.Duration
	lockout    time.Duration
}

func (l *loginLimiter) limitFor(key string) int {
	if strings.HasPrefix(key, "ip:") {
		return l.maxPerIP
	}
	return l.maxPerUser
}

type failureInfo struct {
	count     int
	first     time.Time
	lockedTil time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{failures: map[string]*failureInfo{}, maxPerUser: 10, maxPerIP: 30, window: 15 * time.Minute, lockout: 15 * time.Minute}
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

// clientIP returns the caller's IP. X-Forwarded-For is trusted only when
// the connection comes from loopback (i.e. a reverse proxy on the same host).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first, _, _ := strings.Cut(xff, ","); strings.TrimSpace(first) != "" {
				return strings.TrimSpace(first)
			}
		}
	}
	return host
}
