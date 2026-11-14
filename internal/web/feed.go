package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"image"
	_ "image/gif" // sizes of the photos people add
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"knitnox/internal/engine"
	"knitnox/internal/hub"
	"knitnox/internal/store"
)

// Feed: one page of short posts (/feed). A user writes posts there, and the AI
// in their chats and agents they allowed add posts too (engine/feed.go). A
// post is private unless its owner shares it with everyone on this server;
// only the owner can change, share or delete it.
//
// The page shows the newest feedFirstPage posts and loads feedNextPage older
// ones each time the end of the list comes into view. New posts are announced
// by a small button instead of being pushed into the list, so nothing moves
// while someone is reading. Which posts are shown travels with each request as
// four fields: view, from, agent, q.

const (
	feedFirstPage = 20
	feedNextPage  = 10
	feedNewMax    = 50 // most new posts put in at once
)

// feedState is which posts the page shows.
type feedState struct {
	View  string // mine | public | all | bin
	From  string // "" | user | ai | agent
	Agent string // one agent, by name
	Query string
}

func (st feedState) values() url.Values {
	v := url.Values{}
	if st.View != "" {
		v.Set("view", st.View)
	}
	if st.From != "" {
		v.Set("from", st.From)
	}
	if st.Agent != "" {
		v.Set("agent", st.Agent)
	}
	if st.Query != "" {
		v.Set("q", st.Query)
	}
	return v
}

// Params is the state as a query string, for the links that load more posts.
func (st feedState) Params() string { return st.values().Encode() }

func (st feedState) url() string {
	if q := st.Params(); q != "" {
		return "/feed?" + q
	}
	return "/feed"
}

// ViewURL is the page address for another view, keeping who the posts are from.
func (st feedState) ViewURL(view string) string {
	st.View, st.Query = view, ""
	if view == store.FeedViewBin || view == store.FeedViewPublic {
		st.From, st.Agent = "", ""
	}
	return st.url()
}

// FromURL is the page address for posts from one kind of author (or one agent).
func (st feedState) FromURL(from, agent string) string {
	st.From, st.Agent, st.Query = from, agent, ""
	return st.url()
}

// Filtered reports whether only some of the posts in this view are shown.
func (st feedState) Filtered() bool { return st.From != "" || st.Agent != "" || st.Query != "" }

func (st feedState) filter() store.FeedFilter {
	return store.FeedFilter{View: st.View, From: st.From, Agent: st.Agent, Query: st.Query}
}

// feedPublicOn reports whether people may share posts on this server.
func (s *Server) feedPublicOn() bool {
	st, err := s.st.LoadSettings()
	return err == nil && st.FeedPublic
}

func readFeedState(r *http.Request, publicOn bool) feedState {
	st := feedState{View: r.FormValue("view"), From: r.FormValue("from"), Query: strings.TrimSpace(r.FormValue("q"))}
	switch st.View {
	case store.FeedViewMine, store.FeedViewBin:
	case store.FeedViewPublic, store.FeedViewAll:
		if !publicOn {
			st.View = store.FeedViewMine
		}
	default:
		st.View = store.FeedViewMine
		if publicOn {
			st.View = store.FeedViewAll
		}
	}
	switch st.From {
	case store.FeedByUser, store.FeedByAI, store.FeedByAgent:
	default:
		st.From = ""
	}
	if a := strings.TrimSpace(r.FormValue("agent")); a != "" && len(a) <= 200 {
		st.Agent, st.From = a, store.FeedByAgent
	}
	if rs := []rune(st.Query); len(rs) > 100 {
		st.Query = string(rs[:100])
	}
	return st
}

