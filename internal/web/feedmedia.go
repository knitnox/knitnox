package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"knitnox/internal/store"
)

// Feed settings (/feed/settings): whether the AI and agents may post ("AI
// access"), and under "More settings" how long posts are kept and which
// models make the pictures, voice and video on posts. Models can be chosen from the ones the
// admin shares and the user's own; a user can add a model of their own right
// here, without going to the Settings page first. It is saved in the same
// place as every other provider and key (Settings, "Your API keys"), so a key
// is typed once and can be used everywhere.

// feedModelPick is one of the three model choices on the Feed settings page.
type feedModelPick struct {
	Kind    string // image | voice | video
	Field   string // form field
	Label   string
	Hint    string
	Chosen  int64 // 0 = the server's, -1 = off, else a model id
	Server  string
	Options []store.ModelOption
	Working bool // a model will be used as things stand
}

func (s *Server) feedSettingsPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	fm := s.st.FeedModelsFor(u.ID)
	picks := []feedModelPick{
		{Kind: store.MediaImage, Field: "image_model_id", Label: "Pictures", Hint: "Draws a picture from a description, such as “a loaf of bread on a wooden table”.", Chosen: fm.Image},
		{Kind: store.MediaVoice, Field: "voice_model_id", Label: "Voice", Hint: "Reads a post aloud, so you can listen instead of read.", Chosen: fm.Voice},
		{Kind: store.MediaVideo, Field: "video_model_id", Label: "Video", Hint: "Makes a short video from a description. This is slow (minutes) and usually the most expensive.", Chosen: fm.Video},
	}
	kinds := s.eng.FeedMediaKinds(u.ID)
	for i := range picks {
		p := &picks[i]
		p.Options, _ = s.st.KindModelOptions(u.ID, p.Kind)
		for _, o := range p.Options {
			if !o.Personal && p.Server == "" {
				p.Server = o.ModelName
			}
		}
		p.Working = kinds[p.Kind]
	}
	uid := u.ID
	providers, _ := s.st.ListProviders(&uid)
	agents, _ := s.st.ListAgents(u.ID)
	st, _ := s.st.LoadSettings()
	s.render(w, r, "feed", map[string]any{
		"Tab": "settings", "On": s.st.FeedEnabled(u.ID), "PublicOn": st.FeedPublic, "BinDays": store.FeedBinDays,
		"Notice": r.URL.Query().Get("done"), "Picks": picks, "VoiceName": fm.VoiceName, "Providers": providers,
		"AgentList": agents, "MediaDaily": st.FeedMediaDaily,
		"BinCount": s.st.CountFeed(u.ID, store.FeedFilter{View: store.FeedViewBin}),
		"KeepDays": s.st.FeedKeepDays(u.ID), "KeepChoices": store.FeedKeepChoices,
	})
}

// feedModelsSave stores which models make the media for the user's feed.
func (s *Server) feedModelsSave(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	pick := func(field, kind string) int64 {
		v := formInt(r, field, 0)
		if v <= 0 {
			return max(v, -1)
		}
		opts, _ := s.st.KindModelOptions(u.ID, kind)
		for _, o := range opts {
			if o.ModelID == v {
				return v
			}
		}
		return 0 // not theirs to use: fall back to the server's model
	}
	fm := store.FeedModels{Image: pick("image_model_id", store.MediaImage), Voice: pick("voice_model_id", store.MediaVoice),
		Video: pick("video_model_id", store.MediaVideo), VoiceName: r.FormValue("voice_name")}
	if err := s.st.SaveFeedModels(u.ID, fm); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/feed/settings?done=models#models", http.StatusSeeOther)
}

// quickAddModel reads the small "add my own model" form and saves it as one
// of the user's own models of the given kind: on a provider they already
// have, or on a new one made from an address and a key. It returns the model
// and its provider.
func (s *Server) quickAddModel(r *http.Request, kind string) (modelID, providerID int64, err error) {
	if !store.ValidModelKind(kind) {
		return 0, 0, errors.New("choose what the model is for")
	}
	uid := currentUser(r).ID
	name := strings.TrimSpace(r.FormValue("model"))
	if name == "" || len(name) > 200 {
		return 0, 0, errors.New("type the name of the model, as your provider calls it")
	}
	pid := formInt(r, "provider_id", 0)
	if pid > 0 {
		if !s.st.ProviderOwnedBy(&uid, pid) {
			return 0, 0, errors.New("pick one of your own providers")
		}
	} else {
		base := strings.TrimSpace(r.FormValue("base_url"))
		if !validURL(base) {
			return 0, 0, errors.New("type the address of the provider, starting with http:// or https://")
		}
		label := strings.TrimSpace(r.FormValue("provider_name"))
		if label == "" {
			if u, perr := url.Parse(base); perr == nil {
				label = u.Hostname()
			}
		}
		if pid, err = s.st.CreateProvider(&uid, label, base, strings.TrimSpace(r.FormValue("api_key"))); err != nil {
			return 0, 0, err
		}
	}
	if err := s.st.CreateModelKind(pid, name, kind, 0, 0, 0, false); err != nil {
		return 0, 0, err
	}
	id := s.st.ModelIDByName(pid, name)
	if id == 0 {
		return 0, 0, errors.New("the model could not be saved")
	}
	// A model of that name may have been there already with another purpose.
	if err := s.st.SetModelKind(id, kind); err != nil {
		return 0, 0, err
	}
	return id, pid, nil
}

