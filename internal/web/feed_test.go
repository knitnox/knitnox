package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// hxGet asks for a fragment the way htmx does.
func (c *client) hxGet(path string) (int, string) {
	req, _ := http.NewRequest(http.MethodGet, c.a.srv.URL+path, nil)
	req.Header.Set("HX-Request", "true")
	r, err := c.http.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b)
}

// The Feed page: write, change, share, delete and restore posts. A private
// post is never shown to anyone else, and only its owner can touch a post.
func TestFeedPageAndPermissions(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice") // admin
	bob := a.register("bob")

	page := alice.get("/feed")
	for _, want := range []string{`href="/feed" class="sb-link is-active" aria-current="page"`, "Let the AI post", `data-toggle="#add"`,
		"Your feed is empty", `href="/feed/settings"`, `id="feed-list"`, `name="body"`, `value="public"`, `href="/feed"`} {
		if !strings.Contains(page, want) {
			t.Errorf("empty Feed page is missing %q", want)
		}
	}

	code, card := alice.hx("/feed", url.Values{"body": {"  The van is booked for Friday.  "}, "report": {"## Details\nFriday at nine."}})
	if code != http.StatusOK || !strings.Contains(card, `id="fp-1"`) || !strings.Contains(card, "The van is booked for Friday.") ||
		!strings.Contains(card, "Only you") || !strings.Contains(card, "Read the full report") || strings.Contains(card, "Seen by") {
		t.Fatalf("create: %d %s", code, card)
	}
	if code, _ := alice.hx("/feed", url.Values{"body": {"  "}}); code != http.StatusBadRequest {
		t.Errorf("an empty post was accepted: %d", code)
	}
	alice.hx("/feed", url.Values{"body": {"Hello everyone, the printer works again."}, "visibility": {"public"}})
	alice.hx("/feed", url.Values{"body": {strings.Repeat("x", store.FeedBodyMax+50)}})
	if p, _ := a.st.FeedPostByID(alice.id, 3); len([]rune(p.Body)) != store.FeedBodyMax {
		t.Errorf("a long post was not cut to %d characters: %d", store.FeedBodyMax, len([]rune(p.Body)))
	}
	// Text is shown as text, never as markup.
	alice.hx("/feed", url.Values{"body": {`<script>alert(1)</script>`}, "report": {`<img src=x onerror=alert(1)>`}})
	if p := alice.get("/feed"); strings.Contains(p, "<script>alert(1)") || strings.Contains(p, "<img src=x") {
		t.Fatal("post text reached the page as markup")
	}

	// Bob sees only what alice shared, and it counts as seen once, however often he looks.
	for range 2 {
		page = bob.get("/feed")
	}
	if !strings.Contains(page, "the printer works again") || strings.Contains(page, "van is booked") || !strings.Contains(page, ">alice<") {
		t.Fatal("bob's feed does not show exactly alice's shared post")
	}
	if strings.Contains(page, `hx-post="/feed/p/2/delete"`) || strings.Contains(page, `hx-post="/feed/p/2/unshare"`) {
		t.Fatal("bob is offered actions on alice's post")
	}
	if p, _ := a.st.FeedPostByID(alice.id, 2); p.Reads != 1 {
		t.Fatalf("reads = %d, want 1", p.Reads)
	}
	if p := alice.get("/feed?view=mine"); !strings.Contains(p, "Seen by 1 person") {
		t.Error("alice is not told that one person saw her shared post")
	}
	alice.get("/feed") // her own look does not count
	if p, _ := a.st.FeedPostByID(alice.id, 2); p.Reads != 1 {
		t.Errorf("the owner's own look was counted: %d", p.Reads)
	}
	if p := bob.get("/feed?view=mine"); strings.Contains(p, "printer") || !strings.Contains(p, "Your feed is empty") {
		t.Error("bob's “Mine” view is wrong")
	}

	// Bob cannot reach alice's private post, or change any of her posts, by id.
	for _, path := range []string{"/feed/p/1", "/feed/p/1/edit", "/feed/p/2/edit", "/feed/p/1/picture.svg"} {
		if code := bob.getStatus(path); code != http.StatusNotFound {
			t.Errorf("bob GET %s = %d, want 404", path, code)
		}
	}
	if code := bob.getStatus("/feed/p/2"); code != http.StatusOK {
		t.Errorf("bob cannot open the shared post: %d", code)
	}
	for _, path := range []string{"/feed/p/2", "/feed/p/2/share", "/feed/p/2/delete", "/feed/p/2/restore", "/feed/p/2/purge", "/feed/p/1/share"} {
		if code, _, _ := bob.post(path, url.Values{"body": {"hacked"}, "public": {"1"}}); code != http.StatusNotFound {
			t.Errorf("bob POST %s = %d, want 404", path, code)
		}
	}
	if code, _, _ := bob.post("/feed/p/2/unshare", nil); code != http.StatusForbidden {
		t.Errorf("a member used the admin action: %d", code)
	}
	if p, _ := a.st.FeedPostByID(alice.id, 2); p.Body != "Hello everyone, the printer works again." || !p.Public || p.InBin() {
		t.Fatalf("alice's post changed: %+v", p)
	}
	if p, _ := a.st.FeedPostByID(alice.id, 1); p.Public {
		t.Fatal("bob shared alice's private post")
	}

	// Change in place, share, make private again.
	if form := alice.get("/feed/p/1/edit"); !strings.Contains(form, `hx-post="/feed/p/1"`) || !strings.Contains(form, "The van is booked for Friday.") || !strings.Contains(form, "Friday at nine.") {
		t.Errorf("edit form: %s", form)
	}
	code, card = alice.hx("/feed/p/1", url.Values{"body": {"The bakery order moved to Monday."}, "report": {""}})
	if code != http.StatusOK || !strings.Contains(card, "moved to Monday") || strings.Contains(card, "Read the full report") || !strings.Contains(card, "changed") {
		t.Fatalf("update: %d %s", code, card)
	}
	if _, card = alice.hx("/feed/p/1/share", url.Values{"public": {"1"}}); !strings.Contains(card, "Shared") || !strings.Contains(card, "Make it private again") {
		t.Fatalf("share: %s", card)
	}
	if !strings.Contains(bob.get("/feed"), "moved to Monday") {
		t.Error("a shared post does not reach bob")
	}
	alice.hx("/feed/p/1/share", url.Values{"public": {"0"}})
	if strings.Contains(bob.get("/feed"), "moved to Monday") {
		t.Fatal("a post made private again is still shown to bob")
	}

	// Delete leaves a line with Undo; the bin keeps the post; purge is final.
	if _, gone := alice.hx("/feed/p/1/delete", nil); !strings.Contains(gone, "in the bin") || !strings.Contains(gone, `hx-post="/feed/p/1/restore"`) || !strings.Contains(gone, `id="fp-1"`) {
		t.Fatalf("delete: %s", gone)
	}
	if strings.Contains(alice.get("/feed"), "moved to Monday") || !strings.Contains(alice.get("/feed?view=bin"), "moved to Monday") {
		t.Fatal("a deleted post is not in the bin")
	}
	if code := alice.getStatus("/feed/p/1/edit"); code != http.StatusBadRequest {
		t.Errorf("a binned post can be edited: %d", code)
	}
	if _, card = alice.hx("/feed/p/1/restore", nil); !strings.Contains(card, "moved to Monday") || !strings.Contains(card, `class="feed-post"`) {
		t.Fatalf("undo: %s", card)
	}
	if code, _ := alice.hx("/feed/p/1/purge", nil); code != http.StatusNotFound {
		t.Errorf("a post outside the bin was purged: %d", code)
	}
	alice.hx("/feed/p/1/delete", nil)
	if code, body := alice.hx("/feed/p/1/purge", nil); code != http.StatusOK || strings.TrimSpace(body) != "" {
		t.Fatalf("purge: %d %q", code, body)
	}
	if _, err := a.st.FeedPostByID(alice.id, 1); err == nil {
		t.Fatal("a purged post still exists")
	}

	// A deleted shared post is gone for everyone at once.
	alice.hx("/feed/p/2/delete", nil)
	if strings.Contains(bob.get("/feed"), "printer") || bob.getStatus("/feed/p/2") != http.StatusNotFound {
		t.Fatal("a binned post is still public")
	}
	alice.hx("/feed/p/2/restore", nil)

	// An admin can take someone's post out of the public feed; its owner keeps it.
	bob.hx("/feed", url.Values{"body": {"Bob's shared post."}, "visibility": {"public"}})
	bobs, _, _ := a.st.ListFeed(bob.id, store.FeedFilter{View: store.FeedViewMine})
	item := fmt.Sprintf("/feed/p/%d", bobs[0].ID)
	if p := alice.get("/feed"); !strings.Contains(p, `hx-post="`+item+`/unshare"`) || strings.Contains(p, `hx-post="`+item+`/delete"`) {
		t.Fatal("the admin is not offered exactly “take out of the public feed” on bob's post")
	}
	if code, gone := alice.hx(item+"/unshare", nil); code != http.StatusOK || !strings.Contains(gone, "Taken out of the public feed") {
		t.Fatalf("unshare: %d %s", code, gone)
	}
	if p, err := a.st.FeedPostByID(bob.id, bobs[0].ID); err != nil || p.Public || p.InBin() {
		t.Fatalf("after the admin's action bob's post = %+v, %v", p, err)
	}

	// When the server does not allow sharing, everyone sees only their own posts.
	st, _ := a.st.LoadSettings()
	st.FeedPublic = false
	a.st.SaveSettings(st)
	if p := bob.get("/feed?view=all"); strings.Contains(p, "printer") || strings.Contains(p, `value="public"`) || strings.Contains(p, ">Shared<") {
		t.Fatal("shared posts are shown although sharing is turned off")
	}
	if code := bob.getStatus("/feed/p/2"); code != http.StatusNotFound {
		t.Errorf("a shared post can be opened although sharing is off: %d", code)
	}
	bob.hx("/feed", url.Values{"body": {"Tried to share."}, "visibility": {"public"}})
	if posts, _, _ := a.st.ListFeed(bob.id, store.FeedFilter{View: store.FeedViewMine}); posts[0].Public {
		t.Fatal("a post was shared although sharing is turned off")
	}
	if code, _ := bob.hx(fmt.Sprintf("/feed/p/%d/share", bobs[0].ID), url.Values{"public": {"1"}}); code != http.StatusForbidden {
		t.Errorf("sharing while it is turned off: %d", code)
	}
}

