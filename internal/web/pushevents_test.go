package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"knitnox/internal/engine"
	"knitnox/internal/hub"
	"knitnox/internal/store"
)

// Notifications for the Feed and the Plan Book: who is told, what the
// notification opens, and the switches under Settings → Notifications.
func TestPushFeedAndPlan(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	for _, c := range []*client{alice, bob} { // both have a device that takes notifications
		a.st.SavePushSubscription(c.id, fmt.Sprintf("https://push.example/%d", c.id), "key", "auth")
	}
	var mu sync.Mutex
	var got []string
	a.web.pushSink = func(uid int64, p pushPayload) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, fmt.Sprintf("%d|%s|%s|%s", uid, p.Title, p.Body, p.URL))
	}
	sent := func() string {
		mu.Lock()
		defer mu.Unlock()
		out := strings.Join(got, "\n")
		got = nil
		return out
	}

	// An agent posts to alice's feed: she is told, and the notification opens the Feed at the post.
	agent := store.FeedActor{Kind: store.FeedByAgent, Name: "Scout"}
	id, err := a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "Three new invoices came in."}, agent)
	if err != nil {
		t.Fatal(err)
	}
	a.web.onFeedPost(engine.FeedEvent{UserID: alice.id, PostID: id, By: agent})
	if want := fmt.Sprintf("%d|Scout posted to your feed|Three new invoices came in.|/feed#fp-%d", alice.id, id); sent() != want {
		t.Fatalf("an agent's post: want %q", want)
	}
	// Not while her Feed is open, and not for a post she wrote herself.
	stop := a.web.watchThread(hub.FeedUser(alice.id), alice.id)
	a.web.onFeedPost(engine.FeedEvent{UserID: alice.id, PostID: id, By: agent})
	stop()
	a.web.onFeedPost(engine.FeedEvent{UserID: alice.id, PostID: id, By: store.FeedActor{Kind: store.FeedByUser}})
	if s := sent(); s != "" {
		t.Fatalf("told about a post she can see or wrote: %q", s)
	}
	// A post shared with everyone: the others are told, the author is not.
	a.st.SetFeedPublic(alice.id, id, true)
	a.web.onFeedPost(engine.FeedEvent{UserID: alice.id, PostID: id, Public: true, By: store.FeedActor{Kind: store.FeedByUser}})
	if want := fmt.Sprintf("%d|alice shared a post|Three new invoices came in.|/feed?view=public#fp-%d", bob.id, id); sent() != want {
		t.Fatalf("a shared post: want %q", want)
	}

	// The AI adds a task from a chat alice has left, then ticks it off.
	chat, err := a.st.CreateThread(alice.id, 0)
	if err != nil {
		t.Fatal(err)
	}
	ai := store.PlanActor{Kind: store.PlanByAI, ThreadID: chat}
	task, err := a.st.CreatePlanTask(alice.id, store.PlanTaskInput{Title: "Call Sam", Due: "2030-01-02"}, ai)
	if err != nil {
		t.Fatal(err)
	}
	a.web.onPlanTask(engine.PlanEvent{UserID: alice.id, TaskID: task, By: ai})
	a.web.onPlanTask(engine.PlanEvent{UserID: alice.id, TaskID: task, Done: true, By: ai})
	want := fmt.Sprintf("%d|Plan Book|The AI added a task: Call Sam (due 2030-01-02)|/plan#pt-%d\n%d|Plan Book|The AI ticked off: Call Sam|/plan?view=done#pt-%d", alice.id, task, alice.id, task)
	if s := sent(); s != want {
		t.Fatalf("plan changes:\n%s\nwant\n%s", s, want)
	}
	// Not while she is in that chat, and not for what she did herself.
	stop = a.web.watchThread(chat, alice.id)
	a.web.onPlanTask(engine.PlanEvent{UserID: alice.id, TaskID: task, By: ai})
	stop()
	a.web.onPlanTask(engine.PlanEvent{UserID: alice.id, TaskID: task, By: store.PlanActor{Kind: store.PlanByUser}})
	if s := sent(); s != "" {
		t.Fatalf("told about a task she saw being added: %q", s)
	}

	// The morning reminder: once a day, between 8 and 21 o'clock in her time zone, only when something is due.
	loc := a.st.MustLocation(alice.id)
	morning := time.Date(2030, 1, 2, 8, 30, 0, 0, loc)
	a.web.planReminders(time.Date(2030, 1, 2, 6, 0, 0, 0, loc))
	if s := sent(); s != "" {
		t.Fatalf("a reminder at night: %q", s)
	}
	a.web.planReminders(morning)
	a.web.planReminders(morning.Add(time.Hour))
	if want := fmt.Sprintf("%d|Plan Book|1 task is due today.|/plan?view=today", alice.id); sent() != want {
		t.Fatalf("the morning reminder: want %q", want)
	}
	a.web.planReminders(morning.AddDate(0, 0, 1))
	if want := fmt.Sprintf("%d|Plan Book|1 task is late.|/plan?view=today", alice.id); sent() != want {
		t.Fatalf("the next morning: want %q", want)
	}

	// The switches: every kind is on at first; alice switches Feed and Plan Book off.
	page := alice.get("/settings")
	for _, k := range pushKinds {
		if !strings.Contains(page, fmt.Sprintf(`name="kind" value="%s" checked`, k.Key)) {
			t.Errorf("the %q switch is not on at first", k.Key)
		}
	}
	if code, _, _ := alice.post("/settings/push/kinds", url.Values{"kind": {pushReply, pushApproval, pushAgent}}); code != http.StatusSeeOther {
		t.Fatalf("save the switches: %d", code)
	}
	if a.web.pushWanted(alice.id, pushFeed) || a.web.pushWanted(alice.id, pushPlan) || !a.web.pushWanted(alice.id, pushReply) || !a.web.pushWanted(bob.id, pushFeed) {
		t.Fatal("the switches were not kept as chosen")
	}
	a.web.onFeedPost(engine.FeedEvent{UserID: alice.id, PostID: id, By: agent})
	a.web.onPlanTask(engine.PlanEvent{UserID: alice.id, TaskID: task, By: ai})
	a.web.planReminders(morning.AddDate(0, 0, 2))
	if s := sent(); s != "" {
		t.Fatalf("told about kinds that are switched off: %q", s)
	}
	if page := alice.get("/settings"); strings.Contains(page, fmt.Sprintf(`name="kind" value="%s" checked`, pushFeed)) {
		t.Error("the Feed switch still shows as on")
	}
}
