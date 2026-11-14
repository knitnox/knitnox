package engine

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // sizes of pictures the models send back
	_ "image/jpeg"
	_ "image/png"
	"runtime/debug"
	"strings"
	"time"

	"knitnox/internal/hub"
	"knitnox/internal/llm"
	"knitnox/internal/store"
)

// Pictures, voice and video on feed posts
//
// A post can ask for one piece of media made by an AI model: a picture, the
// post read aloud, or a short video. The post is saved at once with its media
// "pending"; the file is made in the background and the open Feed pages are
// told when it is ready, so nobody waits for a slow model.
//
// Which model is used is the owner's choice on the Feed settings page: their
// own, or the one the admin shares. Media made with a shared model counts
// toward a daily limit per person, set by the admin.
//
// A picture asked for by the AI or an agent is looked at once by the run's own
// model, when that model can see images: if it does not show what was asked
// for, it is drawn a second time from a clearer description.

var feedMediaTimeout = map[string]time.Duration{
	store.MediaImage: 4 * time.Minute,
	store.MediaVoice: 3 * time.Minute,
	store.MediaVideo: 20 * time.Minute,
}

// feedMediaWords names a kind of media in a sentence.
var feedMediaWords = map[string]string{store.MediaImage: "picture", store.MediaVoice: "voice recording", store.MediaVideo: "video"}

// FeedMediaKinds lists the kinds of media that can be made for a user right
// now: a model of that kind is set up and not turned off.
func (e *Engine) FeedMediaKinds(userID int64) map[string]bool {
	out := map[string]bool{}
	for _, kind := range []string{store.MediaImage, store.MediaVoice, store.MediaVideo} {
		if m, _, err := e.st.FeedMediaModel(userID, kind); err == nil && m != nil {
			out[kind] = true
		}
	}
	return out
}

// feedMediaNote says in one sentence, for the system prompt, which media a
// post can carry for this user ("" = none). It changes only when the user
// changes their models.
func (e *Engine) feedMediaNote(userID int64) string {
	kinds := e.FeedMediaKinds(userID)
	var parts []string
	if kinds[store.MediaImage] {
		parts = append(parts, "a picture made by an image model (`image`: describe what it shows; good for scenes and illustrations, while diagrams are better drawn in `svg`)")
	}
	if kinds[store.MediaVoice] {
		parts = append(parts, "a recording of text read aloud (`voice`)")
	}
	if kinds[store.MediaVideo] {
		parts = append(parts, "a short video (`video`: slow and costly, only when asked for)")
	}
	if len(parts) == 0 {
		return ""
	}
	return "A post can also carry one of these: " + strings.Join(parts, "; ") + ". It is made after you post and appears on the post by itself; you do not get it back."
}

// checkModel is the model that may look at a picture this run asks for: the
// run's own model, when it can see images.
func (tc *turnCtx) checkModel() int64 {
	if tc.model != nil && tc.model.Vision {
		return tc.model.ID
	}
	return 0
}