// The first page holds 20 posts, each next step 10 more; new posts are counted
// and fetched apart, so the page can announce them instead of moving.
func TestFeedPagingAndNewPosts(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	for i := 1; i <= 35; i++ {
		if _, err := a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: fmt.Sprintf("Post number %d.", i)}, store.FeedActor{Kind: store.FeedByAgent, Name: "Scout"}); err != nil {
			t.Fatal(err)
		}
	}
	cards := func(s string) int { return strings.Count(s, `class="feed-post`) }

	if n := a.st.FeedUnread(alice.id, true); n != 35 {
		t.Fatalf("unread = %d, want 35", n)
	}
	if home := alice.get("/"); !strings.Contains(home, `title="35 new posts"`) {
		t.Error("the sidebar does not count the new posts")
	}
	page := alice.get("/feed")
	if cards(page) != 20 || !strings.Contains(page, "Post number 35.") || strings.Contains(page, "Post number 15.") ||
		!strings.Contains(page, `hx-get="/feed/more?before=16&view=all"`) || !strings.Contains(page, ">New<") {
		t.Fatalf("first page: %d cards", cards(page))
	}
	// Looking at the feed clears the count; the posts are not “new” the second time.
	if n := a.st.FeedUnread(alice.id, true); n != 0 {
		t.Errorf("unread after looking = %d", n)
	}
	if strings.Contains(alice.get("/feed"), ">New<") {
		t.Error("posts are still marked new on the second look")
	}

	code, more := alice.hxGet("/feed/more?before=16&view=all")
	if code != http.StatusOK || cards(more) != 10 || !strings.Contains(more, "Post number 15.") || !strings.Contains(more, "/feed/more?before=6&view=all") {
		t.Fatalf("second step: %d, %d cards", code, cards(more))
	}
	if _, last := alice.hxGet("/feed/more?before=6&view=all"); cards(last) != 5 || strings.Contains(last, "feed-more") {
		t.Fatalf("last step: %d cards", cards(last))
	}
	if code, _ := alice.hxGet("/feed/more?view=all"); code != http.StatusBadRequest {
		t.Errorf("more without a position: %d", code)
	}

	// Two posts arrive. The count says so; fetching them brings exactly those two.
	a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "Fresh one."}, store.FeedActor{Kind: store.FeedByAI})
	a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "Fresh two."}, store.FeedActor{Kind: store.FeedByAgent, Name: "Scout"})
	var count struct{ N int }
	json.Unmarshal([]byte(alice.get("/feed/count?after=35&view=all")), &count)
	if count.N != 2 {
		t.Fatalf("count = %d, want 2", count.N)
	}
	json.Unmarshal([]byte(alice.get("/feed/count?after=35&view=all&from=ai")), &count)
	if count.N != 1 {
		t.Errorf("count for the AI only = %d, want 1", count.N)
	}
	_, fresh := alice.hxGet("/feed/new?after=35&view=all")
	if cards(fresh) != 2 || !strings.Contains(fresh, "Fresh one.") || !strings.Contains(fresh, "Fresh two.") || strings.Contains(fresh, "feed-more") || !strings.Contains(fresh, ">New<") {
		t.Fatalf("new posts: %s", fresh)
	}

	// Filters: by kind of author, by agent, by word.
	if p := alice.get("/feed?from=ai"); cards(p) != 1 || !strings.Contains(p, "Fresh one.") {
		t.Error("the “AI in chats” filter is wrong")
	}
	if p := alice.get("/feed?agent=Scout"); cards(p) != 20 || strings.Contains(p, "Fresh one.") || !strings.Contains(p, "agent=Scout") {
		t.Error("the agent filter is wrong")
	}
	if p := alice.get("/feed?q=number+7."); cards(p) != 1 || !strings.Contains(p, "Post number 7.") {
		t.Error("finding a post by word is wrong")
	}
	if p := alice.get("/feed?from=user"); cards(p) != 0 || !strings.Contains(p, "Nothing here") {
		t.Error("an empty filter does not say so")
	}
}

