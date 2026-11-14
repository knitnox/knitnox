package web

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// postWithPicture sends the "New post" form with a file in its picture field.
func (c *client) postWithPicture(fields map[string]string, name string, data []byte) (int, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if name != "" {
		fw, _ := mw.CreateFormFile("picture", name)
		fw.Write(data)
	}
	mw.Close()
	req, _ := http.NewRequest(http.MethodPost, c.a.srv.URL+"/feed", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.Header.Set("HX-Request", "true")
	r, err := c.http.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b)
}

func newest(t *testing.T, a *app, userID int64) store.FeedPost {
	t.Helper()
	posts, _, err := a.st.ListFeed(userID, store.FeedFilter{View: store.FeedViewMine, Limit: 1})
	if err != nil || len(posts) == 0 {
		t.Fatalf("no post: %v", err)
	}
	return posts[0]
}

// A person adds a photo or a drawing of their own to a post, with or without text.
func TestFeedOwnPictures(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")

	var shot bytes.Buffer
	png.Encode(&shot, image.NewRGBA(image.Rect(0, 0, 40, 30)))

	// A photo with text.
	code, card := alice.postWithPicture(map[string]string{"body": "The new sign.", "media": "image", "media_prompt": "ignored"}, "sign.png", shot.Bytes())
	p := newest(t, a, alice.id)
	if code != http.StatusOK || p.MediaKind != store.MediaImage || p.MediaStatus != store.MediaReady || !p.MediaOwn || p.MediaW != 40 || p.MediaH != 30 {
		t.Fatalf("photo post: %d %+v", code, p)
	}
	if !strings.Contains(card, fmt.Sprintf(`src="/feed/p/%d/media"`, p.ID)) || strings.Contains(card, "Made by AI") || !strings.Contains(card, `width="40" height="30"`) {
		t.Errorf("the card of a photo post: %s", card)
	}
	if _, err := os.Stat(a.st.FeedMediaPath(&p)); err != nil {
		t.Fatalf("the photo is not on disk: %v", err)
	}
	if code := alice.getStatus(fmt.Sprintf("/feed/p/%d/media", p.ID)); code != http.StatusOK {
		t.Errorf("the owner cannot load the photo: %d", code)
	}
	if code := bob.getStatus(fmt.Sprintf("/feed/p/%d/media", p.ID)); code != http.StatusNotFound {
		t.Errorf("bob can load the photo of a private post: %d", code)
	}

	// A drawing by itself, as a drawing program saves it.
	svg := `<?xml version="1.0"?><!-- made by a program --><!DOCTYPE svg><svg width="200" height="100"><rect width="200" height="100"/></svg>`
	code, card = alice.postWithPicture(map[string]string{"body": ""}, "plan.svg", []byte(svg))
	p = newest(t, a, alice.id)
	if code != http.StatusOK || !p.HasSVG || p.Body != "" || p.SVGW != 200 || p.MediaKind != "" {
		t.Fatalf("drawing post: %d %+v", code, p)
	}
	if !strings.Contains(card, fmt.Sprintf(`/feed/p/%d/picture.svg`, p.ID)) || strings.Contains(card, `class="feed-text"`) {
		t.Errorf("the card of a drawing without text: %s", card)
	}
	// Its text may stay empty when it is changed; a plain post's may not.
	if code, _ := alice.hx(fmt.Sprintf("/feed/p/%d", p.ID), url.Values{"body": {""}}); code != http.StatusOK {
		t.Errorf("a drawing needs no text: %d", code)
	}

	// Neither text nor picture, and files that are no pictures, are refused.
	before := a.st.CountFeed(alice.id, store.FeedFilter{View: store.FeedViewMine})
	if code, _ := alice.postWithPicture(map[string]string{"body": " "}, "", nil); code != http.StatusBadRequest {
		t.Errorf("an empty post: %d", code)
	}
	if code, _ := alice.postWithPicture(map[string]string{"body": "x"}, "notes.png", []byte("just some words")); code != http.StatusBadRequest {
		t.Errorf("a text file as a picture: %d", code)
	}
	if n := a.st.CountFeed(alice.id, store.FeedFilter{View: store.FeedViewMine}); n != before {
		t.Errorf("refused posts were written: %d, was %d", n, before)
	}

	// The AI cannot post without text, and cannot bring a file.
	if _, err := a.st.CreateFeedPost(alice.id, store.FeedPostInput{SVG: "<svg></svg>"}, store.FeedActor{Kind: store.FeedByAI}); err == nil {
		t.Error("the AI posted a picture without text")
	}

	page := alice.get("/feed")
	for _, want := range []string{`name="picture"`, `hx-encoding="multipart/form-data"`, "Add a photo or drawing", `data-when`, `data-tz="UTC"`, ">AI access<", ">More settings<"} {
		if !strings.Contains(page, want) {
			t.Errorf("the feed is missing %q", want)
		}
	}
}

