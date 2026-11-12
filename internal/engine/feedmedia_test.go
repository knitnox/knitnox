package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"knitnox/internal/store"
)

// mediaProvider is a fake provider with the picture, voice and video
// endpoints, and a chat endpoint that plays the model looking at a picture.
type mediaProvider struct {
	*httptest.Server
	mu      sync.Mutex
	prompts []string // what the image endpoint was asked for, in order
	review  []string // answers of the picture check, one per call ("OK" when used up)
	polls   int
}

func newMediaProvider(t *testing.T) *mediaProvider {
	mp := &mediaProvider{}
	var pic bytes.Buffer
	png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	fail := func(w http.ResponseWriter, prompt string) bool {
		if strings.Contains(prompt, "fail") {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"content policy says no"}}`)
			return true
		}
		return false
	}
	body := func(r *http.Request) map[string]any {
		var m map[string]any
		json.NewDecoder(r.Body).Decode(&m)
		return m
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /images/generations", func(w http.ResponseWriter, r *http.Request) {
		prompt, _ := body(r)["prompt"].(string)
		if fail(w, prompt) {
			return
		}
		mp.mu.Lock()
		mp.prompts = append(mp.prompts, prompt)
		mp.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString(pic.Bytes())}}})
	})
	mux.HandleFunc("POST /audio/speech", func(w http.ResponseWriter, r *http.Request) {
		in := body(r)
		if text, _ := in["input"].(string); fail(w, text) {
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		fmt.Fprintf(w, "ID3 voice=%v", in["voice"])
	})
	mux.HandleFunc("POST /videos", func(w http.ResponseWriter, r *http.Request) {
		if prompt, _ := body(r)["prompt"].(string); fail(w, prompt) {
			return
		}
		fmt.Fprint(w, `{"id":"vid_1","status":"queued"}`)
	})
	mux.HandleFunc("GET /videos/{id}", func(w http.ResponseWriter, r *http.Request) {
		mp.mu.Lock()
		mp.polls++
		done := mp.polls >= 2
		mp.mu.Unlock()
		if done {
			fmt.Fprint(w, `{"id":"vid_1","status":"completed"}`)
		} else {
			fmt.Fprint(w, `{"id":"vid_1","status":"in_progress"}`)
		}
	})
	mux.HandleFunc("GET /videos/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		fmt.Fprint(w, "the film")
	})
	mux.HandleFunc("POST /chat/completions", func(w http.ResponseWriter, r *http.Request) {
		answer := "OK"
		mp.mu.Lock()
		if len(mp.review) > 0 {
			answer, mp.review = mp.review[0], mp.review[1:]
		}
		mp.mu.Unlock()
		b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": answer}}}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
	})
	mp.Server = httptest.NewServer(mux)
	t.Cleanup(mp.Close)
	return mp
}

// waitMedia waits until the media of a post is ready or has failed.
func waitMedia(t *testing.T, f *fixture, userID, postID int64) *store.FeedPost {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if p, err := f.st.FeedPostByID(userID, postID); err == nil && p.MediaStatus != store.MediaPending {
			return p
		}
	}
	t.Fatalf("the media of post %d was never finished", postID)
	return nil
}

// sharedProvider is the id of the fixture's shared provider.
func sharedProvider(t *testing.T, f *fixture) int64 {
	ps, err := f.st.ListProviders(nil)
	if err != nil || len(ps) == 0 {
		t.Fatal("no shared provider", err)
	}
	return ps[0].ID
}

// A post can ask for a picture, a recording or a video. It is saved at once
// and the file is made in the background by the model the owner set up.
func TestFeedMedia(t *testing.T) {
	mp := newMediaProvider(t)
	f := setup(t, mp.URL)
	f.eng.VideoPoll = 5 * time.Millisecond
	pid := sharedProvider(t, f)
	ai := store.FeedActor{Kind: store.FeedByAI}

	// Nothing is set up: the tool does not even offer media.
	ts := feedTools(f, f.owner, ai, 0, true)
	if schema := string(ts.defs[0].Function.Parameters); strings.Contains(schema, `"image"`) || strings.Contains(schema, `"voice"`) || strings.Contains(schema, `"video"`) {
		t.Fatalf("media is offered without any model: %s", schema)
	}
	if out := callTool(ts, "feed_post", `{"text":"A loaf.","image":"a loaf of bread"}`); !strings.Contains(out, "no model is set up to make a picture") {
		t.Fatalf("picture without a model = %q", out)
	}
	if f.eng.feedMediaNote(f.owner) != "" {
		t.Error("the prompt mentions media although there is no model")
	}

	for name, kind := range map[string]string{"img-1": store.ModelImage, "tts-1": store.ModelVoice, "vid-1": store.ModelVideo} {
		if err := f.st.CreateModelKind(pid, name, kind, 0, 0, 0, false); err != nil {
			t.Fatal(err)
		}
	}
	// Chat lists never show them.
	if opts, _ := f.st.ModelOptions(f.owner); len(opts) != 1 || opts[0].ModelName != "fake-model" {
		t.Fatalf("chat models = %+v", opts)
	}
	ts = feedTools(f, f.owner, ai, 0, true)
	schema := string(ts.defs[0].Function.Parameters)
	for _, want := range []string{`"image"`, `"voice"`, `"video"`} {
		if !strings.Contains(schema, want) {
			t.Errorf("the tool does not offer %s", want)
		}
	}
	if note := f.eng.feedMediaNote(f.owner); !strings.Contains(note, "`image`") || !strings.Contains(note, "`voice`") || !strings.Contains(note, "`video`") {
		t.Errorf("media note = %q", note)
	}
	if note := feedNote(false, 0, ""); !strings.Contains(note, "viewBox=\"0 0 640 360\"") || !strings.Contains(note, "font-size 16") {
		t.Errorf("the prompt does not explain how to draw a good svg: %q", note)
	}

	// A picture. The run's own model looks at it once: here it asks for a redraw.
	mp.review = []string{"A golden loaf of bread on a wooden table, seen from above"}
	ts2 := &toolset{routes: map[string]toolRoute{}, local: map[string]localTool{}}
	f.eng.addFeedTools(ts2, f.owner, ai, 0, true, time.UTC, f.modelID)
	out := callTool(ts2, "feed_post", `{"text":"Fresh bread today.","image":"bread"}`)
	if !strings.HasPrefix(out, "Posted as #1") || !strings.Contains(out, "The picture is being made now") {
		t.Fatalf("picture post = %q", out)
	}
	p := waitMedia(t, f, f.owner, 1)
	if p.MediaKind != store.MediaImage || p.MediaStatus != store.MediaReady || p.MediaMime != "image/png" || p.MediaW != 40 || p.MediaH != 30 ||
		!strings.Contains(p.MediaPrompt, "golden loaf") || !strings.Contains(p.MediaNote, "drawn again") {
		t.Fatalf("picture = %+v", p)
	}
	if data, err := os.ReadFile(f.st.FeedMediaPath(p)); err != nil || !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Fatalf("picture file: %v", err)
	}
	if len(mp.prompts) != 2 || mp.prompts[0] != "bread" || !strings.Contains(mp.prompts[1], "golden loaf") {
		t.Fatalf("the image model was asked: %q", mp.prompts)
	}
	// A picture that is fine is drawn once and carries no remark.
	callTool(ts2, "feed_post", `{"text":"A bicycle.","image":"a blue bicycle"}`)
	if p := waitMedia(t, f, f.owner, 2); p.MediaStatus != store.MediaReady || p.MediaNote != "" || len(mp.prompts) != 3 {
		t.Fatalf("second picture = %+v, asked %d times", p, len(mp.prompts))
	}

	// A recording, with the voice the owner chose.
	f.st.SaveFeedModels(f.owner, store.FeedModels{VoiceName: "nova"})
	callTool(ts, "feed_post", `{"text":"Good morning.","voice":"Good morning. Three tasks are due."}`)
	p = waitMedia(t, f, f.owner, 3)
	if p.MediaKind != store.MediaVoice || p.MediaStatus != store.MediaReady || p.MediaMime != "audio/mpeg" {
		t.Fatalf("recording = %+v", p)
	}
	if data, _ := os.ReadFile(f.st.FeedMediaPath(p)); string(data) != "ID3 voice=nova" {
		t.Fatalf("recording file = %q", data)
	}

	// A video: the job is asked about until it is done.
	callTool(ts, "feed_post", `{"text":"The counter.","video":"a slow pan over a bakery counter"}`)
	p = waitMedia(t, f, f.owner, 4)
	if p.MediaKind != store.MediaVideo || p.MediaStatus != store.MediaReady || p.MediaMime != "video/mp4" || mp.polls < 2 {
		t.Fatalf("video = %+v after %d looks", p, mp.polls)
	}

	// When the provider refuses, the post stays and says why, in the provider's own words.
	callTool(ts, "feed_post", `{"text":"This one fails.","image":"please fail"}`)
	p = waitMedia(t, f, f.owner, 5)
	if p.MediaStatus != store.MediaFailed || !strings.Contains(p.MediaNote, "content policy says no") || strings.Contains(p.MediaNote, "{") {
		t.Fatalf("failed picture = %+v", p)
	}

	// One piece of media per post.
	if out := callTool(ts, "feed_post", `{"text":"Both.","image":"a","voice":"b"}`); !strings.Contains(out, "not several") {
		t.Fatalf("two kinds at once = %q", out)
	}

	// Turned off by the owner: not offered, not made.
	f.st.SaveFeedModels(f.owner, store.FeedModels{Image: -1})
	if kinds := f.eng.FeedMediaKinds(f.owner); kinds[store.MediaImage] || !kinds[store.MediaVoice] {
		t.Fatalf("after turning pictures off: %v", kinds)
	}
	if _, err := f.eng.CreateFeedPost(f.owner, store.FeedPostInput{Body: "x", MediaKind: store.MediaImage, MediaPrompt: "y"}, ai, 0); err == nil {
		t.Fatal("a picture was made although pictures are turned off")
	}
}

// Media made with the server's models has a daily limit per person; a
// person's own model has none.
func TestFeedMediaDailyLimit(t *testing.T) {
	mp := newMediaProvider(t)
	f := setup(t, mp.URL)
	f.st.CreateModelKind(sharedProvider(t, f), "img-1", store.ModelImage, 0, 0, 0, false)
	st, _ := f.st.LoadSettings()
	st.FeedMediaDaily = 2
	f.st.SaveSettings(st)
	post := func() (int64, error) {
		return f.eng.CreateFeedPost(f.owner, store.FeedPostInput{Body: "A picture.", MediaKind: store.MediaImage, MediaPrompt: "a tree"}, store.FeedActor{}, 0)
	}
	for i := 0; i < 2; i++ {
		id, err := post()
		if err != nil {
			t.Fatal(err)
		}
		waitMedia(t, f, f.owner, id)
	}
	if _, err := post(); err == nil || !strings.Contains(err.Error(), "today's limit of 2") {
		t.Fatalf("the daily limit did not hold: %v", err)
	}
	// Another person has their own count.
	bob, _ := f.st.CreateUser("bob", "x", "user")
	if id, err := f.eng.CreateFeedPost(bob, store.FeedPostInput{Body: "Mine.", MediaKind: store.MediaImage, MediaPrompt: "a tree"}, store.FeedActor{}, 0); err != nil {
		t.Fatal(err)
	} else {
		waitMedia(t, f, bob, id)
	}
	// With a model of their own the limit does not apply, and the choice wins over the server's model.
	own, _ := f.st.CreateProvider(&f.owner, "mine", mp.URL, "key")
	f.st.CreateModelKind(own, "my-img", store.ModelImage, 0, 0, 0, false)
	f.st.SaveFeedModels(f.owner, store.FeedModels{Image: f.st.ModelIDByName(own, "my-img")})
	if m, p, _ := f.st.FeedMediaModel(f.owner, store.MediaImage); m == nil || m.Name != "my-img" || p.IsGlobal() {
		t.Fatalf("chosen model = %+v", m)
	}
	id, err := post()
	if err != nil {
		t.Fatalf("an own model was limited: %v", err)
	}
	waitMedia(t, f, f.owner, id)
	// Bob cannot be given alice's model: his feed falls back to the server's.
	f.st.SaveFeedModels(bob, store.FeedModels{Image: f.st.ModelIDByName(own, "my-img")})
	if m, _, _ := f.st.FeedMediaModel(bob, store.MediaImage); m == nil || m.Name != "img-1" {
		t.Fatalf("bob's model = %+v", m)
	}
}