// feedPostView is a post as one viewer sees it.
type feedPostView struct {
	store.FeedPost
	Mine     bool          // the viewer owns it
	IsNew    bool          // added since the viewer last looked
	CanPull  bool          // the viewer is an admin and may take it out of the public feed
	PublicOn bool          // sharing is allowed on this server
	When     string        // "5 min ago", "Today 14:05", "Mon 14:05", "3 Nov"
	WhenFull string        // the exact local time
	WhenISO  string        // the moment itself, so the page can keep "5 min ago" true
	Chart    template.HTML // the chart, drawn
	// The size a picture is shown at: its own size, made smaller when it is
	// larger than the card, never made larger. 0 = unknown.
	PicW, PicH     int // the picture drawn as SVG
	ShotW, ShotH   int // the picture made by an image model
	MediaWord      string
	MediaIsPending bool
	MediaIsReady   bool
	MediaIsFailed  bool
}

// fitBox scales a size down (never up) to fit maxW by maxH.
func fitBox(w, h, maxW, maxH int) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	scale := min(1, float64(maxW)/float64(w), float64(maxH)/float64(h))
	return max(int(float64(w)*scale+0.5), 1), max(int(float64(h)*scale+0.5), 1)
}

var feedMediaWord = map[string]string{store.MediaImage: "picture", store.MediaVoice: "recording", store.MediaVideo: "video"}

// feedWhen says when something happened in plain words, in the viewer's time zone.
func feedWhen(ts string, loc *time.Location, now time.Time) (words, full string) {
	t, err := time.Parse("2006-01-02 15:04:05", ts)
	if err != nil {
		return ts, ts
	}
	t, now = t.In(loc), now.In(loc)
	full = t.Format("Monday 2 January 2006, 15:04")
	ago := now.Sub(t)
	day := func(x time.Time) time.Time { return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, loc) }
	days := int(day(now).Sub(day(t)).Hours()/24 + 0.5)
	switch {
	case ago < time.Minute:
		return "just now", full
	case ago < time.Hour:
		return fmt.Sprintf("%d min ago", int(ago.Minutes())), full
	case days == 0:
		return "Today " + t.Format("15:04"), full
	case days == 1:
		return "Yesterday " + t.Format("15:04"), full
	case days < 7:
		return t.Format("Mon 15:04"), full
	case t.Year() == now.Year():
		return t.Format("2 Jan"), full
	}
	return t.Format("2 Jan 2006"), full
}

func (s *Server) viewFeedPost(p store.FeedPost, u *store.User, seen int64, publicOn bool) feedPostView {
	v := feedPostView{FeedPost: p, Mine: p.UserID == u.ID, PublicOn: publicOn}
	v.IsNew = seen >= 0 && p.ID > seen && !(v.Mine && p.CreatedBy == store.FeedByUser)
	v.CanPull = !v.Mine && p.Public && u.IsAdmin()
	v.When, v.WhenFull = feedWhen(p.CreatedAt, u.Location(), time.Now())
	if t, err := time.Parse("2006-01-02 15:04:05", p.CreatedAt); err == nil {
		v.WhenISO = t.UTC().Format(time.RFC3339)
	}
	if p.Chart != nil {
		v.Chart = drawFeedChart(*p.Chart)
	}
	v.PicW, v.PicH = fitBox(p.SVGW, p.SVGH, 640, 480)
	v.ShotW, v.ShotH = fitBox(p.MediaW, p.MediaH, 520, 520)
	v.MediaWord = feedMediaWord[p.MediaKind]
	v.MediaIsPending, v.MediaIsReady, v.MediaIsFailed = p.MediaStatus == store.MediaPending, p.MediaStatus == store.MediaReady, p.MediaStatus == store.MediaFailed
	if !v.Mine { // what went wrong, and what the model was asked for, is the owner's business
		v.MediaNote = ""
		if v.MediaIsFailed {
			v.MediaKind, v.MediaIsFailed = "", false
		}
	}
	if !v.Mine { // where a post came from is its owner's business
		v.SourceThreadID, v.SourceTitle = 0, ""
	}
	return v
}

