package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// voiceProvider is a fake provider that can only read text aloud; it counts its calls.
func voiceProvider(t *testing.T) (*httptest.Server, *int) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/speech" {
			http.NotFound(w, r)
			return
		}
		calls++
		w.Header().Set("Content-Type", "audio/mpeg")
		fmt.Fprint(w, "ID3 the recording")
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// Reading an answer aloud: offered only with a voice model, only for AI
// answers, only to people in the chat; made once and then served as audio.
func TestReadAloudOverHTTP(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice") // admin
	bob := a.register("bob")
	carol := a.register("carol")

	tid, _ := a.st.CreateGroup(alice.id, 0, "Bakers") // a group, so that someone else can listen too
	runID, _ := a.st.EnqueueMessage(tid, alice.id, "how do I bake bread?", 10, nil)
	aiID, _ := a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: runID, Role: "assistant", Content: "Mix flour and water."})
	a.st.FinishRun(runID, store.RunDone, "", 1)
	msgs, _ := a.st.RunMessages(runID)
	userMsgID := msgs[0].ID
	speak := func(c *client, mid int64) (int, string) {
		code, body, _ := c.post(fmt.Sprintf("/t/%d/messages/%d/speak", tid, mid), url.Values{})
		return code, body
	}
	page := func(c *client) string { return c.get(fmt.Sprintf("/t/%d/messages", tid)) }

	// No voice model: no button, and asking anyway explains what to do.
	if strings.Contains(page(alice), "data-speak") {
		t.Fatal("Read aloud is offered without a voice model")
	}
	if code, body := speak(alice, aiID); code != http.StatusBadRequest || !strings.Contains(body, "No voice model") {
		t.Fatalf("without a model: %d %q", code, body)
	}

	srv, calls := voiceProvider(t)
	pid, _ := a.st.CreateProvider(nil, "voice", srv.URL, "k")
	if err := a.st.CreateModelKind(pid, "tts-1", store.ModelVoice, 0, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if html := page(alice); strings.Count(html, "data-speak") != 1 || strings.Contains(html, "data-audio") {
		t.Fatalf("with a model: %d buttons, kept recording: %v", strings.Count(html, "data-speak"), strings.Contains(html, "data-audio"))
	}

	// Only people in the chat, and only the AI's answers.
	if code, _ := speak(bob, aiID); code != http.StatusNotFound {
		t.Fatalf("someone outside the chat: %d", code)
	}
	if code, _ := speak(alice, userMsgID); code != http.StatusNotFound {
		t.Fatalf("a person's own message: %d", code)
	}
	if *calls != 0 {
		t.Fatalf("the voice model was called %d times for refused requests", *calls)
	}

	code, body := speak(alice, aiID)
	var out struct{ URL string }
	if code != 200 || json.Unmarshal([]byte(body), &out) != nil || !strings.HasPrefix(out.URL, "/files/") {
		t.Fatalf("speak: %d %q", code, body)
	}
	r, err := alice.http.Get(a.srv.URL + out.URL)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 200 || r.Header.Get("Content-Type") != "audio/mpeg" || !strings.HasPrefix(r.Header.Get("Content-Disposition"), "inline") {
		t.Fatalf("the recording: %d %s %s", r.StatusCode, r.Header.Get("Content-Type"), r.Header.Get("Content-Disposition"))
	}
	if bob.getStatus(out.URL) != http.StatusNotFound {
		t.Fatal("someone outside the chat can load the recording")
	}

	// Asked again, by the same person or by a member: the kept recording, no new call.
	inv, _ := a.st.CreateInvite(tid, alice.id, "viewer", 24*3600*1e9, 5)
	if _, err := a.st.AcceptInvite(inv, carol.id); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*client{alice, carol} {
		if code, body := speak(c, aiID); code != 200 || !strings.Contains(body, out.URL) {
			t.Fatalf("second listen: %d %q", code, body)
		}
	}
	if *calls != 1 {
		t.Fatalf("the voice model was called %d times, want 1", *calls)
	}
	if html := page(carol); !strings.Contains(html, `data-audio="`+out.URL+`"`) {
		t.Fatal("the page does not carry the kept recording")
	}
}

// A picture the AI made is shown on the AI's message and opens like an upload.
func TestMadePictureIsShownInTheChat(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	tid, _ := a.st.CreateThread(alice.id, 0)
	runID, _ := a.st.EnqueueMessage(tid, alice.id, "paint a cat", 10, nil)
	aiID, _ := a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: runID, Role: "assistant", ToolCalls: `[{"id":"c1","type":"function","function":{"name":"make_picture","arguments":"{}"}}]`})
	a.st.FinishRun(runID, store.RunDone, "", 1)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	att, err := a.st.SaveUpload(png, "image/png", "Picture: a cat", store.KindImage, "")
	if err != nil {
		t.Fatal(err)
	}
	id, err := a.st.AddMadeAttachment(tid, aiID, alice.id, att, true)
	if err != nil {
		t.Fatal(err)
	}
	html := alice.get(fmt.Sprintf("/t/%d/messages", tid))
	if !strings.Contains(html, fmt.Sprintf(`data-lightbox="/files/%d"`, id)) || !strings.Contains(html, "msg-images made") {
		t.Fatal("the made picture is not on the page")
	}
	if alice.getStatus(fmt.Sprintf("/files/%d", id)) != 200 {
		t.Fatal("the made picture cannot be loaded")
	}
}