// CreateFeedPost adds a post to userID's feed, tells open Feed pages, and
// starts making its media when it asks for some. checkModel is the chat model
// that may look at a finished picture (0 = nobody looks).
func (e *Engine) CreateFeedPost(userID int64, in store.FeedPostInput, by store.FeedActor, checkModel int64) (int64, error) {
	if in.MediaKind != "" {
		word := feedMediaWords[in.MediaKind]
		if word == "" {
			return 0, errors.New("unknown kind of media")
		}
		in.MediaPrompt = strings.TrimSpace(in.MediaPrompt)
		if in.MediaPrompt == "" {
			return 0, fmt.Errorf("say what the %s should show or say", word)
		}
		m, p, err := e.st.FeedMediaModel(userID, in.MediaKind)
		if err != nil {
			return 0, err
		}
		if m == nil {
			return 0, fmt.Errorf("no model is set up to make a %s; choose one on the Feed settings page", word)
		}
		in.MediaShared = p.IsGlobal()
		if in.MediaShared { // one limit for the Feed and for chats (chatmedia.go)
			if err := e.mediaLimitLeft(userID); err != nil {
				return 0, err
			}
		}
	}
	id, err := e.st.CreateFeedPost(userID, in, by)
	if err != nil {
		return 0, err
	}
	e.announceFeed(userID, id, in.Public, false)
	if e.OnFeedPost != nil {
		ev := FeedEvent{UserID: userID, PostID: id, Public: in.Public, By: by}
		e.hook("feed-post hook", func() { e.OnFeedPost(ev) })
	}
	if in.MediaKind != "" {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			defer func() { // a post must never stay "being made" for ever
				if v := recover(); v != nil {
					e.log.Error("panic while making feed media", "post", id, "value", fmt.Sprint(v), "stack", string(debug.Stack()))
					e.st.FailFeedMedia(id, "something went wrong inside Knitnox")
					e.announceFeed(userID, id, false, true)
				}
			}()
			e.makeFeedMedia(userID, id, in.MediaKind, in.MediaPrompt, checkModel, by.ThreadID)
		}()
	}
	return id, nil
}

// announceFeed tells open Feed pages about a new post, or that one changed.
func (e *Engine) announceFeed(userID, postID int64, public, changed bool) {
	data := map[string]any{"id": postID}
	if changed {
		data["changed"] = true
	}
	e.hub.Publish(hub.FeedUser(userID), EvFeed, data)
	if public {
		e.hub.Publish(hub.FeedPublic, EvFeed, data)
	}
}

// mediaTypes are the file types kept, per kind, with the file ending used on disk.
var mediaTypes = map[string]map[string]string{
	store.MediaImage: {"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif"},
	store.MediaVoice: {"audio/mpeg": ".mp3", "audio/mp3": ".mp3", "audio/wav": ".wav", "audio/x-wav": ".wav", "audio/wave": ".wav",
		"audio/ogg": ".ogg", "audio/opus": ".opus", "audio/aac": ".aac", "audio/flac": ".flac", "audio/mp4": ".m4a"},
	store.MediaVideo: {"video/mp4": ".mp4", "video/webm": ".webm", "video/quicktime": ".mov"},
}

// makeFeedMedia makes the file for one post and records how it went.
func (e *Engine) makeFeedMedia(userID, postID int64, kind, prompt string, checkModel, threadID int64) {
	ctx, cancel := context.WithTimeout(e.baseCtx, feedMediaTimeout[kind])
	defer cancel()
	done := func() {
		public := false
		if p, err := e.st.FeedPostByID(userID, postID); err == nil {
			public = p.Public && !p.InBin()
		}
		e.announceFeed(userID, postID, public, true)
	}
	fail := func(err error) {
		why := err.Error()
		var he *llm.HTTPError
		if errors.As(err, &he) {
			said := he.Body
			var body struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(he.Body), &body) == nil && body.Error.Message != "" {
				said = body.Error.Message // the provider's own words, without the wrapping
			}
			why = fmt.Sprintf("the provider said (%d): %s", he.Status, oneLine(said, 200))
		}
		e.log.Warn("feed media failed", "post", postID, "kind", kind, "err", err)
		e.st.FailFeedMedia(postID, oneLine(why, 300))
		done()
	}
	m, p, err := e.st.FeedMediaModel(userID, kind)
	if err != nil || m == nil {
		fail(errors.New("the model is not set up any more"))
		return
	}
	req := llm.MediaRequest{BaseURL: p.BaseURL, APIKey: p.APIKey, Model: m.Name, Prompt: prompt, UserURL: !p.IsGlobal()}
	make := func() (*llm.Media, error) {
		switch kind {
		case store.MediaImage:
			return llm.GenerateImage(ctx, req)
		case store.MediaVoice:
			req.Voice = e.st.VoiceNameFor(userID, m)
			return llm.Speak(ctx, req)
		}
		return llm.GenerateVideo(ctx, req, e.VideoPoll)
	}
	media, err := make()
	if err != nil {
		fail(err)
		return
	}
	note := ""
	if kind == store.MediaImage && checkModel > 0 {
		if better := e.reviewImage(ctx, userID, threadID, checkModel, prompt, media); better != "" {
			req.Prompt = better
			if again, err := make(); err == nil {
				media, prompt = again, better
				note = "The AI looked at the first picture, found it did not match, and had it drawn again from a clearer description."
			}
		}
	}
	ext := mediaTypes[kind][media.Mime]
	if ext == "" {
		fail(fmt.Errorf("the provider sent a file of type %q, which is not a %s", oneLine(media.Mime, 60), feedMediaWords[kind]))
		return
	}
	w, h := 0, 0
	if kind == store.MediaImage {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(media.Data)); err == nil {
			w, h = cfg.Width, cfg.Height
		}
	}
	if err := e.st.SaveFeedMedia(postID, media.Data, media.Mime, ext, prompt, note, w, h); err != nil {
		if !errors.Is(err, store.ErrNotFound) { // not found = the post was deleted for good meanwhile
			fail(err)
		}
		return
	}
	done()
}