func (s *Server) viewFeedPosts(posts []store.FeedPost, u *store.User, seen int64, publicOn bool) []feedPostView {
	out := make([]feedPostView, len(posts))
	for i, p := range posts {
		out[i] = s.viewFeedPost(p, u, seen, publicOn)
	}
	return out
}

// ---- the chart ----

func feedNum(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" {
		s = "0"
	}
	return s
}

// drawFeedChart draws a chart as SVG. Colours come from the page's style
// sheet (classes), so it fits the light and the dark theme. Every text is escaped.
func drawFeedChart(c store.FeedChart) template.HTML {
	lo, hi := 0.0, 0.0
	for i, v := range c.Values {
		if i == 0 && c.Type == "line" {
			lo, hi = v, v
		}
		lo, hi = min(lo, v), max(hi, v)
	}
	if hi == lo {
		hi, lo = hi+1, lo-1
		if c.Type == "bar" {
			lo = min(lo+1, 0)
		}
	}
	esc := html.EscapeString
	val := func(v float64) string { return esc(feedNum(v) + c.Unit) }
	var b strings.Builder
	desc := c.Title
	if desc == "" {
		desc = "Chart"
	}
	for i, l := range c.Labels {
		desc += fmt.Sprintf("; %s: %s", l, feedNum(c.Values[i])+c.Unit)
	}
	if c.Type == "bar" {
		const rowH, left, right, width = 28, 118.0, 330.0, 400
		h := len(c.Values)*rowH + 8
		fmt.Fprintf(&b, `<svg class="feed-chart-svg" viewBox="0 0 %d %d" role="img" aria-label="%s">`, width, h, esc(desc))
		zero := left + (0-lo)/(hi-lo)*(right-left)
		for i, v := range c.Values {
			y := i*rowH + 4
			x := left + (v-lo)/(hi-lo)*(right-left)
			x0, w := min(zero, x), max(x, zero)-min(zero, x)
			fmt.Fprintf(&b, `<text class="fc-label" x="%g" y="%d" text-anchor="end">%s</text>`, left-8, y+18, esc(clipRunes(c.Labels[i], 15)))
			fmt.Fprintf(&b, `<rect class="fc-bar" x="%.1f" y="%d" width="%.1f" height="%d" rx="5"/>`, x0, y+4, max(w, 2), rowH-10)
			fmt.Fprintf(&b, `<text class="fc-value" x="%.1f" y="%d">%s</text>`, max(x, zero)+6, y+18, val(v))
		}
		b.WriteString(`</svg>`)
		return template.HTML(b.String())
	}
	const width, height, left, right, top, bottom = 400, 196, 50.0, 388.0, 14.0, 158.0
	fmt.Fprintf(&b, `<svg class="feed-chart-svg" viewBox="0 0 %d %d" role="img" aria-label="%s">`, width, height, esc(desc))
	fmt.Fprintf(&b, `<line class="fc-axis" x1="%g" y1="%g" x2="%g" y2="%g"/><line class="fc-axis" x1="%g" y1="%g" x2="%g" y2="%g"/>`, left, top, right, top, left, bottom, right, bottom)
	fmt.Fprintf(&b, `<text class="fc-label" x="%g" y="%g" text-anchor="end">%s</text><text class="fc-label" x="%g" y="%g" text-anchor="end">%s</text>`,
		left-8, top+4, val(hi), left-8, bottom+4, val(lo))
	n := len(c.Values)
	px := func(i int) float64 {
		if n == 1 {
			return (left + right) / 2
		}
		return left + float64(i)/float64(n-1)*(right-left)
	}
	py := func(v float64) float64 { return bottom - (v-lo)/(hi-lo)*(bottom-top) }
	var pts []string
	for i, v := range c.Values {
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", px(i), py(v)))
	}
	fmt.Fprintf(&b, `<polyline class="fc-line" points="%s"/>`, strings.Join(pts, " "))
	every := (n + 4) / 5 // at most five labels under the line
	for i, v := range c.Values {
		fmt.Fprintf(&b, `<circle class="fc-dot" cx="%.1f" cy="%.1f" r="4"><title>%s: %s</title></circle>`, px(i), py(v), esc(c.Labels[i]), val(v))
		if i%every == 0 {
			anchor := "middle"
			if i == 0 && n > 1 {
				anchor = "start"
			} else if i == n-1 && n > 1 {
				anchor = "end"
			}
			fmt.Fprintf(&b, `<text class="fc-label" x="%.1f" y="%g" text-anchor="%s">%s</text>`, px(i), bottom+20, anchor, esc(clipRunes(c.Labels[i], 10)))
		}
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

func clipRunes(s string, n int) string {
	if rs := []rune(s); len(rs) > n {
		return string(rs[:n-1]) + "…"
	}
	return s
}

// ---- the page ----

func (s *Server) feedPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	publicOn := s.feedPublicOn()
	st := readFeedState(r, publicOn)
	f := st.filter()
	f.Limit = feedFirstPage
	posts, more, err := s.st.ListFeed(u.ID, f)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	seen := s.st.FeedSeen(u.ID)
	if st.View == store.FeedViewBin {
		seen = -1 // nothing in the bin is "new"
	}
	s.feedShown(u, st, posts, publicOn)
	data := map[string]any{
		"State": st, "Posts": s.viewFeedPosts(posts, u, seen, publicOn), "More": more, "PublicOn": publicOn,
		"On": s.st.FeedEnabled(u.ID), "BinDays": store.FeedBinDays, "Tab": "feed",
		"KeepDays": s.st.FeedKeepDays(u.ID), "TZ": u.Location().String(),
		"BodyMax": store.FeedBodyMax, "ReportMax": store.FeedReportMax, "Kinds": s.eng.FeedMediaKinds(u.ID),
		"Agents": s.st.FeedAgentNames(u.ID), "BinCount": s.st.CountFeed(u.ID, store.FeedFilter{View: store.FeedViewBin}),
		"Empty": len(posts) == 0 && !st.Filtered() && st.View != store.FeedViewBin &&
			s.st.CountFeed(u.ID, store.FeedFilter{View: store.FeedViewMine}) == 0,
	}
	if len(posts) > 0 {
		data["Last"] = posts[len(posts)-1].ID
	}
	s.render(w, r, "feed", data)
}

// feedShown records that posts were put in front of the viewer: public posts
// of others get a read, and on the full, unfiltered feed the "looked at" mark moves up.
func (s *Server) feedShown(u *store.User, st feedState, posts []store.FeedPost, publicOn bool) {
	if len(posts) == 0 || st.View == store.FeedViewBin {
		return
	}
	s.st.MarkFeedRead(u.ID, posts)
	whole := st.View == store.FeedViewAll || (st.View == store.FeedViewMine && !publicOn)
	if whole && !st.Filtered() {
		s.st.SetFeedSeen(u.ID, posts[0].ID)
	}
}

// feedMore answers with the next older posts, and the marker that loads the ones after them.
func (s *Server) feedMore(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	publicOn := s.feedPublicOn()
	st := readFeedState(r, publicOn)
	f := st.filter()
	f.Limit, f.BeforeID = feedNextPage, formInt(r, "before", 0)
	if f.BeforeID <= 0 {
		s.errorPage(w, r, http.StatusBadRequest, "Which posts?")
		return
	}
	posts, more, err := s.st.ListFeed(u.ID, f)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.st.MarkFeedRead(u.ID, posts)
	data := map[string]any{"State": st, "Posts": s.viewFeedPosts(posts, u, -1, publicOn), "More": more}
	if len(posts) > 0 {
		data["Last"] = posts[len(posts)-1].ID
	}
	s.renderPartial(w, r, "feed", "feed-items", data)
}

// feedNew answers with the posts added after the newest one on the page.
func (s *Server) feedNew(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	publicOn := s.feedPublicOn()
	st := readFeedState(r, publicOn)
	f := st.filter()
	f.Limit, f.AfterID = feedNewMax, max(formInt(r, "after", 0), 0)
	if st.View == store.FeedViewBin {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	posts, _, err := s.st.ListFeed(u.ID, f)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	seen := s.st.FeedSeen(u.ID)
	s.feedShown(u, st, posts, publicOn)
	s.renderPartial(w, r, "feed", "feed-items", map[string]any{"State": st, "Posts": s.viewFeedPosts(posts, u, seen, publicOn)})
}

// feedCount says how many posts were added after the newest one on the page.
func (s *Server) feedCount(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	st := readFeedState(r, s.feedPublicOn())
	f := st.filter()
	f.AfterID = max(formInt(r, "after", 0), 0)
	n := 0
	if st.View != store.FeedViewBin {
		n = s.st.CountFeed(u.ID, f)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"n": n})
}

// feedEvents is the SSE stream of the Feed page: one small event each time a
// post is added to the viewer's own feed or shared with everyone.
func (s *Server) feedEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	mine, stopMine := s.hub.Subscribe(hub.FeedUser(currentUser(r).ID))
	defer stopMine()
	shared, stopShared := s.hub.Subscribe(hub.FeedPublic)
	defer stopShared()

	fmt.Fprint(w, "retry: 5000\n\n")
	flusher.Flush()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-mine:
			fmt.Fprintf(w, "data: %s\n\n", b)
		case b := <-shared:
			fmt.Fprintf(w, "data: %s\n\n", b)
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
		}
		flusher.Flush()
	}
}

