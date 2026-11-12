package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

func TestHelpGuides(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")

	if got := alice.getStatus("/help"); got != http.StatusSeeOther {
		t.Fatalf("/help = %d, want a redirect to the user guide", got)
	}
	for path, want := range map[string]string{
		"/help/guide":      "Start in three steps",
		"/help/features":   "Known limits",
		"/help/developers": "Run it in two minutes",
		"/help/build":      "Cross-compilation: build for another system",
	} {
		page := alice.get(path)
		if !strings.Contains(page, want) {
			t.Errorf("%s is missing %q", path, want)
		}
		if strings.Contains(page, "<h1># ") || !strings.Contains(page, `id="help-body"`) {
			t.Errorf("%s did not render as a guide", path)
		}
	}
	if got := alice.getStatus("/help/nope"); got != http.StatusNotFound {
		t.Fatalf("unknown guide = %d", got)
	}
	if !strings.Contains(alice.get("/"), `href="/help"`) {
		t.Fatal("sidebar has no Help link")
	}
}

func TestIssues(t *testing.T) {
	a := newApp(t)
	admin := a.register("alice") // first account is the admin
	bob := a.register("bob")
	carol := a.register("carol")

	code, body, _ := bob.post("/issues", url.Values{"kind": {"idea"}, "title": {"Dark  mode   toggle"}, "body": {"A switch in <b>Settings</b>."}})
	if code != http.StatusSeeOther {
		t.Fatalf("create issue = %d %s", code, body)
	}
	if code, _, _ := bob.post("/issues", url.Values{"title": {"   "}}); code != http.StatusBadRequest {
		t.Fatalf("empty title = %d", code)
	}
	list, _ := a.st.ListIssues(store.IssueOpen, 0)
	if len(list) != 1 || list[0].Title != "Dark mode toggle" || list[0].Kind != "idea" || list[0].AuthorName != "bob" {
		t.Fatalf("issues = %+v", list)
	}
	path := fmt.Sprintf("/issues/%d", list[0].ID)

	// Everyone can read it; the text is escaped.
	page := carol.get("/issues")
	if !strings.Contains(page, "Dark mode toggle") || strings.Contains(page, "A switch in <b>Settings</b>") ||
		!strings.Contains(page, "A switch in &lt;b&gt;Settings&lt;/b&gt;") {
		t.Fatal("issue is not listed safely for other users")
	}
	if strings.Contains(page, `action="`+path+`"`) {
		t.Fatal("a stranger is offered actions on someone else's issue")
	}

	// Only the author or an admin can change it; only an admin can reply.
	for _, action := range []string{"done", "delete", "reply"} {
		if code, _, _ := carol.post(path, url.Values{"action": {action}, "reply": {"x"}}); code != http.StatusForbidden {
			t.Fatalf("stranger %s = %d", action, code)
		}
	}
	if code, _, _ := bob.post(path, url.Values{"action": {"reply"}, "reply": {"x"}}); code != http.StatusForbidden {
		t.Fatalf("author replying as admin = %d", code)
	}
	if code, _, _ := admin.post(path, url.Values{"action": {"reply"}, "reply": {"Added in the next build."}, "close": {"1"}}); code != http.StatusSeeOther {
		t.Fatalf("admin reply = %d", code)
	}
	got, _ := a.st.IssueByID(list[0].ID)
	if got.Status != store.IssueDone || got.Reply != "Added in the next build." || got.ReplyBy != "alice" {
		t.Fatalf("after reply = %+v", got)
	}
	if page := carol.get("/issues?show=done"); !strings.Contains(page, "Added in the next build.") {
		t.Fatal("reply is not shown")
	}
	if strings.Contains(carol.get("/issues"), "Dark mode toggle") {
		t.Fatal("a done issue is still listed as open")
	}
	if !strings.Contains(bob.get("/issues?show=mine"), "Dark mode toggle") || strings.Contains(carol.get("/issues?show=mine"), "Dark mode toggle") {
		t.Fatal("the Mine filter is wrong")
	}

	// The author reopens and deletes their own issue.
	bob.post(path, url.Values{"action": {"reopen"}})
	if got, _ := a.st.IssueByID(list[0].ID); got.Status != store.IssueOpen {
		t.Fatal("reopen did not work")
	}
	if code, _, _ := bob.post(path, url.Values{"action": {"delete"}}); code != http.StatusSeeOther {
		t.Fatalf("author delete = %d", code)
	}
	if _, err := a.st.IssueByID(list[0].ID); err == nil {
		t.Fatal("issue was not deleted")
	}

	// One account cannot flood the list.
	for i := 0; i < store.MaxOpenIssuesPerUser; i++ {
		if _, err := a.st.CreateIssue(carol.id, "carol", "bug", fmt.Sprintf("issue %d", i), ""); err != nil {
			t.Fatal(err)
		}
	}
	if code, _, _ := carol.post("/issues", url.Values{"title": {"one too many"}}); code != http.StatusTooManyRequests {
		t.Fatalf("over the limit = %d", code)
	}
}
