package web

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"knitnox/internal/store"
)

func (a *app) anon() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (a *app) login(c *http.Client, user, pass string) int {
	r, err := c.PostForm(a.srv.URL+"/login", url.Values{"username": {user}, "password": {pass}})
	if err != nil {
		a.t.Fatal(err)
	}
	r.Body.Close()
	return r.StatusCode
}

// Sign-up asks for the password twice and refuses when they differ.
func TestSignupNeedsThePasswordTwice(t *testing.T) {
	a := newApp(t)
	c := a.anon()
	for _, second := range []string{"", "password124"} {
		r, _ := c.PostForm(a.srv.URL+"/register", url.Values{"username": {"alice"}, "password": {"password123"}, "password2": {second}})
		r.Body.Close()
		if r.StatusCode != http.StatusBadRequest {
			t.Fatalf("second password %q: status %d", second, r.StatusCode)
		}
	}
	if n, _ := a.st.CountUsers(); n != 0 {
		t.Fatalf("%d accounts were created", n)
	}
	a.register("alice") // the helper types it twice
	if page := a.anon(); a.login(page, "alice", "password123") != http.StatusSeeOther {
		t.Fatal("alice cannot sign in")
	}
}

func TestChangeUsername(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice") // admin
	bob := a.register("bob")

	if code, body, _ := bob.hxPost("/settings/username", url.Values{"username": {"robert"}, "current": {"wrong-password"}}); code != 200 || !strings.Contains(body, "password is wrong") {
		t.Fatalf("wrong password: %d %q", code, body)
	}
	if code, body, _ := bob.hxPost("/settings/username", url.Values{"username": {"Alice"}, "current": {"password123"}}); code != 200 || !strings.Contains(body, "taken") {
		t.Fatalf("taken name: %d %q", code, body)
	}
	if code, body, _ := bob.hxPost("/settings/username", url.Values{"username": {"x"}, "current": {"password123"}}); code != 200 || !strings.Contains(body, "3 to 32") {
		t.Fatalf("bad name: %d %q", code, body)
	}
	if u, _ := a.st.UserByID(bob.id); u.Username != "bob" {
		t.Fatalf("bob was renamed to %q by a refused request", u.Username)
	}
	bob.hxPost("/settings/username", url.Values{"username": {"robert"}, "current": {"password123"}})
	if u, _ := a.st.UserByID(bob.id); u.Username != "robert" {
		t.Fatalf("username = %q", u.Username)
	}
	if !strings.Contains(bob.get("/settings"), "robert") {
		t.Fatal("bob's session did not survive the rename")
	}
	if a.login(a.anon(), "robert", "password123") != http.StatusSeeOther {
		t.Fatal("cannot sign in with the new name")
	}
	if a.login(a.anon(), "bob", "password123") != http.StatusUnauthorized {
		t.Fatal("the old name still signs in")
	}

	// An admin renames someone; a normal user cannot.
	if code, _, _ := bob.hxPost("/admin/users/"+itoa(alice.id)+"/username", url.Values{"username": {"hacked"}}); code != http.StatusForbidden {
		t.Fatalf("a user renamed the admin: %d", code)
	}
	alice.hxPost("/admin/users/"+itoa(bob.id)+"/username", url.Values{"username": {"bobby"}})
	if u, _ := a.st.UserByID(bob.id); u.Username != "bobby" {
		t.Fatalf("after the admin's rename: %q", u.Username)
	}
	if code, body, _ := alice.hxPost("/admin/users/"+itoa(bob.id)+"/username", url.Values{"username": {"alice"}}); !strings.Contains(body, "taken") {
		t.Fatalf("admin took their own name for bob: %d %q", code, body)
	}
}

// Changing a password needs it twice and signs the other devices out.
func TestChangePasswordSignsOtherDevicesOut(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	other := a.anon() // alice on a second device
	a.login(other, "alice", "password123")
	otherGet := func() int {
		r, _ := other.Get(a.srv.URL + "/settings")
		r.Body.Close()
		return r.StatusCode
	}
	if otherGet() != 200 {
		t.Fatal("second device is not signed in")
	}
	if _, body, _ := alice.hxPost("/settings/password", url.Values{"current": {"password123"}, "new": {"new-password-1"}, "new2": {"new-password-2"}}); !strings.Contains(body, "not the same") {
		t.Fatalf("mismatch accepted: %q", body)
	}
	if a.login(a.anon(), "alice", "password123") != http.StatusSeeOther {
		t.Fatal("the password changed although the two did not match")
	}
	if _, body, _ := alice.hxPost("/settings/password", url.Values{"current": {"password123"}, "new": {"new-password-1"}, "new2": {"new-password-1"}}); !strings.Contains(body, "Password changed") {
		t.Fatalf("change refused: %q", body)
	}
	if !strings.Contains(alice.get("/settings"), "alice") {
		t.Fatal("the device that changed the password was signed out")
	}
	if otherGet() == 200 {
		t.Fatal("the second device is still signed in")
	}
	if a.login(a.anon(), "alice", "new-password-1") != http.StatusSeeOther {
		t.Fatal("cannot sign in with the new password")
	}
}

// The admin's "Back up now" writes a backup that opens, keeps only the newest
// ones, and shows its state on the settings page.
func TestAdminBackup(t *testing.T) {
	a := newApp(t)
	dir := t.TempDir()
	a.web.backupDir, a.web.dataDir = dir, filepath.Dir(a.st.UploadDir)
	alice := a.register("alice")
	bob := a.register("bob")

	if code, _, _ := bob.hxPost("/admin/backup", url.Values{}); code != http.StatusForbidden {
		t.Fatalf("a normal user started a backup: %d", code)
	}
	old := filepath.Join(dir, "knitnox-20200101-000000")
	os.MkdirAll(old, 0o700)
	st, _ := a.st.LoadSettings()
	st.BackupKeep = 1
	a.st.SaveSettings(st)

	if code, body, _ := alice.hxPost("/admin/backup", url.Values{}); code != 200 || !strings.Contains(body, "Backup written") {
		t.Fatalf("backup: %d %q", code, body)
	}
	names, _ := store.ListBackups(dir)
	if len(names) != 1 || names[0] == "knitnox-20200101-000000" {
		t.Fatalf("backups after pruning: %v", names)
	}
	if err := store.CheckDatabaseFile(filepath.Join(dir, names[0], "knitnox.db")); err != nil {
		t.Fatal(err)
	}
	if page := alice.get("/admin/settings"); !strings.Contains(page, "Last backup:") {
		t.Fatal("the settings page does not show the backup")
	}

	// Automatic: nothing is due right after a backup; one is once the last is old.
	a.web.autoBackup()
	if names, _ := store.ListBackups(dir); len(names) != 1 {
		t.Fatalf("a second backup ran at once: %v", names)
	}
	a.st.SetValue(kvBackupAt, "2020-01-01 00:00:00")
	os.Rename(filepath.Join(dir, names[0]), filepath.Join(dir, "knitnox-20200102-000000")) // free this second's name
	a.web.autoBackup()
	if got, _ := store.ListBackups(dir); len(got) != 1 || got[0] == "knitnox-20200102-000000" {
		t.Fatalf("the due backup did not run: %v", got)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// hxPost sends a form the way htmx does: the answer is a message, not a redirect.
func (c *client) hxPost(path string, form url.Values) (int, string, http.Header) {
	req, _ := http.NewRequest(http.MethodPost, c.a.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.Header.Set("HX-Request", "true")
	r, err := c.http.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, strings.TrimSpace(string(b)), r.Header
}
