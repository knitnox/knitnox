package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"knitnox/internal/llm"
	"knitnox/internal/store"
)

// Pictures and voice inside a chat
//
// Two things use the picture and voice models outside the Feed:
//
//   - make_picture: a built-in tool for the AI in a person's own chat. The
//     picture is saved like an upload and linked to the AI's message, so it is
//     shown in the conversation, kept with the chat, backed up with it and
//     deleted with it.
//   - Read aloud: a person asks for one answer to be spoken. The recording is
//     made once and kept on the message; everyone in the chat can play it.
//
// Both use the models chosen on the Feed settings page ("Pictures, voice and
// video") and share the Feed's daily limit for the server's models.

const (
	chatPictureTimeout = 4 * time.Minute
	chatSpeakTimeout   = 3 * time.Minute
	speakMaxChars      = 4000 // what one speech request takes; longer answers are read up to here
)

// mediaLimitLeft reports an error when the user has used up today's
// pictures, recordings and videos made with the server's models.
func (e *Engine) mediaLimitLeft(userID int64) error {
	st, err := e.st.LoadSettings()
	if err != nil || st.FeedMediaDaily <= 0 {
		return nil
	}
	since := feedDayStart(e.st.MustLocation(userID))
	if used := e.st.FeedMediaSince(userID, since) + e.st.MadeMediaSince(userID, since); used >= st.FeedMediaDaily {
		return fmt.Errorf("today's limit of %d pictures, recordings and videos made with the server's models is used up; it starts again tomorrow", st.FeedMediaDaily)
	}
	return nil
}

// providerWords turns a provider's refusal into one readable line.
func providerWords(err error) string {
	var he *llm.HTTPError
	if !errors.As(err, &he) {
		return oneLine(err.Error(), 300)
	}
	said := he.Body
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(he.Body), &body) == nil && body.Error.Message != "" {
		said = body.Error.Message
	}
	return fmt.Sprintf("the provider said (%d): %s", he.Status, oneLine(said, 200))
}

// CanMakeInChat reports which chat media a user can have made right now.
func (e *Engine) CanMakeInChat(userID int64) (picture, voice bool) {
	kinds := e.FeedMediaKinds(userID)
	return kinds[store.MediaImage], kinds[store.MediaVoice]
}

const pictureNote = "\n\nYou can make pictures with the make_picture tool. Use it when the user asks for a picture, drawing, illustration or photo. " +
	"The picture is shown to the user in the chat by itself; you do not get it back and cannot see it, so do not describe details you did not ask for."

// addPictureTool gives the AI in a person's own chat a tool that makes a
// picture with that person's picture model. It reports whether it was added.
func (e *Engine) addPictureTool(ts *toolset, tc *turnCtx) bool {
	owner := tc.thread.OwnerUserID
	if m, _, err := e.st.FeedMediaModel(owner, store.MediaImage); err != nil || m == nil {
		return false
	}
	ts.addLocal("make_picture", "Make one picture with an image model and show it to the user in this chat. It takes up to a minute.",
		`{"type":"object","properties":{"description":{"type":"string","description":"What the picture shows: subject, setting, style. Plain and concrete, under 800 characters."}},"required":["description"]}`,
		func(ctx context.Context, raw json.RawMessage) string {
			var in struct {
				Description string `json:"description"`
			}
			if msg := parseArgs(raw, &in); msg != "" {
				return msg
			}
			prompt := clipText(strings.TrimSpace(in.Description), 2000)
			if prompt == "" {
				return "Error: say what the picture should show."
			}
			if tc.callMsgID == 0 {
				return "Error: the picture could not be placed in the chat."
			}
			m, p, err := e.st.FeedMediaModel(owner, store.MediaImage) // again: it may have been changed since the run began
			if err != nil || m == nil {
				return "Error: no picture model is set up any more. The user can choose one under Feed → More settings → Pictures, voice and video."
			}
			shared := p.IsGlobal()
			if shared {
				if err := e.mediaLimitLeft(owner); err != nil {
					return "Error: " + err.Error() + ". Tell the user."
				}
			}
			cctx, cancel := context.WithTimeout(ctx, chatPictureTimeout)
			defer cancel()
			media, err := llm.GenerateImage(cctx, llm.MediaRequest{BaseURL: p.BaseURL, APIKey: p.APIKey, Model: m.Name, Prompt: prompt, UserURL: !shared})
			if err != nil {
				if ctx.Err() != nil {
					return "Error: cancelled"
				}
				e.log.Warn("chat picture failed", "thread", tc.thread.ID, "err", err)
				return "Error: the picture could not be made: " + providerWords(err) + ". Tell the user; do not try again with the same description."
			}
			if _, ok := store.AllowedImageTypes[media.Mime]; !ok {
				return fmt.Sprintf("Error: the provider sent a file of type %q, which is not a picture.", oneLine(media.Mime, 60))
			}
			att, err := e.st.SaveUpload(media.Data, media.Mime, "Picture: "+oneLine(prompt, 80), store.KindImage, "")
			if err != nil {
				return "Error: the picture could not be saved: " + err.Error()
			}
			if _, err := e.st.AddMadeAttachment(tc.thread.ID, tc.callMsgID, owner, att, shared); err != nil {
				e.st.RemoveUploads([]store.NewAttachment{att})
				return "Error: the picture could not be saved: " + err.Error()
			}
			e.hub.Publish(tc.thread.ID, EvMessages, nil)
			return "The picture was made and is now shown to the user in this chat, above your answer. You cannot see it. " +
				"Say in one short sentence that it is there; do not paste a link and do not repeat the description."
		})
	return true
}

