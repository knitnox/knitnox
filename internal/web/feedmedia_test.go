package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// The first screen of the Feed is the posts: writing a post, the settings and
// the explanations are one click away, not on the page.
func TestFeedFirstScreenIsThePosts(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "A first post."}, store.FeedActor{})
	page := alice.get("/feed")
	for _, want := range []string{`id="add" method="post" action="/feed" data-reset hidden`, `data-toggle="#add"`, "A first post.", `href="/feed/settings"`, ">Latest<", ">Mine<"} {
		if !strings.Contains(page, want) {
			t.Errorf("the feed is missing %q", want)
		}
	}
	for _, not := range []string{"Who may post to my feed", "How the Feed works</h2>", "Your feed is empty", "Add my own model"} {
		if strings.Contains(page, not) {
			t.Errorf("the feed's first screen still shows %q", not)
		}
	}
	settings := alice.get("/feed/settings")
	for _, want := range []string{"Who may post to my feed", "Pictures, voice and video", "How the Feed works", "Add my own model", `action="/feed/settings/models"`, "The server's model (none is set up yet)", `href="/feed"`} {
		if !strings.Contains(settings, want) {
			t.Errorf("feed settings is missing %q", want)
		}
	}
}

// Each model has a purpose. Picture, voice and video models are chosen on the
// Feed settings page; a user can add one of their own right there.
func TestFeedModelsAndKinds(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice") // admin
	bob := a.register("bob")

	// The admin shares a chat model and a picture model; the kind is guessed from the name and can be changed.
	alice.post("/admin/providers", url.Values{"name": {"Shared"}, "base_url": {"http://127.0.0.1:9/v1"}, "models": {"gpt-test, dall-e-test, speaker-1"}})
	shared, _ := a.st.ListProviders(nil)
	pid := shared[0].ID
	kinds := map[string]string{}
	for _, m := range shared[0].Models {
		kinds[m.Name] = m.Kind
	}
	if kinds["gpt-test"] != store.ModelChat || kinds["dall-e-test"] != store.ModelImage || kinds["speaker-1"] != store.ModelChat {
		t.Fatalf("guessed kinds = %v", kinds)
	}
	speaker := a.st.ModelIDByName(pid, "speaker-1")
	if code, _ := alice.hx(fmt.Sprintf("/admin/models/%d/kind", speaker), url.Values{"kind": {"voice"}}); code != http.StatusOK {
		t.Fatalf("change kind: %d", code)
	}
	if code, _ := alice.hx(fmt.Sprintf("/admin/models/%d/kind", speaker), url.Values{"kind": {"hologram"}}); code != http.StatusBadRequest {
		t.Errorf("an unknown kind was accepted: %d", code)
	}
	if code, _ := bob.hx(fmt.Sprintf("/admin/models/%d/kind", speaker), url.Values{"kind": {"chat"}}); code != http.StatusForbidden {
		t.Errorf("a member changed a shared model: %d", code)
	}
	if code, _ := bob.hx(fmt.Sprintf("/settings/models/%d/kind", speaker), url.Values{"kind": {"chat"}}); code != http.StatusNotFound {
		t.Errorf("a member changed a shared model through their own settings: %d", code)
	}
	if m, _ := a.st.ModelByID(speaker); m.Kind != store.ModelVoice {
		t.Fatalf("speaker-1 is now %q", m.Kind)
	}
	alice.hx(fmt.Sprintf("/admin/providers/%d/models", pid), url.Values{"name": {"clip-maker"}, "kind": {"video"}})
	if m, _ := a.st.ModelByID(a.st.ModelIDByName(pid, "clip-maker")); m == nil || m.Kind != store.ModelVideo || m.Vision {
		t.Fatalf("model added with a kind = %+v", m)
	}
	// Only the chat model can be picked for a chat.
	if home := bob.get("/"); !strings.Contains(home, "gpt-test") || strings.Contains(home, "dall-e-test") || strings.Contains(home, "speaker-1") || strings.Contains(home, "clip-maker") {
		t.Fatal("the chat's model list is not exactly the chat models")
	}
	if p := alice.get("/admin/providers"); !strings.Contains(p, `hx-post="/admin/models/`+fmt.Sprint(speaker)+`/kind"`) || !strings.Contains(p, ">Pictures<") {
		t.Error("the providers page does not show what each model is for")
	}

	// Bob sees the shared models as the server's choice.
	page := bob.get("/feed/settings")
	for _, want := range []string{"The server's model: dall-e-test", "The server's model: speaker-1", "The server's model: clip-maker", ">Ready<"} {
		if !strings.Contains(page, want) {
			t.Errorf("bob's feed settings is missing %q", want)
		}
	}
	if !strings.Contains(bob.get("/feed"), `name="media" value="image"`) {
		t.Error("the composer does not offer a picture although a picture model is set up")
	}

	// Bob adds a picture model of his own, right there. It is chosen, and kept with his other keys.
	code, _, h := bob.post("/feed/settings/models/add", url.Values{"kind": {"image"}, "model": {"my-painter"}, "base_url": {"https://api.example.com/v1/"}, "api_key": {"sk-bob"}})
	if code != http.StatusSeeOther || h.Get("Location") != "/feed/settings?done=added#models" {
		t.Fatalf("add own model: %d %s", code, h.Get("Location"))
	}
	own, _ := a.st.ListProviders(&bob.id)
	if len(own) != 1 || own[0].Name != "api.example.com" || own[0].BaseURL != "https://api.example.com/v1" || own[0].APIKey != "sk-bob" ||
		len(own[0].Models) != 1 || own[0].Models[0].Kind != store.ModelImage {
		t.Fatalf("bob's providers = %+v", own)
	}
	mine := own[0].Models[0].ID
	if fm := a.st.FeedModelsFor(bob.id); fm.Image != mine {
		t.Fatalf("the new model was not chosen: %+v", fm)
	}
	if p := bob.get("/settings"); !strings.Contains(p, "my-painter") || !strings.Contains(p, "api.example.com") {
		t.Error("the model added in Feed settings is not listed under Settings")
	}
	if p := bob.get("/feed/settings"); !strings.Contains(p, "my-painter — api.example.com (your own)") || !strings.Contains(p, `data-quick-provider`) {
		t.Error("feed settings does not offer bob's own model and provider")
	}
	// A second model on the provider he already has: no address or key again.
	bob.post("/feed/settings/models/add", url.Values{"kind": {"voice"}, "model": {"my-voice"}, "provider_id": {fmt.Sprint(own[0].ID)}})
	if own, _ = a.st.ListProviders(&bob.id); len(own) != 1 || len(own[0].Models) != 2 {
		t.Fatalf("after the second model: %+v", own)
	}
	for name, form := range map[string]url.Values{
		"no name":            {"kind": {"image"}, "base_url": {"https://x.example/v1"}},
		"no address":         {"kind": {"image"}, "model": {"m"}},
		"bad kind":           {"kind": {"chat"}, "model": {"m"}, "base_url": {"https://x.example/v1"}},
		"someone's provider": {"kind": {"image"}, "model": {"m"}, "provider_id": {fmt.Sprint(pid)}},
	} {
		if code, _, _ := bob.post("/feed/settings/models/add", form); code != http.StatusBadRequest {
			t.Errorf("%s was accepted: %d", name, code)
		}
	}

	// Saving the choices: off, the server's, own. Alice cannot choose bob's model.
	bob.post("/feed/settings/models", url.Values{"image_model_id": {fmt.Sprint(mine)}, "voice_model_id": {"0"}, "video_model_id": {"-1"}, "voice_name": {"nova"}})
	if fm := a.st.FeedModelsFor(bob.id); fm.Image != mine || fm.Voice != 0 || fm.Video != -1 || fm.VoiceName != "nova" {
		t.Fatalf("saved choices = %+v", fm)
	}
	if !strings.Contains(bob.get("/feed/settings"), "Off: never make videos") || strings.Contains(bob.get("/feed"), `name="media" value="video"`) {
		t.Error("a kind that is turned off is still offered")
	}
	alice.post("/feed/settings/models", url.Values{"image_model_id": {fmt.Sprint(mine)}})
	if fm := a.st.FeedModelsFor(alice.id); fm.Image != 0 {
		t.Fatalf("alice chose bob's model: %+v", fm)
	}

	// Asking for a picture without any model says what to do, and posts nothing.
	carol := a.register("carol")
	a.st.SaveFeedModels(carol.id, store.FeedModels{Image: -1})
	if code, msg := carol.hx("/feed", url.Values{"body": {"x"}, "media": {"image"}, "media_prompt": {"a tree"}}); code != http.StatusBadRequest || !strings.Contains(msg, "Feed settings") {
		t.Fatalf("picture without a model: %d %s", code, msg)
	}
	if n := a.st.CountFeed(carol.id, store.FeedFilter{View: store.FeedViewMine}); n != 0 {
		t.Errorf("a post was saved although its picture was refused: %d", n)
	}
}

