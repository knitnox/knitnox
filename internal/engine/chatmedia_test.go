package engine

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// aiMessage puts a finished turn into a chat and returns the AI's message.
func aiMessage(t *testing.T, f *fixture, threadID int64, answer string) *store.Message {
	t.Helper()
	runID, err := f.st.EnqueueMessage(threadID, f.owner, "question", 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := f.st.CreateMessage(&store.Message{ThreadID: threadID, RunID: runID, Role: "assistant", Content: answer})
	if err != nil {
		t.Fatal(err)
	}
	f.st.FinishRun(runID, store.RunDone, "", 1)
	m, err := f.st.ThreadMessage(threadID, id)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func pictureTool(f *fixture, tc *turnCtx) (*toolset, bool) {
	ts := &toolset{routes: map[string]toolRoute{}, local: map[string]localTool{}}
	return ts, f.eng.addPictureTool(ts, tc)
}

// The AI in a person's own chat can make a picture. It is kept with the
// chat like an upload, counts toward the daily limit when the server's model
// made it, and a refusal by the provider reaches the model in plain words.
func TestMakePictureInChat(t *testing.T) {
	mp := newMediaProvider(t)
	f := setup(t, mp.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	thread, _ := f.st.ThreadByID(tid)
	msg := aiMessage(t, f, tid, "")
	tc := &turnCtx{thread: thread, callMsgID: msg.ID}

	if _, ok := pictureTool(f, tc); ok {
		t.Fatal("the tool is offered although no picture model is set up")
	}
	if pic, voice := f.eng.CanMakeInChat(f.owner); pic || voice {
		t.Fatal("chat media is offered without any model")
	}
	f.st.CreateModelKind(sharedProvider(t, f), "img-1", store.ModelImage, 0, 0, 0, false)
	ts, ok := pictureTool(f, tc)
	if !ok {
		t.Fatal("the tool is not offered with a picture model")
	}

	out := callTool(ts, "make_picture", `{"description":"a loaf of bread on a table"}`)
	if !strings.Contains(out, "is now shown to the user") {
		t.Fatalf("answer = %q", out)
	}
	atts, _ := f.st.AttachmentsByMessage([]int64{msg.ID})
	if len(atts[msg.ID]) != 1 || !atts[msg.ID][0].IsImage() || atts[msg.ID][0].Mime != "image/png" {
		t.Fatalf("attachments on the AI's message: %+v", atts[msg.ID])
	}
	if _, err := os.Stat(f.st.AttachmentPath(&atts[msg.ID][0])); err != nil {
		t.Fatalf("the picture file: %v", err)
	}
	if mp.prompts[len(mp.prompts)-1] != "a loaf of bread on a table" {
		t.Fatalf("the image model was asked for %q", mp.prompts)
	}
	if n := f.st.MadeMediaSince(f.owner, "2000-01-01 00:00:00"); n != 1 {
		t.Fatalf("%d made with the server's model, want 1", n)
	}
	// The picture is not a file the hourly sweep may take for an orphan.
	if n, _ := f.st.SweepOrphanUploads(); n != 0 {
		t.Fatalf("the sweep removed %d files", n)
	}

	// The provider says no: the model is told why, nothing is stored.
	if out := callTool(ts, "make_picture", `{"description":"please fail"}`); !strings.Contains(out, "content policy says no") {
		t.Fatalf("refusal = %q", out)
	}
	if out := callTool(ts, "make_picture", `{"description":"  "}`); !strings.HasPrefix(out, "Error:") {
		t.Fatalf("empty description = %q", out)
	}
	if atts, _ := f.st.AttachmentsByMessage([]int64{msg.ID}); len(atts[msg.ID]) != 1 {
		t.Fatalf("%d attachments after two failures", len(atts[msg.ID]))
	}

	// The daily limit covers chat pictures and feed media together.
	saveSettings(t, f.st, func(s *store.Settings) { s.FeedMediaDaily = 1 })
	if out := callTool(ts, "make_picture", `{"description":"one more"}`); !strings.Contains(out, "limit of 1") {
		t.Fatalf("over the limit = %q", out)
	}
	if _, err := f.eng.CreateFeedPost(f.owner, store.FeedPostInput{Body: "x", MediaKind: store.MediaImage, MediaPrompt: "y"}, store.FeedActor{Kind: store.FeedByUser}, 0); err == nil || !strings.Contains(err.Error(), "limit of 1") {
		t.Fatalf("a feed picture after the chat used up the limit: %v", err)
	}
}

// Deleting the chat takes the picture's row with it, and the sweep its file.
func TestMadePictureGoesWithTheChat(t *testing.T) {
	mp := newMediaProvider(t)
	f := setup(t, mp.URL)
	f.st.CreateModelKind(sharedProvider(t, f), "img-1", store.ModelImage, 0, 0, 0, false)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	thread, _ := f.st.ThreadByID(tid)
	msg := aiMessage(t, f, tid, "")
	ts, _ := pictureTool(f, &turnCtx{thread: thread, callMsgID: msg.ID})
	callTool(ts, "make_picture", `{"description":"a cat"}`)

	files := f.st.ThreadUploadFiles(tid)
	if len(files) != 1 {
		t.Fatalf("files of the chat: %v", files)
	}
	if err := f.st.DeleteThread(tid); err != nil {
		t.Fatal(err)
	}
	f.st.RemoveFiles(files)
	if atts, _ := f.st.AttachmentsByMessage([]int64{msg.ID}); len(atts[msg.ID]) != 0 {
		t.Fatal("the picture's row outlived the chat")
	}
	entries, _ := os.ReadDir(f.st.UploadDir)
	if len(entries) != 0 {
		t.Fatalf("%d files left in uploads", len(entries))
	}
}

// An answer is read aloud once; later listeners get the same recording.
func TestReadAloud(t *testing.T) {
	mp := newMediaProvider(t)
	f := setup(t, mp.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	msg := aiMessage(t, f, tid, "# Bread\n\nMix **flour** and water. See [the guide](https://example.com/x).\n\n```go\nfmt.Println(1)\n```")
	ctx := context.Background()

	if _, err := f.eng.SpeakMessage(ctx, f.owner, msg); !errors.Is(err, store.ErrNoMediaModel) {
		t.Fatalf("without a voice model: %v", err)
	}
	f.st.CreateModelKind(sharedProvider(t, f), "tts-1", store.ModelVoice, 0, 0, 0, false)
	if _, voice := f.eng.CanMakeInChat(f.owner); !voice {
		t.Fatal("read aloud is not offered with a voice model")
	}
	a, err := f.eng.SpeakMessage(ctx, f.owner, msg)
	if err != nil || !a.IsAudio() || a.Mime != "audio/mpeg" || a.MessageID != msg.ID {
		t.Fatalf("recording = %+v, %v", a, err)
	}
	saveSettings(t, f.st, func(s *store.Settings) { s.FeedMediaDaily = 1 }) // used up by the recording itself
	again, err := f.eng.SpeakMessage(ctx, f.owner, msg)
	if err != nil || again.ID != a.ID {
		t.Fatalf("second listen = %+v, %v (a kept recording must not be made or counted again)", again, err)
	}
	other := aiMessage(t, f, tid, "Another answer.")
	if _, err := f.eng.SpeakMessage(ctx, f.owner, other); err == nil || !strings.Contains(err.Error(), "limit of 1") {
		t.Fatalf("over the limit: %v", err)
	}
	saveSettings(t, f.st, func(s *store.Settings) { s.FeedMediaDaily = 0 })
	if _, err := f.eng.SpeakMessage(ctx, f.owner, aiMessage(t, f, tid, "please fail")); err == nil || !strings.Contains(err.Error(), "content policy says no") {
		t.Fatalf("a refusal by the provider: %v", err)
	}
}

func TestSpeakable(t *testing.T) {
	got := speakable("# Bread\n\nMix **flour** and _water_. See [the guide](https://example.com/x) and ![a loaf](https://example.com/l.png).\n\n```go\nfmt.Println(1)\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n---\nDone.")
	for _, bad := range []string{"#", "*", "_", "`", "https://", "fmt.Println", "|", "---"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q is still in %q", bad, got)
		}
	}
	for _, want := range []string{"Bread", "Mix flour and water.", "the guide", "a loaf", "a code example is left out", "Done."} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing from %q", want, got)
		}
	}
	long := speakable(strings.Repeat("A sentence that is read. ", 400))
	if n := len([]rune(long)); n > speakMaxChars+60 || !strings.HasSuffix(long, "is not read aloud.") {
		t.Errorf("long text: %d characters, ends %q", n, long[len(long)-40:])
	}
	if speakable("```\nonly code\n```") == "" {
		t.Error("an answer of only code should still say that code was left out")
	}
}