var speaking sync.Map // message id -> true while its recording is being made

// ErrSpeaking is returned when a recording of the message is already being made.
var ErrSpeaking = errors.New("this answer is already being read; it will be ready in a moment")

var (
	mdFence = regexp.MustCompile("(?s)```.*?(```|$)")
	mdLink  = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	mdMarks = regexp.MustCompile("[*_#>`|~]+")
	mdRule  = regexp.MustCompile(`(?m)^[\s|:=-]*[-=]{3,}[\s|:=-]*$`) // rules, and the line under a table's head
)

// speakable turns an answer into text that sounds right when read: code is
// left out, links keep only their words, and Markdown signs are dropped.
func speakable(md string) string {
	s := mdFence.ReplaceAllString(md, " (a code example is left out) ")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdRule.ReplaceAllString(s, "")
	s = mdMarks.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > speakMaxChars {
		cut := string(r[:speakMaxChars])
		if i := strings.LastIndexAny(cut, ".!?"); i > speakMaxChars/2 {
			cut = cut[:i+1]
		}
		s = cut + " The rest of the answer is not read aloud."
	}
	return s
}

// SpeakMessage returns the recording of an AI answer, making it with the
// asking user's voice model when there is none yet.
func (e *Engine) SpeakMessage(ctx context.Context, userID int64, msg *store.Message) (*store.Attachment, error) {
	if a, err := e.st.MessageAudio(msg.ID); err == nil {
		return a, nil
	}
	text := speakable(msg.Content)
	if text == "" {
		return nil, errors.New("there is nothing to read in this answer")
	}
	m, p, err := e.st.FeedMediaModel(userID, store.MediaVoice)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, store.ErrNoMediaModel
	}
	shared := p.IsGlobal()
	if shared {
		if err := e.mediaLimitLeft(userID); err != nil {
			return nil, err
		}
	}
	if _, busy := speaking.LoadOrStore(msg.ID, true); busy {
		return nil, ErrSpeaking
	}
	defer speaking.Delete(msg.ID)

	cctx, cancel := context.WithTimeout(ctx, chatSpeakTimeout)
	defer cancel()
	media, err := llm.Speak(cctx, llm.MediaRequest{BaseURL: p.BaseURL, APIKey: p.APIKey, Model: m.Name, Prompt: text,
		Voice: e.st.FeedModelsFor(userID).VoiceName, UserURL: !shared})
	if err != nil {
		e.log.Warn("read aloud failed", "message", msg.ID, "err", err)
		return nil, errors.New("the recording could not be made: " + providerWords(err))
	}
	if _, ok := store.AllowedAudioTypes[media.Mime]; !ok {
		return nil, fmt.Errorf("the provider sent a file of type %q, which is not a recording", oneLine(media.Mime, 60))
	}
	att, err := e.st.SaveUpload(media.Data, media.Mime, "Read aloud", store.KindAudio, "")
	if err != nil {
		return nil, err
	}
	if _, err := e.st.AddMadeAttachment(msg.ThreadID, msg.ID, userID, att, shared); err != nil {
		e.st.RemoveUploads([]store.NewAttachment{att})
		return nil, err
	}
	return e.st.MessageAudio(msg.ID)
}