// A chart is drawn by the server from numbers; a picture drawn by the AI is
// only handed out as an image that can do nothing.
func TestFeedChartAndPicture(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	chart := &store.FeedChart{Type: "bar", Title: `Visits <b>`, Unit: "%", Labels: []string{`Mon"><script>`, "Tue"}, Values: []float64{12.5, -3}}
	evil := `<svg viewBox="0 0 10 10"><script>alert(document.cookie)</script><circle r="4"/></svg>`
	id, err := a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "Numbers.", Chart: chart, SVG: evil}, store.FeedActor{Kind: store.FeedByAI})
	if err != nil {
		t.Fatal(err)
	}
	page := alice.get("/feed")
	for _, want := range []string{`class="feed-chart-svg"`, `class="fc-bar"`, "12.5%", "-3%", "Visits &lt;b&gt;", fmt.Sprintf(`<img src="/feed/p/%d/picture.svg"`, id)} {
		if !strings.Contains(page, want) {
			t.Errorf("the post is missing %q", want)
		}
	}
	if strings.Contains(page, "<script>alert") || strings.Contains(page, `Mon"><script>`) {
		t.Fatal("chart or picture text reached the page as markup")
	}

	req, _ := http.NewRequest(http.MethodGet, a.srv.URL+fmt.Sprintf("/feed/p/%d/picture.svg", id), nil)
	r, err := alice.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if ct, csp := r.Header.Get("Content-Type"), r.Header.Get("Content-Security-Policy"); !strings.HasPrefix(ct, "image/svg+xml") ||
		!strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "sandbox") || strings.Contains(csp, "script-src") {
		t.Fatalf("picture headers: %q, %q", ct, csp)
	}
	if code := bob.getStatus(fmt.Sprintf("/feed/p/%d/picture.svg", id)); code != http.StatusNotFound {
		t.Errorf("bob can load the picture of a private post: %d", code)
	}

	line := drawFeedChart(store.FeedChart{Type: "line", Labels: []string{"a", "b", "c"}, Values: []float64{1, 5, 3}})
	if !strings.Contains(string(line), `class="fc-line"`) || strings.Count(string(line), `class="fc-dot"`) != 3 {
		t.Errorf("line chart: %s", line)
	}
}

