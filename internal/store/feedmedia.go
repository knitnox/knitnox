package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// What a feed post can carry besides text (see feed.go for the posts
// themselves): a picture drawn as SVG, and one piece of media made by an AI
// model: a picture, a voice recording or a video.

const (
	FeedMediaPromptMax = 2000
	FeedMediaMaxBytes  = 80 << 20 // largest picture, voice or video file kept

	MediaImage = "image"
	MediaVoice = "voice"
	MediaVideo = "video"

	MediaPending = "pending"
	MediaReady   = "ready"
	MediaFailed  = "failed"
)

// ---- the picture a post carries (drawn as SVG) ----

var (
	svgAttr    = regexp.MustCompile(`(?i)\s(width|height|viewBox)\s*=\s*["']([^"']*)["']`)
	svgRootTag = regexp.MustCompile(`(?is)<svg\b[^>]*>`)
)

// svgSize reads the size an svg element gives itself: width and height, or
// else its viewBox. 0, 0 when it gives neither in plain numbers.
func svgSize(head string) (w, h int) {
	root := svgRootTag.FindString(head)
	num := func(v string) float64 {
		f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), "px"), 64)
		if err != nil || f <= 0 || f > 100000 {
			return 0
		}
		return f
	}
	var fw, fh float64
	var box []float64
	for _, m := range svgAttr.FindAllStringSubmatch(root, -1) {
		switch strings.ToLower(m[1]) {
		case "width":
			fw = num(m[2])
		case "height":
			fh = num(m[2])
		case "viewbox":
			for _, part := range strings.FieldsFunc(m[2], func(r rune) bool { return r == ' ' || r == ',' }) {
				f, _ := strconv.ParseFloat(part, 64)
				box = append(box, f)
			}
		}
	}
	if (fw == 0 || fh == 0) && len(box) == 4 && box[2] > 0 && box[3] > 0 {
		fw, fh = box[2], box[3]
	}
	if fw == 0 || fh == 0 {
		return 0, 0
	}
	return int(fw + 0.5), int(fh + 0.5)
}

// fitSVG makes a picture scale as one piece: an svg that gives a width and a
// height but no viewBox would otherwise be cut off or blown up when it is
// shown at another size.
func fitSVG(svg string) string {
	root := svgRootTag.FindString(svg)
	if root == "" || strings.Contains(strings.ToLower(root), "viewbox") {
		return svg
	}
	if w, h := svgSize(root); w > 0 && h > 0 {
		end := len(root) - 1
		if strings.HasSuffix(root, "/>") {
			end--
		}
		return strings.Replace(svg, root, root[:end]+fmt.Sprintf(` viewBox="0 0 %d %d"`, w, h)+root[end:], 1)
	}
	return svg
}

// ---- pictures, voice and video made by AI models ----

// FeedModels says which models make the media for a user's feed.
// 0 = the server's model, -1 = off, otherwise a model id.
type FeedModels struct {
	Image, Voice, Video int64
	VoiceName           string // the voice to speak with, "" = the one set on the model, or the model's usual one
}

// VoiceNameFor is the voice a recording for userID is spoken with by model m:
// the name the person chose in their Feed settings, else the one set on the
// model, else "" (the model's usual voice).
func (s *Store) VoiceNameFor(userID int64, m *Model) string {
	if own := s.FeedModelsFor(userID).VoiceName; own != "" {
		return own
	}
	return m.VoiceName
}

func (s *Store) FeedModelsFor(userID int64) FeedModels {
	var m FeedModels
	s.DB.QueryRow(`SELECT image_model_id, voice_model_id, video_model_id, voice_name FROM feed_settings WHERE user_id = ?`, userID).
		Scan(&m.Image, &m.Voice, &m.Video, &m.VoiceName)
	return m
}

func (s *Store) SaveFeedModels(userID int64, m FeedModels) error {
	_, err := s.DB.Exec(`INSERT INTO feed_settings(user_id, image_model_id, voice_model_id, video_model_id, voice_name) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET image_model_id = excluded.image_model_id, voice_model_id = excluded.voice_model_id,
		video_model_id = excluded.video_model_id, voice_name = excluded.voice_name`,
		userID, m.Image, m.Voice, m.Video, clip(strings.TrimSpace(m.VoiceName), 60))
	return err
}