// The Memory settings have the same small form: an embedding model of your own, added and used at once.
func TestMemoryOwnEmbeddingModelInPlace(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	if p := alice.get("/memory/settings"); !strings.Contains(p, "Add my own embedding model") || !strings.Contains(p, `action="/memory/settings/model/add"`) {
		t.Fatal("memory settings has no place to add an embedding model")
	}
	code, _, h := alice.post("/memory/settings/model/add", url.Values{"model": {"text-embedding-test"}, "base_url": {"https://embed.example.com/v1"}, "api_key": {"sk-a"}})
	if code != http.StatusSeeOther || h.Get("Location") != "/memory/settings?done=search" {
		t.Fatalf("add: %d %s", code, h.Get("Location"))
	}
	own, _ := a.st.ListProviders(&alice.id)
	if len(own) != 1 || len(own[0].Models) != 1 || own[0].Models[0].Kind != store.ModelEmbedding {
		t.Fatalf("providers = %+v", own)
	}
	if ms, _ := a.st.MemorySettingsFor(alice.id); ms.EmbedProviderID != own[0].ID || ms.EmbedModel != "text-embedding-test" {
		t.Fatalf("memory settings = %+v", ms)
	}
	// It is an embedding model, so no chat offers it.
	if opts, _ := a.st.ModelOptions(alice.id); len(opts) != 0 {
		t.Fatalf("an embedding model is offered for chats: %+v", opts)
	}
	if code, _, _ := alice.post("/memory/settings/model/add", url.Values{"model": {""}, "base_url": {"https://embed.example.com/v1"}}); code != http.StatusBadRequest {
		t.Errorf("a model without a name was accepted: %d", code)
	}
}