// feedFail shows why a change was refused.
func (s *Server) feedFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.errorPage(w, r, http.StatusNotFound, "That post does not exist (any more).")
	case errors.Is(err, store.ErrFeedBody):
		s.errorPage(w, r, http.StatusBadRequest, "A post needs some text, or a picture.")
	default:
		msg := err.Error()
		s.errorPage(w, r, http.StatusBadRequest, "That did not work: "+strings.ToUpper(msg[:1])+msg[1:]+".")
	}
}

// ---- posts ----

func (s *Server) renderFeedCard(w http.ResponseWriter, r *http.Request, p *store.FeedPost) {
	s.renderPartial(w, r, "feed", "feed-card", map[string]any{"P": s.viewFeedPost(*p, currentUser(r), -1, s.feedPublicOn())})
}

// feedOwnPicture reads the picture a person adds to their post from their
// device: a photo, kept as a file, or an SVG drawing, kept as text. Neither is
// trusted: the bytes decide what it is, never the name or the type the browser
// sent, and both are only ever shown as an image (feedMedia, feedPicture).
func feedOwnPicture(r *http.Request, in *store.FeedPostInput) error {
	f, _, err := r.FormFile("picture")
	if err != nil {
		return nil // no picture with this post
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, store.FeedPhotoMax+1))
	if err != nil || len(data) > store.FeedPhotoMax {
		return store.ErrFeedPhoto
	}
	if len(data) == 0 {
		return nil
	}
	mime, _, _ := strings.Cut(http.DetectContentType(data), ";")
	if _, ok := store.AllowedImageTypes[mime]; ok {
		in.Photo = &store.FeedPhoto{Data: data, Mime: mime}
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			in.Photo.W, in.Photo.H = cfg.Width, cfg.Height
		}
		return nil
	}
	// A drawing: drawing programs put a prolog, a doctype or a comment before the <svg> element.
	i := bytes.Index(bytes.ToLower(data), []byte("<svg"))
	if i < 0 || !utf8.Valid(data) || len(data)-i > store.FeedSVGOwnMax {
		return store.ErrFeedPhoto
	}
	in.SVG = string(data[i:])
	return nil
}

