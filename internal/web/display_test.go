package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// Settings → Appearance: tool calls are one quiet line per answer unless the
// person asks for the details, and the "Thinking" block can be switched off.
// What must never be hidden stays: a tool waiting for permission, a failure,
// and a sign that a tool is running.
func TestDisplaySwitches(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	tid, _ := a.st.CreateThread(alice.id, 0)
	path := fmt.Sprintf("/t/%d", tid)
	call := func(id, name string) string {
		return `[{"id":"` + id + `","type":"function","function":{"name":"` + name + `","arguments":"{\"q\":\"now\"}"}}]`
	}

	// A finished turn: one tool call, its answer, then the reply with some thinking.
	run1, _ := a.st.EnqueueMessage(tid, alice.id, "what time is it?", 10, nil)
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run1, Role: "assistant", ToolCalls: call("c1", "s3_clock")})
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run1, Role: "tool", Content: "TOOLANSWER 12:00", ToolCallID: "c1", ToolName: "s3_clock"})
	final, _ := a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run1, Role: "assistant"})
	a.st.UpdateMessageFull(final, "It is noon.", "MYREASONING", "", store.MsgDone)
	a.st.FinishRun(run1, store.RunDone, "", 2)

	// The default: thinking shown, tools tucked into one line that still holds the details.
	page := alice.get(path)
	for _, want := range []string{"Used 1 tool<", "TOOLANSWER 12:00", "MYREASONING", `data-thinking="1"`, "It is noon.", ">clock<"} {
		if !strings.Contains(page, want) {
			t.Errorf("by default the chat lacks %q", want)
		}
	}
	if strings.Contains(page, "Result from") || strings.Contains(page, "Used <b") {
		t.Error("by default tool calls are still rows of their own")
	}
	if n := strings.Count(page, `class="msg msg-ai`); n != 1 {
		t.Errorf("%d AI messages drawn, want 1: the step that only called a tool should not leave an empty row", n)
	}

	// A tool that failed is said on the line itself.
	run2, _ := a.st.EnqueueMessage(tid, alice.id, "and tomorrow?", 10, nil)
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run2, Role: "assistant", ToolCalls: call("c2", "s3_clock")})
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run2, Role: "tool", Content: "Error: tool \"clock\" could not be reached", ToolCallID: "c2", ToolName: "s3_clock"})
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run2, Role: "assistant", Content: "I could not check."})
	a.st.FinishRun(run2, store.RunDone, "", 2)
	if page := alice.get(path); !strings.Contains(page, "1 did not work") {
		t.Error("a failed tool is not mentioned on the line")
	}

	// A run that is waiting for a tool says so, and one waiting for permission shows the card.
	run3, _ := a.st.EnqueueMessage(tid, alice.id, "check again", 10, nil)
	a.st.StartRun(run3)
	asking, _ := a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run3, Role: "assistant", ToolCalls: call("c3", "s3_clock")})
	if page := alice.get(path); !strings.Contains(page, "is-working") || !strings.Contains(page, "Using <b") {
		t.Error("nothing shows that a tool is running")
	}
	a.st.CreateApproval(&store.ToolApproval{ThreadID: tid, RunID: run3, MessageID: asking, ToolCallID: "c3", ServerID: 3, ServerName: "Clock", ToolName: "clock", Args: "{}"})
	if page := alice.get(path); !strings.Contains(page, "The AI wants to run") || !strings.Contains(page, "Allow once") {
		t.Fatal("a tool waiting for permission is hidden when tool details are off")
	}
	a.st.ExpirePendingApprovals()
	a.st.FinishRun(run3, store.RunCancelled, "", 1)

	// The switches.
	set := func(c *client, key, on string) (int, string) {
		code, body, _ := c.hxPost("/settings/display", url.Values{"key": {key}, "on": {on}})
		return code, body
	}
	if code, _ := set(alice, "everything", "1"); code != http.StatusBadRequest {
		t.Fatalf("an unknown switch: %d", code)
	}
	if code, body := set(alice, "tools", "1"); code != 200 || !strings.Contains(body, `aria-label="Show tool details"`) || !strings.Contains(body, "row of its own") {
		t.Fatalf("tool details on: %d", code)
	}
	page = alice.get(path)
	if !strings.Contains(page, "Result from") || strings.Contains(page, "Used 1 tool<") || !strings.Contains(page, "MYREASONING") {
		t.Error("with tool details on the chat should show each call and result, and still the thinking")
	}
	if code, _ := set(alice, "thinking", "0"); code != 200 {
		t.Fatalf("thinking off: %d", code)
	}
	page = alice.get(path)
	if strings.Contains(page, "MYREASONING") || !strings.Contains(page, `data-thinking="0"`) || !strings.Contains(page, "Result from") {
		t.Error("with thinking off the block is still there, or the tool rows went with it")
	}
	if u, _ := a.st.UserByID(alice.id); u.ShowThinking || !u.ShowTools {
		t.Fatalf("stored for alice: thinking %v, tools %v", u.ShowThinking, u.ShowTools)
	}
	// One person's choice is theirs alone.
	if u, _ := a.st.UserByID(bob.id); !u.ShowThinking || u.ShowTools {
		t.Fatalf("bob's settings changed with alice's: thinking %v, tools %v", u.ShowThinking, u.ShowTools)
	}
	if s := alice.get("/settings"); !strings.Contains(s, "Show thinking") || !strings.Contains(s, "Show tool details") {
		t.Error("the settings page lacks the switches")
	}

	// A read-only link is the owner's decision, not the reader's: it does not follow these switches.
	sh, _ := a.st.SaveShare(tid, false)
	if _, shared, _ := a.anonGet("/s/" + sh.Token); strings.Contains(shared, "TOOLANSWER") || strings.Contains(shared, "MYREASONING") {
		t.Error("a read-only link shows tool answers or thinking")
	}
}