// feedModelAdd adds the user's own picture, voice or video model and chooses it.
func (s *Server) feedModelAdd(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	kind := r.FormValue("kind")
	if kind != store.MediaImage && kind != store.MediaVoice && kind != store.MediaVideo {
		s.errorPage(w, r, http.StatusBadRequest, "Choose what the model makes: pictures, voice or video.")
		return
	}
	id, _, err := s.quickAddModel(r, kind)
	if err != nil {
		msg := err.Error()
		s.errorPage(w, r, http.StatusBadRequest, strings.ToUpper(msg[:1])+msg[1:]+".")
		return
	}
	fm := s.st.FeedModelsFor(u.ID)
	switch kind {
	case store.MediaImage:
		fm.Image = id
	case store.MediaVoice:
		fm.Voice = id
	default:
		fm.Video = id
	}
	if err := s.st.SaveFeedModels(u.ID, fm); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/feed/settings?done=added#models", http.StatusSeeOther)
}

// memoryModelAdd adds the user's own embedding model from the Memory settings and switches to it.
func (s *Server) memoryModelAdd(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	_, pid, err := s.quickAddModel(r, store.ModelEmbedding)
	if err != nil {
		msg := err.Error()
		s.errorPage(w, r, http.StatusBadRequest, strings.ToUpper(msg[:1])+msg[1:]+".")
		return
	}
	ms, err := s.st.MemorySettingsFor(u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	ms.EmbedProviderID, ms.EmbedModel = pid, strings.TrimSpace(r.FormValue("model"))
	if err := s.st.SaveMemorySettings(u.ID, ms); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.eng.IndexMemoriesSoon(store.UserMemory(u.ID))
	http.Redirect(w, r, "/memory/settings?done=search", http.StatusSeeOther)
}

// feedMedia serves the picture, recording or video of a post the viewer may
// see. The file came from an AI provider, so it is handed out under a policy
// that lets it do nothing but be shown or played.
func (s *Server) feedMedia(w http.ResponseWriter, r *http.Request) {
	p, err := s.st.FeedPostVisible(currentUser(r).ID, pathID(r, "id"), s.feedPublicOn())
	if err != nil || p.MediaStatus != store.MediaReady || p.MediaFile == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(s.st.FeedMediaPath(p))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	h := w.Header()
	h.Set("Content-Type", p.MediaMime)
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, "", time.Time{}, f) // answers range requests, which players need to seek
}

// ---- what a model is for (Settings and Admin, providers list) ----

func (s *Server) modelKind(w http.ResponseWriter, r *http.Request, sc scope) {
	kind := r.FormValue("kind")
	if !store.ValidModelKind(kind) {
		s.errorPage(w, r, http.StatusBadRequest, "Unknown kind of model.")
		return
	}
	s.modelAction(w, r, sc, "model.kind", func(id int64) error { return s.st.SetModelKind(id, kind) })
}

func (s *Server) userModelKind(w http.ResponseWriter, r *http.Request) {
	s.modelKind(w, r, userScope(r))
}
func (s *Server) adminModelKind(w http.ResponseWriter, r *http.Request) {
	s.modelKind(w, r, adminScope())
}

// modelVoice sets the voice a voice model speaks with. For a model the server
// shares, it is the voice everyone hears unless they chose their own in Feed settings.
func (s *Server) modelVoice(w http.ResponseWriter, r *http.Request, sc scope) {
	name := r.FormValue("voice_name")
	s.modelAction(w, r, sc, "model.voice", func(id int64) error { return s.st.SetModelVoiceName(id, name) })
}

func (s *Server) userModelVoice(w http.ResponseWriter, r *http.Request) {
	s.modelVoice(w, r, userScope(r))
}
func (s *Server) adminModelVoice(w http.ResponseWriter, r *http.Request) {
	s.modelVoice(w, r, adminScope())
}

// modelKindWords is what the kinds are called on the page.
var modelKindWords = map[string]string{store.ModelChat: "Chat", store.ModelEmbedding: "Embedding (memory search)",
	store.ModelImage: "Pictures", store.ModelVoice: "Voice", store.ModelVideo: "Video"}

func modelKindLabel(kind string) string {
	if w := modelKindWords[kind]; w != "" {
		return w
	}
	return fmt.Sprint(kind)
}