func (s *Server) feedCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	in := store.FeedPostInput{Body: r.FormValue("body"), Report: r.FormValue("report"),
		Public: r.FormValue("visibility") == store.FeedPublic && s.feedPublicOn()}
	if err := feedOwnPicture(r, &in); err != nil {
		s.feedFail(w, r, err)
		return
	}
	// Optional: a picture, the post read aloud, or a video, made by an AI model.
	// A photo of their own takes that place: a post carries one of them.
	kind := r.FormValue("media")
	if in.Photo != nil {
		kind = ""
	}
	switch kind {
	case store.MediaImage, store.MediaVideo:
		in.MediaKind, in.MediaPrompt = kind, r.FormValue("media_prompt")
	case store.MediaVoice:
		in.MediaKind, in.MediaPrompt = kind, strings.TrimSpace(in.Body) // the post itself is read aloud
	}
	id, err := s.eng.CreateFeedPost(u.ID, in, store.FeedActor{Kind: store.FeedByUser}, 0)
	if err != nil {
		s.feedFail(w, r, err)
		return
	}
	p, err := s.st.FeedPostByID(u.ID, id)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
		return
	}
	s.renderFeedCard(w, r, p)
}

// loadFeedPost returns the post in the path if the current user owns it.
func (s *Server) loadFeedPost(w http.ResponseWriter, r *http.Request) (*store.FeedPost, bool) {
	p, err := s.st.FeedPostByID(currentUser(r).ID, pathID(r, "id"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "Post not found.")
		return nil, false
	}
	return p, true
}

