package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"knitnox/internal/store"
)

func TestTimeStamps(t *testing.T) {
	now := time.Date(2026, 11, 11, 13, 5, 0, 0, time.UTC)
	dhaka, err := time.LoadLocation("Asia/Dhaka")
	if err != nil {
		t.Fatal("time zone data is missing from the binary:", err)
	}
	if got := timeStamp(now, nil); got != "[Wed, 11 Nov 2026, 13:05]" {
		t.Errorf("stamp in UTC = %q", got)
	}
	if got := messageStamp("2026-11-11 20:30:00", dhaka); got != "[Thu, 12 Nov 2026, 02:30]" { // already tomorrow further east
		t.Errorf("stamp in Dhaka = %q", got)
	}
	if got := messageStamp("not a time", dhaka); got != "" {
		t.Errorf("stamp of a broken time = %q", got)
	}
	if got := clockLine(now, dhaka); got != "Wednesday, 11 November 2026, 19:05 (Asia/Dhaka, UTC+06:00)" {
		t.Errorf("clock in Dhaka = %q", got)
	}
	if got := clockGuide(dhaka, false); !strings.Contains(got, "the user's time zone (Asia/Dhaka)") {
		t.Errorf("guide = %q", got)
	}
	if got := clockGuide(time.Local, true); !strings.Contains(got, "the server's time zone (UTC") {
		t.Errorf("group guide = %q", got)
	}
	// The context meter of a group chat passes no zone; this used to panic.
	if got := clockGuide(nil, true); !strings.Contains(got, "the server's time zone (UTC+00:00)") {
		t.Errorf("group guide without a zone = %q", got)
	}
}

var stampRe = regexp.MustCompile(`^\[\w{3}, \d{1,2} \w{3} \d{4}, \d\d:\d\d\]\n`)

// Every user message carries the time it was sent; the system prompt carries
// none, so the start of the prompt is identical from one turn to the next
// (which is what provider-side prompt caching needs).
func TestRunsAreToldTheTimeWithoutBreakingThePromptCache(t *testing.T) {
	type msg struct{ Role, Content string }
	var mu sync.Mutex
	var calls [][]msg
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []msg `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		calls = append(calls, req.Messages)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	t.Cleanup(srv.Close)
	f := setup(t, srv.URL)
	last := func() []msg {
		mu.Lock()
		defer mu.Unlock()
		return calls[len(calls)-1]
	}

	if err := f.st.SetTimezone(f.owner, "Not/AZone", true); err != store.ErrBadTimezone {
		t.Fatalf("a made-up zone was accepted: %v", err)
	}
	if err := f.st.SetTimezone(f.owner, "Asia/Dhaka", true); err != nil {
		t.Fatal(err)
	}
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	waitRun(t, f.st, mustSubmit(t, f, tid, f.owner, "first"))
	first := last()
	waitRun(t, f.st, mustSubmit(t, f, tid, f.owner, "what day is tomorrow?"))
	second := last()

	// system, user "first", assistant "ok", user question: no extra messages.
	if len(first) != 2 || len(second) != 4 {
		t.Fatalf("sent %d then %d messages: %+v", len(first), len(second), second)
	}
	sys := second[0].Content
	if !strings.Contains(sys, "the user's time zone (Asia/Dhaka)") {
		t.Errorf("the system prompt does not explain the time stamps: %q", sys)
	}
	if strings.Contains(sys, "Current date") || regexp.MustCompile(`\b20\d\d\b|\d\d:\d\d`).MatchString(sys) {
		t.Errorf("the system prompt contains a date or time, which would change it between turns: %q", sys)
	}
	for _, i := range []int{1, 3} {
		if second[i].Role != "user" || !stampRe.MatchString(second[i].Content) {
			t.Errorf("user message %d has no time stamp: %q", i, second[i].Content)
		}
	}
	if !strings.HasSuffix(second[3].Content, "]\nwhat day is tomorrow?") {
		t.Errorf("newest message = %q", second[3].Content)
	}
	want := timeStamp(time.Now(), f.st.MustLocation(f.owner))
	if got := second[3].Content[:len(want)]; got != want && got != timeStamp(time.Now().Add(-time.Minute), f.st.MustLocation(f.owner)) {
		t.Errorf("newest stamp = %q, want now (%q)", got, want)
	}
	// The cache test: everything the first turn sent is an unchanged prefix of the second.
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("message %d changed between turns:\n was %q\n now %q", i, first[i].Content, second[i].Content)
		}
	}

	// A group chat goes by the server's clock, whatever the members' zones are.
	bob, _ := f.st.CreateUser("bob", "x", "user")
	f.st.SetTimezone(bob, "America/Lima", true)
	tok, _ := f.st.CreateInvite(tid, f.owner, "member", time.Hour, 1)
	if _, err := f.st.AcceptInvite(tok, bob); err != nil {
		t.Fatal(err)
	}
	waitRun(t, f.st, mustSubmit(t, f, tid, bob, "hello all"))
	group := last()
	if sys := group[0].Content; !strings.Contains(sys, "the server's time zone (UTC") || strings.Contains(sys, "Asia/Dhaka") || strings.Contains(sys, "America/Lima") {
		t.Errorf("group system prompt = %q", sys)
	}
	newest := group[len(group)-1].Content
	if want := timeStamp(time.Now(), time.Local) + "\n[bob]: hello all"; newest != want &&
		newest != timeStamp(time.Now().Add(-time.Minute), time.Local)+"\n[bob]: hello all" {
		t.Errorf("group message = %q, want %q", newest, want)
	}

	// Agents start fresh each run: their prompt states the owner's date and time.
	a := newAgent(t, f, store.Agent{Name: "Helper", Instructions: "Help."})
	id, _ := f.eng.RunAgent(a, "go")
	waitRun(t, f.st, id)
	agent := last()
	if sys := agent[0].Content; !strings.Contains(sys, "Current date and time for alice: ") || !strings.Contains(sys, "(Asia/Dhaka, UTC+06:00)") {
		t.Errorf("agent prompt has no local time: %q", sys)
	}
	if stampRe.MatchString(agent[len(agent)-1].Content) {
		t.Errorf("an agent's message is stamped as well: %q", agent[len(agent)-1].Content)
	}
}