// Old posts are removed for good with their files, after as many days as the
// owner chose. Kept posts stay.
func TestFeedOldPostsAreRemoved(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")

	var shot bytes.Buffer
	png.Encode(&shot, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	add := func(userID int64, body string, days int, photo bool) store.FeedPost {
		in := store.FeedPostInput{Body: body}
		if photo {
			in.Photo = &store.FeedPhoto{Data: shot.Bytes(), Mime: "image/png"}
		}
		id, err := a.st.CreateFeedPost(userID, in, store.FeedActor{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.st.DB.Exec(`UPDATE feed_posts SET created_at = datetime('now', ?) WHERE id = ?`, fmt.Sprintf("-%d days", days), id); err != nil {
			t.Fatal(err)
		}
		p, _ := a.st.FeedPostByID(userID, id)
		return *p
	}
	old := add(alice.id, "old, with a photo", 31, true)
	kept := add(alice.id, "old, kept", 200, true)
	fresh := add(alice.id, "three weeks", 21, false)
	bobs := add(bob.id, "bob keeps everything", 400, false)

	if a.st.FeedKeepDays(alice.id) != store.FeedKeepDefault {
		t.Fatalf("posts are kept for %d days to begin with", a.st.FeedKeepDays(alice.id))
	}
	if code, card := alice.hx(fmt.Sprintf("/feed/p/%d/keep", kept.ID), url.Values{"keep": {"1"}}); code != http.StatusOK || !strings.Contains(card, "Kept") || !strings.Contains(card, "Stop keeping") {
		t.Fatalf("keep: %d %s", code, card)
	}
	if code, _ := bob.hx(fmt.Sprintf("/feed/p/%d/keep", fresh.ID), url.Values{"keep": {"1"}}); code != http.StatusNotFound {
		t.Errorf("bob kept alice's post: %d", code)
	}
	if code, _, h := bob.post("/feed/settings/keep", url.Values{"keep_days": {"0"}}); code != http.StatusSeeOther || h.Get("Location") != "/feed/settings?done=keep#more" {
		t.Fatalf("bob's choice: %d %s", code, h.Get("Location"))
	}

	gone := func(userID int64, p store.FeedPost) bool {
		_, err := a.st.FeedPostByID(userID, p.ID)
		return err != nil
	}
	if n, err := a.st.PurgeOldFeedPosts(); err != nil || n != 1 {
		t.Fatalf("first sweep removed %d: %v", n, err)
	}
	if !gone(alice.id, old) || gone(alice.id, kept) || gone(alice.id, fresh) || gone(bob.id, bobs) {
		t.Fatal("the sweep did not remove exactly the one old post")
	}
	if _, err := os.Stat(a.st.FeedMediaPath(&old)); !os.IsNotExist(err) {
		t.Errorf("the photo of the removed post is still on disk: %v", err)
	}
	if _, err := os.Stat(a.st.FeedMediaPath(&kept)); err != nil {
		t.Errorf("the photo of the kept post is gone: %v", err)
	}

	// A shorter time reaches posts that are already there; a value that is not offered is not taken.
	alice.post("/feed/settings/keep", url.Values{"keep_days": {"14"}})
	if n, _ := a.st.PurgeOldFeedPosts(); n != 1 || !gone(alice.id, fresh) || gone(alice.id, kept) {
		t.Fatalf("after choosing 14 days the sweep removed %d", n)
	}
	alice.post("/feed/settings/keep", url.Values{"keep_days": {"3"}})
	if d := a.st.FeedKeepDays(alice.id); d != store.FeedKeepDefault {
		t.Errorf("a number of days that is not offered was stored as %d", d)
	}

	settings := alice.get("/feed/settings?done=keep")
	for _, want := range []string{"<h2>AI access</h2>", "<h2>More settings</h2>", `action="/feed/settings/keep"`, `<option value="30" selected>30 days</option>`, "Never: keep every post", "Posts older than 30 days are removed"} {
		if !strings.Contains(settings, want) {
			t.Errorf("feed settings is missing %q", want)
		}
	}
}