// feedCard renders one post the viewer may see (also when editing is cancelled).
func (s *Server) feedCard(w http.ResponseWriter, r *http.Request) {
	p, err := s.st.FeedPostVisible(currentUser(r).ID, pathID(r, "id"), s.feedPublicOn())
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "Post not found.")
		return
	}
	s.renderFeedCard(w, r, p)
}

// feedEdit renders the edit form in place of the card.
func (s *Server) feedEdit(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadFeedPost(w, r)
	if !ok {
		return
	}
	if p.InBin() {
		s.errorPage(w, r, http.StatusBadRequest, "This post is in the bin. Restore it first.")
		return
	}
	s.renderPartial(w, r, "feed", "feed-edit", map[string]any{"P": p,
		"HasPicture": p.HasSVG || (p.MediaKind == store.MediaImage && p.MediaStatus == store.MediaReady), "BodyMax": store.FeedBodyMax, "ReportMax": store.FeedReportMax})
}

func (s *Server) feedUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadFeedPost(w, r)
	if !ok {
		return
	}
	if err := s.st.UpdateFeedPost(p.UserID, p.ID, r.FormValue("body"), r.FormValue("report")); err != nil {
		s.feedFail(w, r, err)
		return
	}
	s.feedAfter(w, r, p.ID)
}

// feedAfter answers a change with the post as it is now.
func (s *Server) feedAfter(w http.ResponseWriter, r *http.Request, id int64) {
	if !isHX(r) {
		back(w, r, "/feed")
		return
	}
	p, err := s.st.FeedPostByID(currentUser(r).ID, id)
	if err != nil {
		s.feedFail(w, r, err)
		return
	}
	s.renderFeedCard(w, r, p)
}

// feedShare shares a post with everyone on this server, or makes it private again.
func (s *Server) feedShare(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadFeedPost(w, r)
	if !ok {
		return
	}
	public := r.FormValue("public") == "1"
	if public && !s.feedPublicOn() {
		s.errorPage(w, r, http.StatusForbidden, "Sharing posts is turned off on this server.")
		return
	}
	if err := s.st.SetFeedPublic(p.UserID, p.ID, public); err != nil {
		s.feedFail(w, r, err)
		return
	}
	if public && !p.Public {
		s.hub.Publish(hub.FeedPublic, engine.EvFeed, map[string]any{"id": p.ID})
	}
	s.feedAfter(w, r, p.ID)
}

// feedKeep marks a post so it is never removed for being old, or takes the mark away.
func (s *Server) feedKeep(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadFeedPost(w, r)
	if !ok {
		return
	}
	if err := s.st.SetFeedKept(p.UserID, p.ID, r.FormValue("keep") == "1"); err != nil {
		s.feedFail(w, r, err)
		return
	}
	s.feedAfter(w, r, p.ID)
}

