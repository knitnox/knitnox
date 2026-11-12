package store

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Attachment kinds.
const (
	KindImage = "image" // sent to vision models as image parts
	KindFile  = "file"  // document; its extracted text is sent inline
)

// Attachment is an uploaded file linked to a user message.
type Attachment struct {
	ID        int64
	ThreadID  int64
	MessageID int64
	FileName  string // name on disk inside UploadDir
	OrigName  string
	Mime      string
	Size      int64
	Kind      string
	Text      string // extracted text (files only)
}

func (a *Attachment) IsImage() bool { return a.Kind == KindImage }

// NewAttachment is a file already written to UploadDir, not yet in the DB.
type NewAttachment struct {
	FileName string
	OrigName string
	Mime     string
	Size     int64
	Kind     string
	Text     string
}

// AllowedImageTypes maps accepted image MIME types to file extensions.
var AllowedImageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// AllowedFileTypes maps accepted document MIME types to file extensions.
var AllowedFileTypes = map[string]string{
	"application/pdf":  ".pdf",
	"text/plain":       ".txt",
	"text/markdown":    ".md",
	"text/csv":         ".csv",
	"application/json": ".json",
	"text/html":        ".html",
	"text/xml":         ".xml",
}

// SaveUpload writes data to UploadDir under a random name.
func (s *Store) SaveUpload(data []byte, mime, origName, kind, text string) (NewAttachment, error) {
	ext, ok := AllowedImageTypes[mime]
	if !ok {
		ext, ok = AllowedFileTypes[mime]
	}
	if !ok && kind == KindAudio { // only recordings a voice model made, never an upload
		ext, ok = AllowedAudioTypes[mime]
	}
	if !ok {
		return NewAttachment{}, fmt.Errorf("unsupported file type %s", mime)
	}
	if kind == "" {
		if _, img := AllowedImageTypes[mime]; img {
			kind = KindImage
		} else {
			kind = KindFile
		}
	}
	name := randomToken()[:32] + ext
	if err := os.WriteFile(filepath.Join(s.UploadDir, name), data, 0o600); err != nil {
		return NewAttachment{}, err
	}
	return NewAttachment{FileName: name, OrigName: origName, Mime: mime, Size: int64(len(data)), Kind: kind, Text: text}, nil
}

// RemoveUploads deletes files that never made it into the database.
func (s *Store) RemoveUploads(list []NewAttachment) {
	for _, a := range list {
		os.Remove(filepath.Join(s.UploadDir, a.FileName))
	}
}

// AttachmentPath is the absolute path of a stored file.
func (s *Store) AttachmentPath(a *Attachment) string {
	return filepath.Join(s.UploadDir, filepath.Base(a.FileName))
}

// DataURL returns the file as a base64 data URL for vision models.
func (s *Store) DataURL(a *Attachment) (string, error) {
	b, err := os.ReadFile(s.AttachmentPath(a))
	if err != nil {
		return "", err
	}
	return "data:" + a.Mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

const attCols = `id, thread_id, message_id, file_name, orig_name, mime, size, kind, text`

func scanAttachment(sc interface{ Scan(...any) error }) (*Attachment, error) {
	a := &Attachment{}
	err := sc.Scan(&a.ID, &a.ThreadID, &a.MessageID, &a.FileName, &a.OrigName, &a.Mime, &a.Size, &a.Kind, &a.Text)
	return a, err
}

func (s *Store) AttachmentByID(id int64) (*Attachment, error) {
	a, err := scanAttachment(s.DB.QueryRow(`SELECT `+attCols+` FROM attachments WHERE id = ?`, id))
	return a, notFound(err)
}

// AttachmentsByMessage loads attachments for many messages at once.
func (s *Store) AttachmentsByMessage(messageIDs []int64) (map[int64][]Attachment, error) {
	out := map[int64][]Attachment{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(messageIDs))
	for i, id := range messageIDs {
		args[i] = id
	}
	rows, err := s.DB.Query(`SELECT `+attCols+` FROM attachments
		WHERE message_id IN (?`+strings.Repeat(",?", len(args)-1)+`) ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out[a.MessageID] = append(out[a.MessageID], *a)
	}
	return out, rows.Err()
}

// ThreadUploadFiles lists the files of a thread (to delete them with it).
func (s *Store) ThreadUploadFiles(threadID int64) []string {
	rows, err := s.DB.Query(`SELECT file_name FROM attachments WHERE thread_id = ?`, threadID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if rows.Scan(&f) == nil {
			out = append(out, f)
		}
	}
	return out
}

func (s *Store) RemoveFiles(names []string) {
	for _, n := range names {
		os.Remove(filepath.Join(s.UploadDir, filepath.Base(n)))
	}
}

// SweepOrphanUploads removes files older than an hour that no row references
// (left behind by cascades, e.g. deleted users or edited-away runs).
func (s *Store) SweepOrphanUploads() (int, error) {
	known := map[string]bool{}
	rows, err := s.DB.Query(`SELECT file_name FROM attachments`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var f string
		if rows.Scan(&f) == nil {
			known[f] = true
		}
	}
	rows.Close()
	entries, err := os.ReadDir(s.UploadDir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || known[e.Name()] {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
			if os.Remove(filepath.Join(s.UploadDir, e.Name())) == nil {
				n++
			}
		}
	}
	return n, nil
}
