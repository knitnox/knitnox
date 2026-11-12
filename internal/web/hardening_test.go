package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

func (a *app) signup(c *http.Client, name string) int {
	r, err := c.PostForm(a.srv.URL+"/register", url.Values{"username": {name}, "password": {"password123"}, "password2": {"password123"}})
	if err != nil {
		a.t.Fatal(err)
	}
	r.Body.Close()
	return r.StatusCode
}

// Out of the box the first account can be made and nobody else can sign up.
// When the admin opens sign-up, new accounts get the allowance the admin chose.
func TestSignupIsClosedUntilTheAdminOpensIt(t *testing.T) {
	a := newApp(t)
	if err := a.st.SaveSettings(store.DefaultSettings()); err != nil { // as a fresh install
		t.Fatal(err)
	}
	if code := a.signup(a.anon(), "alice"); code != http.StatusSeeOther {
		t.Fatalf("the first account: %d", code)
	}
	if code := a.signup(a.anon(), "mallory"); code != http.StatusForbidden {
		t.Fatalf("a second sign-up on a fresh install: %d, want 403", code)
	}
	if _, _, err := a.st.UserAuth("mallory"); err == nil {
		t.Fatal("mallory has an account")
	}

	st, _ := a.st.LoadSettings()
	st.AllowSignup, st.SignupTokenLimit, st.SignupPeriod = true, 5000, "daily"
	a.st.SaveSettings(st)
	if code := a.signup(a.anon(), "bob"); code != http.StatusSeeOther {
		t.Fatalf("sign-up after the admin opened it: %d", code)
	}
	bob, _, _ := a.st.UserAuth("bob")
	if bob.Role != "user" {
		t.Fatalf("bob is %s", bob.Role)
	}
	if q, _ := a.st.QuotaFor(bob.ID); q.TokenLimit != 5000 || q.Period != "daily" {
		t.Fatalf("bob's allowance = %+v", q)
	}
	alice, _, _ := a.st.UserAuth("alice")
	if q, _ := a.st.QuotaFor(alice.ID); alice.Role != "admin" || q.TokenLimit != 0 {
		t.Fatalf("alice = %s, allowance %+v", alice.Role, q)
	}
}

// One address cannot make accounts without end.
func TestSignupIsLimitedPerAddress(t *testing.T) {
	a := newApp(t)
	a.web.logins.limits["signup"] = 3
	codes := []int{}
	for _, name := range []string{"user1", "user2", "user3", "user4"} {
		codes = append(codes, a.signup(a.anon(), name))
	}
	if codes[0] != 303 || codes[1] != 303 || codes[2] != 303 || codes[3] != http.StatusBadRequest {
		t.Fatalf("codes = %v", codes)
	}
	if _, _, err := a.st.UserAuth("user4"); err == nil {
		t.Fatal("the fourth account was made")
	}
}

// A stranger failing on purpose locks only themselves out of an account, not
// its owner on another address; one address trying many names is stopped; and
// many addresses hammering one name are stopped in the end too.
func TestLoginLimiterKeys(t *testing.T) {
	l := newLoginLimiter()
	attacker, owner := loginKeys("203.0.113.9", "Alice"), loginKeys("198.51.100.4", "alice")
	for i := 0; i < 10; i++ {
		l.fail(attacker...)
	}
	if locked, _ := l.blocked(attacker...); !locked {
		t.Fatal("ten wrong passwords from one address were not stopped")
	}
	if locked, _ := l.blocked(owner...); locked {
		t.Fatal("the owner is locked out by someone else's wrong guesses")
	}

	l = newLoginLimiter()
	for i := 0; i < 30; i++ {
		l.fail(loginKeys("203.0.113.9", "name"+string(rune('a'+i)))...)
	}
	if locked, _ := l.blocked(loginKeys("203.0.113.9", "someone-new")...); !locked {
		t.Fatal("one address trying thirty names was not stopped")
	}

	l = newLoginLimiter()
	for i := 0; i < 100; i++ {
		l.fail(loginKeys("203.0.113."+string(rune('0'+i%10))+string(rune('0'+i/10)), "alice")...)
	}
	if locked, _ := l.blocked(owner...); !locked {
		t.Fatal("a hundred guesses at one name from many addresses were not stopped")
	}
}

// Behind a proxy on this machine the caller is the last address in
// X-Forwarded-For; what the caller put in front of it is ignored.
func TestClientIP(t *testing.T) {
	cases := []struct{ remote, xff, want string }{
		{"127.0.0.1:5000", "203.0.113.7", "203.0.113.7"},
		{"127.0.0.1:5000", "1.2.3.4, 203.0.113.7", "203.0.113.7"}, // 1.2.3.4 was typed by the caller
		{"[::1]:5000", "203.0.113.7", "203.0.113.7"},
		{"127.0.0.1:5000", "", "127.0.0.1"},
		{"127.0.0.1:5000", "not an address", "127.0.0.1"},
		{"198.51.100.4:5000", "203.0.113.7", "198.51.100.4"}, // not from a local proxy: the header is not believed
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if got := clientIP(r); got != c.want {
			t.Errorf("remote %s, header %q: %s, want %s", c.remote, c.xff, got, c.want)
		}
	}
}

// Whether an account exists or is disabled is told only to someone who knows its password.
func TestLoginSaysNothingAboutAccounts(t *testing.T) {
	a := newApp(t)
	a.register("alice")
	bob := a.register("bob")
	a.st.SetUserDisabled(bob.id, true)

	page := func(user, pass string) (int, string) {
		r, err := a.anon().PostForm(a.srv.URL+"/login", url.Values{"username": {user}, "password": {pass}})
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var b strings.Builder
		buf := make([]byte, 64<<10)
		for {
			n, err := r.Body.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return r.StatusCode, b.String()
	}
	_, unknown := page("nobody", "password123")
	_, wrong := page("alice", "wrong-password")
	_, disabledWrong := page("bob", "wrong-password")
	for name, body := range map[string]string{"unknown name": unknown, "wrong password": wrong, "disabled, wrong password": disabledWrong} {
		if !strings.Contains(body, "Wrong username or password.") || strings.Contains(body, "disabled") {
			t.Errorf("%s: the page says something else", name)
		}
	}
	if code, body := page("bob", "password123"); code != http.StatusUnauthorized || !strings.Contains(body, "This account is disabled") {
		t.Errorf("disabled with the right password: %d", code)
	}
}

// The health check answers "ok" while the database does, and says so when it does not.
func TestHealthCheckLooksAtTheDatabase(t *testing.T) {
	a := newApp(t)
	get := func() int {
		r, err := http.Get(a.srv.URL + "/healthz")
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if code := get(); code != 200 {
		t.Fatalf("healthy: %d", code)
	}
	a.st.Close()
	if code := get(); code != http.StatusServiceUnavailable {
		t.Fatalf("database closed: %d, want 503", code)
	}
}
