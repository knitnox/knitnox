package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Feed: short posts in one place. A person, the AI in their chats and their
// agents (when allowed) post into the person's feed, so every post records who
// wrote it. A post is private unless its owner shares it with everyone on this
// server. Deleting moves a post to the bin, where it can be restored for
// FeedBinDays. Only the owner can change, share or delete a post.

// Limits and fixed values.
const (
	FeedBodyMax     = 500   // the short text of a post
	FeedReportMax   = 12000 // the longer report behind "read more"
	FeedSVGMax      = 60000
	FeedChartPoints = 24
	FeedBinDays     = 30
	FeedDailyMax    = 50 // most posts per day an agent can be allowed
	FeedDailyDef    = 5

	FeedByUser  = "user"
	FeedByAI    = "ai"
	FeedByAgent = "agent"

	FeedPrivate = "private"
	FeedPublic  = "public"

	FeedAccessNone = "none"
	FeedAccessPost = "post"

	FeedViewMine   = "mine"   // my own posts, private and public
	FeedViewPublic = "public" // what everyone shared
	FeedViewAll    = "all"    // both
	FeedViewBin    = "bin"    // my deleted posts
)

var (
	ErrFeedBody  = errors.New("a post needs some text")
	ErrFeedChart = fmt.Errorf("a chart needs a type (bar or line) and 1 to %d labels, each with one number", FeedChartPoints)
	ErrFeedSVG   = errors.New("the picture must be one <svg> … </svg> element")
)

// FeedEnabled reports whether the AI in this user's chats may post to their feed.
func (s *Store) FeedEnabled(userID int64) bool {
	var on bool
	s.DB.QueryRow(`SELECT ai_enabled FROM feed_settings WHERE user_id = ?`, userID).Scan(&on)
	return on
}

func (s *Store) SetFeedEnabled(userID int64, on bool) error {
	_, err := s.DB.Exec(`INSERT INTO feed_settings(user_id, ai_enabled) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET ai_enabled = excluded.ai_enabled`, userID, b2i(on))
	return err
}

// FeedSeen is the newest post id the user has looked at.
func (s *Store) FeedSeen(userID int64) int64 {
	var id int64
	s.DB.QueryRow(`SELECT seen_id FROM feed_settings WHERE user_id = ?`, userID).Scan(&id)
	return id
}

// SetFeedSeen moves the "looked at" mark forward (never back).
func (s *Store) SetFeedSeen(userID, postID int64) error {
	_, err := s.DB.Exec(`INSERT INTO feed_settings(user_id, seen_id) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET seen_id = MAX(seen_id, excluded.seen_id)`, userID, postID)
	return err
}

// FeedActor says who is posting.
type FeedActor struct {
	Kind     string // FeedByUser | FeedByAI | FeedByAgent
	Name     string // the agent's name
	AgentID  int64
	ThreadID int64  // chat the post came from
	Notice   string // FeedNoticeProblem | FeedNoticeRecovered: written by Knitnox about the agent, "" otherwise
}

// Posts Knitnox writes about an agent (feed_posts.notice).
const (
	FeedNoticeProblem   = "problem"   // the agent started having trouble
	FeedNoticeRecovered = "recovered" // it works again
)

// FeedChart is a small chart the server draws: bars or a line, one number per label.
type FeedChart struct {
	Type   string    `json:"type"` // bar | line
	Title  string    `json:"title,omitempty"`
	Unit   string    `json:"unit,omitempty"` // shown after each number, e.g. "%" or "kg"
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
}

// clean checks a chart and trims its texts.
func (c *FeedChart) clean() error {
	if c.Type != "bar" && c.Type != "line" {
		return ErrFeedChart
	}
	if len(c.Labels) == 0 || len(c.Labels) > FeedChartPoints || len(c.Labels) != len(c.Values) {
		return ErrFeedChart
	}
	for i, v := range c.Values {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e15 {
			return ErrFeedChart
		}
		c.Labels[i] = clip(strings.Join(strings.Fields(c.Labels[i]), " "), 40)
	}
	c.Title = clip(strings.Join(strings.Fields(c.Title), " "), 120)
	c.Unit = clip(strings.TrimSpace(c.Unit), 12)
	return nil
}

