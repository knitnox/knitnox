package store

import "errors"

// KindAudio is an answer read aloud by a voice model.
const KindAudio = "audio"

func (a *Attachment) IsAudio() bool { return a.Kind == KindAudio }

// AllowedAudioTypes maps the recording types kept to file extensions.
var AllowedAudioTypes = map[string]string{
	"audio/mpeg": ".mp3", "audio/mp3": ".mp3", "audio/wav": ".wav", "audio/x-wav": ".wav", "audio/wave": ".wav",
	"audio/ogg": ".ogg", "audio/opus": ".opus", "audio/aac": ".aac", "audio/flac": ".flac", "audio/mp4": ".m4a",
}

// AddMadeAttachment links a file an AI model made (already written with
// SaveUpload) to a message. shared says a model of the server made it, which
// counts toward the person's daily media limit.
func (s *Store) AddMadeAttachment(threadID, messageID, userID int64, a NewAttachment, shared bool) (int64, error) {
	made := 1
	if shared {
		made = 2
	}
	res, err := s.DB.Exec(`INSERT INTO attachments(thread_id, message_id, user_id, file_name, orig_name, mime, size, kind, text, made)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', ?)`, threadID, messageID, userID, a.FileName, a.OrigName, a.Mime, a.Size, a.Kind, made)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// MadeMediaSince counts the pictures and recordings made in chats for a user
// with the server's models since a time.
func (s *Store) MadeMediaSince(userID int64, since string) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM attachments WHERE user_id = ? AND made = 2 AND created_at >= ?`, userID, since).Scan(&n)
	return n
}

// MessageAudio returns the recording of a message, if one was made.
func (s *Store) MessageAudio(messageID int64) (*Attachment, error) {
	a, err := scanAttachment(s.DB.QueryRow(`SELECT `+attCols+` FROM attachments WHERE message_id = ? AND kind = ? ORDER BY id LIMIT 1`, messageID, KindAudio))
	return a, notFound(err)
}

// ThreadMessage returns one message of a thread.
func (s *Store) ThreadMessage(threadID, messageID int64) (*Message, error) {
	rows, err := s.DB.Query(msgSelect+` WHERE m.id = ? AND m.thread_id = ?`, messageID, threadID)
	if err != nil {
		return nil, err
	}
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, ErrNotFound
	}
	return &msgs[0], nil
}

// ErrNoMediaModel is returned when no model of the needed kind is set up for the user.
var ErrNoMediaModel = errors.New("no model is set up for this")