// Who may post: the user's switch for the AI in chats, and one switch and a
// daily limit per agent, set by the agent's owner only.
func TestFeedSwitches(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")

	if code, _, h := alice.post("/feed/settings", url.Values{"enabled": {"1"}}); code != http.StatusSeeOther || h.Get("Location") != "/feed/settings?done=on" {
		t.Fatalf("turn on: %d %s", code, h.Get("Location"))
	}
	if !a.st.FeedEnabled(alice.id) || a.st.FeedEnabled(bob.id) {
		t.Fatal("the switch did not change exactly alice's setting")
	}
	if p := alice.get("/feed/settings?done=on"); !strings.Contains(p, "The AI can now post to your feed") || !strings.Contains(p, "Turn off") {
		t.Error("the page does not show that the AI may post now")
	}

	aid, err := a.st.CreateAgent(&store.Agent{UserID: alice.id, Name: "Scout", Instructions: "Look.", MaxSteps: 5, DailyRuns: 5, HistoryRuns: 3}, 0)
	if err != nil {
		t.Fatal(err)
	}
	access := fmt.Sprintf("/agents/%d/settings", aid)
	form := func(more url.Values) url.Values {
		v := url.Values{"name": {"Scout"}, "instructions": {"Look."}}
		for k, val := range more {
			v[k] = val
		}
		return v
	}
	if ag, _ := a.st.AgentByID(aid); ag.FeedAccess != store.FeedAccessNone || ag.FeedDaily != store.FeedDailyDef {
		t.Fatalf("a new agent starts with feed access %q, %d a day", ag.FeedAccess, ag.FeedDaily)
	}
	code, _ := alice.hx(access, form(url.Values{"feed_post": {"1"}, "feed_daily": {"999"}}))
	panel := alice.get(fmt.Sprintf("/agents/%d/panel", aid))
	if code != http.StatusOK || !strings.Contains(panel, `name="feed_post" value="1" checked`) || !strings.Contains(panel, `name="feed_daily" type="number" min="1" max="50" value="50"`) {
		t.Fatalf("allow: %d", code)
	}
	if ag, _ := a.st.AgentByID(aid); ag.FeedAccess != store.FeedAccessPost || ag.FeedDaily != store.FeedDailyMax {
		t.Fatalf("after allowing: %q, %d", ag.FeedAccess, ag.FeedDaily)
	}
	if code, _ := bob.hx(access, form(url.Values{"feed_post": {"1"}})); code != http.StatusNotFound {
		t.Errorf("bob changed alice's agent: %d", code)
	}
	alice.hx(access, form(url.Values{"feed_daily": {"3"}})) // the box unticked
	if ag, _ := a.st.AgentByID(aid); ag.FeedAccess != store.FeedAccessNone || ag.FeedDaily != 3 {
		t.Fatalf("after stopping: %q, %d", ag.FeedAccess, ag.FeedDaily)
	}
}
