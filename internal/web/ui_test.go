package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// setCookie puts a browser-side cookie (the kind app.js writes) into the jar.
func (c *client) setCookie(name, value string) {
	u, _ := url.Parse(c.a.srv.URL)
	c.http.Jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
}

func TestAppearanceCookies(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")

	if page := alice.get("/"); !strings.Contains(page, `<html lang="en" data-theme="auto" data-accent="iris">`) {
		t.Fatal("default appearance is not auto + iris")
	}
	alice.setCookie("kn_theme", "dark")
	alice.setCookie("kn_accent", "forest")
	page := alice.get("/settings")
	if !strings.Contains(page, `data-theme="dark" data-accent="forest"`) {
		t.Fatal("appearance cookies are not applied to the page")
	}
	if !strings.Contains(page, `data-theme-set="dark" aria-pressed="true"`) || !strings.Contains(page, `data-accent-set="forest" aria-pressed="true"`) {
		t.Fatal("Settings does not show the current appearance as selected")
	}

	// Anything unknown falls back to the defaults instead of reaching the page.
	alice.setCookie("kn_theme", `"><script>`)
	alice.setCookie("kn_accent", "neon")
	if page := alice.get("/"); !strings.Contains(page, `data-theme="auto" data-accent="iris"`) {
		t.Fatal("unknown appearance values were not replaced by the defaults")
	}
}

func TestNavigationAndGettingStarted(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice") // admin
	bob := a.register("bob")

	home := alice.get("/")
	for _, want := range []string{
		`class="skip-link" href="#main"`, `id="main"`, // keyboard users can skip the sidebar
		`data-open-palette`, `id="palette"`, `Search inside messages`, // search and jump
		`id="start"`, `Connect an AI model`, `href="/admin/providers"`, // getting started, admin wording
		`href="/settings"`, `href="/admin"`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("home is missing %q", want)
		}
	}
	bobHome := bob.get("/")
	if strings.Contains(bobHome, `href="/admin`) {
		t.Error("a member is offered admin links")
	}
	if !strings.Contains(bobHome, `href="/settings#keys"`) {
		t.Error("a member is not pointed to their own API keys")
	}

	// The current page is marked in the sidebar.
	if page := alice.get("/agents"); !strings.Contains(page, `href="/agents" class="sb-link is-active" aria-current="page"`) {
		t.Error("the Agents link is not marked as current")
	}

	// Hiding the list sticks (app.js sets this cookie).
	alice.setCookie("kn_start", "off")
	if strings.Contains(alice.get("/"), `id="start"`) {
		t.Error("the getting-started list is shown after it was hidden")
	}
}

// The time zone comes from the device (app.js) unless the user fixed it by hand.
func TestTimezoneSetting(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")

	if page := alice.get("/settings"); !strings.Contains(page, `data-tz="" data-tz-auto="true"`) || !strings.Contains(page, "zone not known yet") {
		t.Fatal("a new account does not ask the device for its time zone")
	}
	// What app.js sends when the device's zone differs from the saved one.
	if code, _, _ := alice.post("/settings/timezone", url.Values{"tz": {"Asia/Dhaka"}, "auto": {"1"}}); code != http.StatusNoContent {
		t.Fatalf("device zone: %d", code)
	}
	if page := alice.get("/settings"); !strings.Contains(page, `data-tz="Asia/Dhaka"`) || !strings.Contains(page, `value="Asia/Dhaka"`) || strings.Contains(page, "zone not known yet") {
		t.Fatal("the saved zone is not shown")
	}
	if code, _, _ := alice.post("/settings/timezone", url.Values{"tz": {"Mars/Olympus"}, "auto": {"1"}}); code != http.StatusBadRequest {
		t.Errorf("a made-up zone: %d", code)
	}

	// Fixing it by hand (the Settings form, sent by htmx) stops the device from changing it.
	req, _ := http.NewRequest(http.MethodPost, a.srv.URL+"/settings/timezone", strings.NewReader(url.Values{"tz": {"Europe/Lisbon"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", alice.csrf)
	req.Header.Set("HX-Request", "true")
	r, err := alice.http.Do(req)
	if err != nil || r.StatusCode != http.StatusOK || r.Header.Get("HX-Retarget") != "#flash" {
		t.Fatalf("settings form: %v %v", err, r)
	}
	r.Body.Close()
	alice.post("/settings/timezone", url.Values{"tz": {"Asia/Dhaka"}, "auto": {"1"}})
	if u, _ := a.st.UserByID(alice.id); u.Timezone != "Europe/Lisbon" || u.TimezoneAuto {
		t.Fatalf("after fixing by hand: %q auto=%v", u.Timezone, u.TimezoneAuto)
	}
}