// FeedMediaModel returns the model that makes media of one kind for a user,
// with its provider. Nobody has to choose anything first:
//
//	the model they picked on the Feed settings page, if it still exists;
//	else the first model of that kind the admin shares with everyone;
//	else the first one on their own providers.
//
// nil, nil, nil = none (they turned that kind off, or nothing is set up).
func (s *Store) FeedMediaModel(userID int64, kind string) (*Model, *Provider, error) {
	fm := s.FeedModelsFor(userID)
	choice := map[string]int64{MediaImage: fm.Image, MediaVoice: fm.Voice, MediaVideo: fm.Video}[kind]
	if choice < 0 {
		return nil, nil, nil
	}
	opts, err := s.KindModelOptions(userID, kind)
	if err != nil {
		return nil, nil, err
	}
	var id, own int64
	for _, o := range opts {
		if o.ModelID == choice {
			id = o.ModelID
			break
		}
		if id == 0 && !o.Personal {
			id = o.ModelID // the server's model, unless the chosen one turns up
		}
		if own == 0 && o.Personal {
			own = o.ModelID
		}
	}
	if id == 0 {
		id = own // nothing shared: a model on their own provider works without being picked
	}
	if id == 0 {
		return nil, nil, nil
	}
	m, err := s.ModelByID(id)
	if err != nil {
		return nil, nil, err
	}
	p, err := s.ProviderByID(m.ProviderID)
	if err != nil {
		return nil, nil, err
	}
	return m, p, nil
}

// FeedMediaSince counts the media a user had made with server models since a moment (UTC text time).
func (s *Store) FeedMediaSince(userID int64, since string) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM feed_posts WHERE user_id = ? AND media_kind != '' AND media_shared = 1 AND created_at >= ?`, userID, since).Scan(&n)
	return n
}

// FeedMediaDir is the folder that holds the files of pictures, voice and video.
func (s *Store) FeedMediaDir() string { return filepath.Join(filepath.Dir(s.UploadDir), "feedmedia") }

// FeedMediaPath is the file of a post's media.
func (s *Store) FeedMediaPath(p *FeedPost) string {
	return filepath.Join(s.FeedMediaDir(), filepath.Base(p.MediaFile))
}

// writeFeedMedia writes a file into the feed media folder under a random name.
func (s *Store) writeFeedMedia(data []byte, ext string) (string, error) {
	if err := os.MkdirAll(s.FeedMediaDir(), 0o700); err != nil {
		return "", err
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	name := hex.EncodeToString(buf) + ext
	if err := os.WriteFile(filepath.Join(s.FeedMediaDir(), name), data, 0o600); err != nil {
		return "", err
	}
	return name, nil
}

// SaveFeedMedia writes the finished file and marks the post's media as ready.
func (s *Store) SaveFeedMedia(postID int64, data []byte, mime, ext, prompt, note string, w, h int) error {
	if len(data) == 0 || len(data) > FeedMediaMaxBytes {
		return fmt.Errorf("the file has %d bytes; at most %d are kept", len(data), FeedMediaMaxBytes)
	}
	name, err := s.writeFeedMedia(data, ext)
	if err != nil {
		return err
	}
	res, err := s.DB.Exec(`UPDATE feed_posts SET media_status = 'ready', media_file = ?, media_mime = ?, media_prompt = ?, media_note = ?, media_w = ?, media_h = ?
		WHERE id = ? AND media_status = 'pending'`, name, mime, clip(prompt, FeedMediaPromptMax), clip(note, 400), w, h, postID)
	if err := affected(res, err); err != nil {
		os.Remove(filepath.Join(s.FeedMediaDir(), name)) // the post is gone meanwhile
		return err
	}
	return nil
}

// FailFeedMedia records that a post's media could not be made.
func (s *Store) FailFeedMedia(postID int64, why string) error {
	_, err := s.DB.Exec(`UPDATE feed_posts SET media_status = 'failed', media_note = ? WHERE id = ? AND media_status = 'pending'`, clip(why, 400), postID)
	return err
}

// FailPendingFeedMedia marks media that was still being made when the server stopped.
func (s *Store) FailPendingFeedMedia() error {
	_, err := s.DB.Exec(`UPDATE feed_posts SET media_status = 'failed', media_note = 'The server was restarted while this was being made.' WHERE media_status = 'pending'`)
	return err
}

// SweepFeedMedia deletes media files that no post points to any more (janitor).
func (s *Store) SweepFeedMedia() (int, error) {
	entries, err := os.ReadDir(s.FeedMediaDir())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	rows, err := s.DB.Query(`SELECT media_file FROM feed_posts WHERE media_file != ''`)
	if err != nil {
		return 0, err
	}
	used := map[string]bool{}
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			used[name] = true
		}
	}
	rows.Close()
	n := 0
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() || used[e.Name()] || time.Since(info.ModTime()) < time.Hour { // a fresh file may not be recorded yet
			continue
		}
		if os.Remove(filepath.Join(s.FeedMediaDir(), e.Name())) == nil {
			n++
		}
	}
	return n, nil
}