const imageReviewPrompt = `You check a picture that an image model drew for a short post. You are given the description it was asked to draw.
If the picture shows what the description asks for, answer with the single word OK.
If it clearly does not (wrong subject, missing main thing, unreadable), answer with one better description for the image model: plain, concrete, under 600 characters, and nothing else.`

// reviewImage lets a chat model that can see images look at a finished
// picture once. It returns a clearer description to draw from, or "" when the
// picture is fine or nobody can look.
func (e *Engine) reviewImage(ctx context.Context, userID, threadID, modelID int64, prompt string, media *llm.Media) string {
	m, err := e.st.ModelByID(modelID)
	if err != nil || !m.Enabled || !m.Vision || m.Kind != store.ModelChat || len(media.Data) > 15<<20 {
		return ""
	}
	p, err := e.st.ProviderByID(m.ProviderID)
	if err != nil || !p.Enabled {
		return ""
	}
	if p.IsGlobal() && e.checkQuota(userID) != nil {
		return ""
	}
	msgs := []llm.Message{
		{Role: "system", Content: imageReviewPrompt},
		{Role: "user", Content: "The description: " + prompt, Images: []string{"data:" + media.Mime + ";base64," + base64.StdEncoding.EncodeToString(media.Data)}},
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	res, err := llm.Complete(cctx, llm.Request{BaseURL: p.BaseURL, APIKey: p.APIKey, Model: m.Name, Messages: msgs, UserURL: !p.IsGlobal()})
	if err != nil {
		e.log.Warn("feed picture check failed", "err", err)
		return ""
	}
	entry := store.UsageEntry{UserID: userID, ThreadID: threadID, ProviderID: p.ID, ModelName: m.Name, Kind: "feed-check", Billable: p.IsGlobal()}
	if res.Usage != nil {
		entry.PromptTokens, entry.CompletionTokens = res.Usage.PromptTokens, res.Usage.CompletionTokens
	} else {
		entry.Estimated = true
		entry.PromptTokens, entry.CompletionTokens = llm.EstimateTokens(imageReviewPrompt+prompt)+800, llm.EstimateTokens(res.Content)
	}
	entry.Cost = (float64(entry.PromptTokens)*m.InputPrice + float64(entry.CompletionTokens)*m.OutputPrice) / 1e6
	e.st.RecordUsage(entry)

	answer := strings.TrimSpace(res.Content)
	if answer == "" || strings.HasPrefix(strings.ToUpper(answer), "OK") || len([]rune(answer)) < 12 {
		return ""
	}
	return clipText(answer, 800)
}