// A picture, recording or video is served only to those who may see its post,
// and only as a file that can be shown or played.
func TestFeedMediaFile(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	id, _ := a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "With a picture.", MediaKind: store.MediaImage, MediaPrompt: "a tree"}, store.FeedActor{})
	item := fmt.Sprintf("/feed/p/%d/media", id)

	if p := alice.get("/feed"); !strings.Contains(p, "Making the picture…") || strings.Contains(p, `src="`+item+`"`) {
		t.Fatal("a picture that is still being made is not shown as such")
	}
	if code := alice.getStatus(item); code != http.StatusNotFound {
		t.Errorf("a picture that is not ready was served: %d", code)
	}
	if err := a.st.SaveFeedMedia(id, []byte("\x89PNG fake"), "image/png", ".png", "a tree", "", 800, 400); err != nil {
		t.Fatal(err)
	}
	// Shown at its own size, made smaller to fit, never larger.
	if p := alice.get("/feed"); !strings.Contains(p, `<img src="`+item+`" alt="a tree" width="520" height="260"`) || !strings.Contains(p, "Made by AI") {
		t.Fatal("the finished picture is not on the post at a fitting size")
	}
	req, _ := http.NewRequest(http.MethodGet, a.srv.URL+item, nil)
	r, err := alice.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusOK || r.Header.Get("Content-Type") != "image/png" || !strings.Contains(r.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("picture: %d %q %q", r.StatusCode, r.Header.Get("Content-Type"), r.Header.Get("Content-Security-Policy"))
	}
	if code := bob.getStatus(item); code != http.StatusNotFound {
		t.Errorf("bob can load the picture of a private post: %d", code)
	}
	alice.hx(fmt.Sprintf("/feed/p/%d/share", id), url.Values{"public": {"1"}})
	if code := bob.getStatus(item); code != http.StatusOK {
		t.Errorf("bob cannot load the picture of a shared post: %d", code)
	}

	// What went wrong is told to the owner only.
	bad, _ := a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "Failed one.", MediaKind: store.MediaVoice, MediaPrompt: "hello", Public: true}, store.FeedActor{})
	a.st.FailFeedMedia(bad, "the provider said (401): wrong key sk-secret")
	if p := alice.get("/feed"); !strings.Contains(p, "The recording could not be made: the provider said (401)") {
		t.Error("the owner is not told why the recording failed")
	}
	if p := bob.get("/feed"); !strings.Contains(p, "Failed one.") || strings.Contains(p, "sk-secret") || strings.Contains(p, "could not be made") {
		t.Error("someone else is shown why alice's recording failed")
	}
}

// A drawn picture keeps its own size on the page: it is fitted, never blown up.
func TestFeedDrawnPictureSize(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	small := `<svg width="200" height="100"><rect width="200" height="100" fill="red"/></svg>`
	wide := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1280 720"><rect width="1280" height="720"/></svg>`
	vague := `<svg xmlns="http://www.w3.org/2000/svg" width="100%"><rect width="10" height="10"/></svg>`
	for _, svg := range []string{small, wide, vague} {
		if _, err := a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "Drawn.", SVG: svg}, store.FeedActor{Kind: store.FeedByAI}); err != nil {
			t.Fatal(err)
		}
	}
	// Without a viewBox the picture could not scale as one piece: it gets one from its width and height.
	if svg, _ := a.st.FeedSVG(alice.id, 1, false); !strings.Contains(svg, `viewBox="0 0 200 100"`) || !strings.Contains(svg, `width="200"`) {
		t.Fatalf("stored small picture = %s", svg)
	}
	page := alice.get("/feed")
	for _, want := range []string{
		`<img src="/feed/p/1/picture.svg" alt="Picture drawn for this post" width="200" height="100"`, // small stays small
		`<img src="/feed/p/2/picture.svg" alt="Picture drawn for this post" width="640" height="360"`, // large is fitted
		`<img src="/feed/p/3/picture.svg" alt="Picture drawn for this post" loading="lazy"`,           // unknown size: the style sheet caps it
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page is missing %s", want)
		}
	}
}