// cleanSVG accepts one svg element. It is only ever shown as an image (never
// put into a page), so scripts and links inside it cannot do anything.
func cleanSVG(svg string) (string, error) {
	svg = strings.TrimSpace(svg)
	if svg == "" {
		return "", nil
	}
	if strings.HasPrefix(svg, "<?xml") {
		if i := strings.Index(svg, "?>"); i > 0 {
			svg = strings.TrimSpace(svg[i+2:])
		}
	}
	if len(svg) > FeedSVGMax || !strings.HasPrefix(strings.ToLower(svg), "<svg") || !strings.HasSuffix(strings.ToLower(svg), "</svg>") {
		return "", ErrFeedSVG
	}
	if !strings.Contains(svg[:min(len(svg), 600)], "http://www.w3.org/2000/svg") { // browsers need this to show it as an image
		svg = "<svg xmlns=\"http://www.w3.org/2000/svg\"" + svg[4:]
	}
	return fitSVG(svg), nil
}

func cleanFeedBody(s string) string {
	return clip(strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")), FeedBodyMax)
}

// FeedPost is one post.
type FeedPost struct {
	ID             int64
	UserID         int64
	Username       string // the owner
	Body           string
	Report         string
	Chart          *FeedChart
	HasSVG         bool
	Public         bool
	CreatedBy      string
	ActorName      string
	AgentID        int64
	SourceThreadID int64
	SourceTitle    string
	Notice         string // "" | problem | recovered: Knitnox reporting on the agent, not the agent itself
	Reads          int    // other people who were shown this post (public posts)
	SVGW, SVGH     int    // the picture's own size, 0 when it does not say
	MediaKind      string // "" | image | voice | video: made by an AI model
	MediaStatus    string // pending | ready | failed
	MediaFile      string // name on disk in the feed media folder
	MediaMime      string
	MediaPrompt    string // what the model was asked for
	MediaNote      string // why it failed, or that the picture was redrawn after a check
	MediaW, MediaH int    // size of a picture, 0 when unknown
	EditedAt       string // set when the owner changed the text
	DeletedAt      string // set while in the bin
	CreatedAt      string
}

// InBin reports whether the post was deleted and can still be restored.
func (p FeedPost) InBin() bool { return p.DeletedAt != "" }

// Edited reports whether the post was changed after it was written.
func (p FeedPost) Edited() bool { return p.EditedAt != "" }

// FeedPostInput is a new post.
type FeedPostInput struct {
	Body   string
	Report string
	Chart  *FeedChart
	SVG    string
	Public bool
	// Media to be made by an AI model after the post is written (see engine/feedmedia.go).
	MediaKind   string // "" | image | voice | video
	MediaPrompt string
	MediaShared bool // made with a server model
}

// CreateFeedPost adds a post to userID's feed.
func (s *Store) CreateFeedPost(userID int64, in FeedPostInput, by FeedActor) (int64, error) {
	body := cleanFeedBody(in.Body)
	if body == "" {
		return 0, ErrFeedBody
	}
	svg, err := cleanSVG(in.SVG)
	if err != nil {
		return 0, err
	}
	chart := ""
	if in.Chart != nil {
		if err := in.Chart.clean(); err != nil {
			return 0, err
		}
		b, _ := json.Marshal(in.Chart)
		chart = string(b)
	}
	kind := by.Kind
	if kind != FeedByAI && kind != FeedByAgent {
		kind = FeedByUser
	}
	visibility := FeedPrivate
	if in.Public {
		visibility = FeedPublic
	}
	var agent, thread any
	if by.AgentID > 0 {
		agent = by.AgentID
	}
	if by.ThreadID > 0 {
		thread = by.ThreadID
	}
	notice := ""
	if kind == FeedByAgent && (by.Notice == FeedNoticeProblem || by.Notice == FeedNoticeRecovered) {
		notice = by.Notice
	}
	mediaStatus := ""
	switch in.MediaKind {
	case MediaImage, MediaVoice, MediaVideo:
		mediaStatus = MediaPending
	default:
		in.MediaKind, in.MediaPrompt = "", ""
	}
	res, err := s.DB.Exec(`INSERT INTO feed_posts(user_id, body, report, chart, svg, visibility, created_by, actor_name, agent_id, source_thread_id,
		media_kind, media_status, media_prompt, media_shared, notice) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, userID, body, clip(strings.TrimSpace(in.Report), FeedReportMax), chart, svg,
		visibility, kind, clip(by.Name, 60), agent, thread, in.MediaKind, mediaStatus, clip(strings.TrimSpace(in.MediaPrompt), FeedMediaPromptMax), b2i(in.MediaShared), notice)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateFeedPost replaces the text and the report of a post the user owns.
func (s *Store) UpdateFeedPost(userID, id int64, body, report string) error {
	body = cleanFeedBody(body)
	if body == "" {
		return ErrFeedBody
	}
	res, err := s.DB.Exec(`UPDATE feed_posts SET body = ?, report = ?, edited_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		body, clip(strings.TrimSpace(report), FeedReportMax), Now(), id, userID)
	return affected(res, err)
}

// SetFeedPublic shares a post with everyone on this server, or makes it private again.
func (s *Store) SetFeedPublic(userID, id int64, public bool) error {
	visibility := FeedPrivate
	if public {
		visibility = FeedPublic
	}
	res, err := s.DB.Exec(`UPDATE feed_posts SET visibility = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`, visibility, id, userID)
	return affected(res, err)
}

// UnshareFeedPost makes any public post private again (for admins). The owner keeps it.
func (s *Store) UnshareFeedPost(id int64) error {
	res, err := s.DB.Exec(`UPDATE feed_posts SET visibility = 'private' WHERE id = ? AND visibility = 'public'`, id)
	return affected(res, err)
}

// DeleteFeedPost moves a post to the bin.
func (s *Store) DeleteFeedPost(userID, id int64) error {
	res, err := s.DB.Exec(`UPDATE feed_posts SET deleted_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`, Now(), id, userID)
	return affected(res, err)
}

// RestoreFeedPost takes a post out of the bin.
func (s *Store) RestoreFeedPost(userID, id int64) error {
	res, err := s.DB.Exec(`UPDATE feed_posts SET deleted_at = NULL WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL`, id, userID)
	return affected(res, err)
}

// PurgeFeedPost deletes a binned post for good.
func (s *Store) PurgeFeedPost(userID, id int64) error {
	res, err := s.DB.Exec(`DELETE FROM feed_posts WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL`, id, userID)
	return affected(res, err)
}

// EmptyFeedBin deletes everything in a user's bin for good.
func (s *Store) EmptyFeedBin(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM feed_posts WHERE user_id = ? AND deleted_at IS NOT NULL`, userID)
	return err
}

// PurgeOldFeedBin removes posts that sat in the bin longer than FeedBinDays (janitor).
func (s *Store) PurgeOldFeedBin() (int64, error) {
	res, err := s.DB.Exec(`DELETE FROM feed_posts WHERE deleted_at IS NOT NULL AND deleted_at < ?`,
		FormatTime(time.Now().AddDate(0, 0, -FeedBinDays)))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

const feedCols = `p.id, p.user_id, u.username, p.body, p.report, p.chart, substr(p.svg, 1, 700), p.visibility = 'public', p.created_by, p.actor_name,
	COALESCE(p.agent_id, 0), COALESCE(p.source_thread_id, 0), COALESCE(th.title, ''),
	(SELECT COUNT(*) FROM feed_reads r WHERE r.post_id = p.id), COALESCE(p.edited_at, ''), COALESCE(p.deleted_at, ''), p.created_at,
	p.media_kind, p.media_status, p.media_file, p.media_mime, p.media_prompt, p.media_note, p.media_w, p.media_h, p.notice`
const feedFrom = ` FROM feed_posts p JOIN users u ON u.id = p.user_id LEFT JOIN threads th ON th.id = p.source_thread_id `

func scanFeed(rows *sql.Rows) ([]FeedPost, error) {
	defer rows.Close()
	var out []FeedPost
	for rows.Next() {
		var p FeedPost
		var chart, svgHead string
		if err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.Body, &p.Report, &chart, &svgHead, &p.Public, &p.CreatedBy, &p.ActorName,
			&p.AgentID, &p.SourceThreadID, &p.SourceTitle, &p.Reads, &p.EditedAt, &p.DeletedAt, &p.CreatedAt,
			&p.MediaKind, &p.MediaStatus, &p.MediaFile, &p.MediaMime, &p.MediaPrompt, &p.MediaNote, &p.MediaW, &p.MediaH, &p.Notice); err != nil {
			return nil, err
		}
		if svgHead != "" {
			p.HasSVG = true
			p.SVGW, p.SVGH = svgSize(svgHead)
		}
		if chart != "" {
			c := &FeedChart{}
			if json.Unmarshal([]byte(chart), c) == nil && c.clean() == nil {
				p.Chart = c
			}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) feedOne(where string, args ...any) (*FeedPost, error) {
	rows, err := s.DB.Query(`SELECT `+feedCols+feedFrom+`WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	list, err := scanFeed(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

// FeedPostByID returns a post the user owns (also when it is in the bin).
func (s *Store) FeedPostByID(userID, id int64) (*FeedPost, error) {
	return s.feedOne(`p.id = ? AND p.user_id = ?`, id, userID)
}

// FeedPostVisible returns a post the viewer may see: their own, or a public
// one that is not in the bin (when public posts are allowed).
func (s *Store) FeedPostVisible(viewerID, id int64, publicOn bool) (*FeedPost, error) {
	if !publicOn {
		return s.FeedPostByID(viewerID, id)
	}
	return s.feedOne(`p.id = ? AND (p.user_id = ? OR (p.visibility = 'public' AND p.deleted_at IS NULL))`, id, viewerID)
}

// FeedSVG returns the picture of a post the viewer may see.
func (s *Store) FeedSVG(viewerID, id int64, publicOn bool) (string, error) {
	if _, err := s.FeedPostVisible(viewerID, id, publicOn); err != nil {
		return "", err
	}
	var svg string
	err := s.DB.QueryRow(`SELECT svg FROM feed_posts WHERE id = ?`, id).Scan(&svg)
	if err == nil && svg == "" {
		return "", ErrNotFound
	}
	return svg, notFound(err)
}

// FeedFilter selects posts for one viewer. Posts are paged by id, newest first.
type FeedFilter struct {
	View     string // FeedViewMine | FeedViewPublic | FeedViewAll | FeedViewBin
	From     string // "" = anyone; FeedByUser | FeedByAI | FeedByAgent
	Agent    string // only posts by the agent with this name
	Query    string // only posts whose text or report contains this
	BeforeID int64  // only posts older than this one (0 = from the newest)
	AfterID  int64  // only posts newer than this one
	Limit    int
}

func (f FeedFilter) where(viewerID int64) (string, []any) {
	var where string
	args := []any{viewerID}
	switch f.View {
	case FeedViewBin:
		where = `p.user_id = ? AND p.deleted_at IS NOT NULL`
	case FeedViewPublic:
		where, args = `p.visibility = 'public' AND p.deleted_at IS NULL`, nil
	case FeedViewAll:
		where = `(p.user_id = ? OR p.visibility = 'public') AND p.deleted_at IS NULL`
	default:
		where = `p.user_id = ? AND p.deleted_at IS NULL`
	}
	switch f.From {
	case FeedByUser, FeedByAI, FeedByAgent:
		where += ` AND p.created_by = ?`
		args = append(args, f.From)
	}
	if f.Agent != "" {
		where += ` AND p.created_by = 'agent' AND p.actor_name = ?`
		args = append(args, f.Agent)
	}
	if q := strings.ToLower(strings.TrimSpace(f.Query)); q != "" {
		where += ` AND (lower(p.body) LIKE ? ESCAPE '\' OR lower(p.report) LIKE ? ESCAPE '\')`
		like := "%" + likeEscaper.Replace(q) + "%"
		args = append(args, like, like)
	}
	if f.BeforeID > 0 {
		where += ` AND p.id < ?`
		args = append(args, f.BeforeID)
	}
	if f.AfterID > 0 {
		where += ` AND p.id > ?`
		args = append(args, f.AfterID)
	}
	return where, args
}

// ListFeed returns up to Limit posts, newest first, and whether older ones remain.
func (s *Store) ListFeed(viewerID int64, f FeedFilter) (posts []FeedPost, more bool, err error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	where, args := f.where(viewerID)
	rows, err := s.DB.Query(`SELECT `+feedCols+feedFrom+`WHERE `+where+` ORDER BY p.id DESC LIMIT ?`, append(args, limit+1)...)
	if err != nil {
		return nil, false, err
	}
	posts, err = scanFeed(rows)
	if err != nil {
		return nil, false, err
	}
	if len(posts) > limit {
		posts, more = posts[:limit], true
	}
	return posts, more, nil
}

// CountFeed says how many posts match a filter.
func (s *Store) CountFeed(viewerID int64, f FeedFilter) int {
	where, args := f.where(viewerID)
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM feed_posts p WHERE `+where, args...).Scan(&n)
	return n
}

// FeedUnread counts posts the user has not looked at yet: newer than their
// mark and not typed by themselves. Public posts of others count when publicOn.
func (s *Store) FeedUnread(userID int64, publicOn bool) int {
	scope := `p.user_id = ?`
	if publicOn {
		scope = `(p.user_id = ? OR p.visibility = 'public')`
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM feed_posts p WHERE `+scope+` AND p.deleted_at IS NULL
		AND p.id > COALESCE((SELECT seen_id FROM feed_settings WHERE user_id = ?), 0)
		AND NOT (p.user_id = ? AND p.created_by = 'user')`, userID, userID, userID).Scan(&n)
	return n
}

// MarkFeedRead records that the viewer was shown these posts. Only public
// posts of other people count, and each person counts once.
func (s *Store) MarkFeedRead(viewerID int64, posts []FeedPost) {
	for _, p := range posts {
		if p.Public && p.UserID != viewerID && !p.InBin() {
			s.DB.Exec(`INSERT OR IGNORE INTO feed_reads(post_id, user_id) VALUES (?, ?)`, p.ID, viewerID)
		}
	}
}

// FeedAgentNames lists the agents that have posts in the user's feed, by name.
func (s *Store) FeedAgentNames(userID int64) []string {
	rows, err := s.DB.Query(`SELECT DISTINCT actor_name FROM feed_posts
		WHERE user_id = ? AND created_by = 'agent' AND actor_name != '' AND deleted_at IS NULL ORDER BY actor_name LIMIT 40`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			out = append(out, name)
		}
	}
	return out
}

// FeedAgentPostsSince counts the posts an agent added since a moment (UTC text
// time), including ones that were deleted since: the daily limit is about how
// much it writes.
func (s *Store) FeedAgentPostsSince(agentID int64, since string) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM feed_posts WHERE agent_id = ? AND created_at >= ? AND notice = ''`, agentID, since).Scan(&n)
	return n
}

// SetAgentFeedAccess sets whether an agent may post to its owner's feed, and how often per day.
func (s *Store) SetAgentFeedAccess(agentID int64, level string, daily int) error {
	if level != FeedAccessPost {
		level = FeedAccessNone
	}
	_, err := s.DB.Exec(`UPDATE agents SET feed_access = ?, feed_daily = ? WHERE id = ?`, level, min(max(daily, 1), FeedDailyMax), agentID)
	return err
}