// feedKeepSave stores for how many days the user's posts are kept.
func (s *Server) feedKeepSave(w http.ResponseWriter, r *http.Request) {
	if err := s.st.SetFeedKeepDays(currentUser(r).ID, int(formInt(r, "keep_days", store.FeedKeepDefault))); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/feed/settings?done=keep#more", http.StatusSeeOther)
}

// feedUnshare lets an admin take someone's post out of the public feed. Its owner keeps it, private.
func (s *Server) feedUnshare(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	if err := s.st.UnshareFeedPost(id); err != nil {
		s.feedFail(w, r, err)
		return
	}
	s.audit(r, "feed.unshare", fmt.Sprint(id), "took a post out of the public feed")
	if !isHX(r) {
		back(w, r, "/feed")
		return
	}
	s.renderPartial(w, r, "feed", "feed-gone", map[string]any{"ID": id, "Text": "Taken out of the public feed. Its owner still has it, as a private post."})
}

// feedDelete moves a post to the bin and leaves a line with Undo in its place.
func (s *Server) feedDelete(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadFeedPost(w, r)
	if !ok {
		return
	}
	if err := s.st.DeleteFeedPost(p.UserID, p.ID); err != nil {
		s.feedFail(w, r, err)
		return
	}
	if !isHX(r) {
		back(w, r, "/feed")
		return
	}
	s.renderPartial(w, r, "feed", "feed-gone", map[string]any{"ID": p.ID, "Undo": true,
		"Text": fmt.Sprintf("The post is in the bin, where it stays for %d days.", store.FeedBinDays)})
}

func (s *Server) feedRestore(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadFeedPost(w, r)
	if !ok {
		return
	}
	if err := s.st.RestoreFeedPost(p.UserID, p.ID); err != nil {
		s.feedFail(w, r, err)
		return
	}
	if isHX(r) && r.FormValue("view") == store.FeedViewBin {
		s.renderPartial(w, r, "feed", "feed-gone", map[string]any{"ID": p.ID, "Text": "Restored. The post is back on your feed."})
		return
	}
	s.feedAfter(w, r, p.ID)
}

// feedPurge deletes a binned post for good; its card disappears.
func (s *Server) feedPurge(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadFeedPost(w, r)
	if !ok {
		return
	}
	if err := s.st.PurgeFeedPost(p.UserID, p.ID); err != nil {
		s.feedFail(w, r, err)
		return
	}
	if !isHX(r) {
		back(w, r, "/feed?view=bin")
	}
}

func (s *Server) feedBinEmpty(w http.ResponseWriter, r *http.Request) {
	if err := s.st.EmptyFeedBin(currentUser(r).ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/feed?view=bin", http.StatusSeeOther)
}

// feedPicture serves a post's picture. It was written by an AI, so it is only
// ever handed out as an image under a policy that lets it do nothing: no
// scripts, no requests, no forms, even when someone opens the address directly.
func (s *Server) feedPicture(w http.ResponseWriter, r *http.Request) {
	svg, err := s.st.FeedSVG(currentUser(r).ID, pathID(r, "id"), s.feedPublicOn())
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/svg+xml; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cache-Control", "private, max-age=3600")
	w.Write([]byte(svg))
}

// ---- who may post ----

// feedSwitch lets the AI in the user's chats post to their feed, or stops it.
func (s *Server) feedSwitch(w http.ResponseWriter, r *http.Request) {
	on := r.FormValue("enabled") == "1"
	if err := s.st.SetFeedEnabled(currentUser(r).ID, on); err != nil {
		s.serverError(w, r, err)
		return
	}
	done := "off"
	if on {
		done = "on"
	}
	http.Redirect(w, r, "/feed/settings?done="+done, http.StatusSeeOther)
}
