package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"knitnox/internal/engine"
	"knitnox/internal/files"
	"knitnox/internal/store"
)

const (
	maxMessageLen   = 100_000
	messagePageSize = 200  // messages per page (initial + each "load earlier")
	messageMaxLoad  = 3000 // hard cap for a refreshed page that loaded history
	maxAttachments  = 6
	maxImageBytes   = 10 << 20
	maxFileBytes    = 20 << 20
)

func templateEscape(s string) string { return template.HTMLEscapeString(s) }

// isFetch marks requests made by app.js, which want plain-text errors.
func isFetch(r *http.Request) bool { return r.Header.Get("X-Fetch") == "1" }

// loadThread returns the thread if the current user is a member.
func (s *Server) loadThread(w http.ResponseWriter, r *http.Request) (*store.Thread, string, bool) {
	t, err := s.st.ThreadByID(pathID(r, "id"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "Chat not found.")
		return nil, "", false
	}
	role := s.st.MemberRole(t.ID, currentUser(r).ID)
	if role == "" {
		s.errorPage(w, r, http.StatusNotFound, "Chat not found.")
		return nil, "", false
	}
	return t, role, true
}

func (s *Server) loadOwnedThread(w http.ResponseWriter, r *http.Request) (*store.Thread, bool) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return nil, false
	}
	if role != "owner" {
		s.errorPage(w, r, http.StatusForbidden, "Only the chat owner can do that.")
		return nil, false
	}
	return t, true
}

// defaultModel picks the model of the user's latest chat, else the first option.
func (s *Server) defaultModel(userID int64, opts []store.ModelOption) int64 {
	if len(opts) == 0 {
		return 0
	}
	var last int64
	s.st.DB.QueryRow(`SELECT COALESCE(model_id, 0) FROM threads WHERE owner_user_id = ? ORDER BY updated_at DESC LIMIT 1`, userID).Scan(&last)
	for _, o := range opts {
		if o.ModelID == last {
			return last
		}
	}
	return opts[0].ModelID
}

func findOption(opts []store.ModelOption, id int64) *store.ModelOption {
	for i := range opts {
		if opts[i].ModelID == id {
			return &opts[i]
		}
	}
	return nil
}

// readUploads validates and stores the "files" parts of a multipart request
// (images and documents). vision=false rejects images. The caller must link
// them to a message or call RemoveUploads.
func (s *Server) readUploads(r *http.Request, vision bool) ([]store.NewAttachment, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	parts := append(r.MultipartForm.File["files"], r.MultipartForm.File["images"]...)
	if len(parts) == 0 {
		return nil, nil
	}
	if len(parts) > maxAttachments {
		return nil, fmt.Errorf("You can attach up to %d files per message.", maxAttachments)
	}
	var saved []store.NewAttachment
	fail := func(err error) ([]store.NewAttachment, error) {
		s.st.RemoveUploads(saved)
		return nil, err
	}
	for _, fh := range parts {
		if fh.Size > maxFileBytes {
			return fail(fmt.Errorf("%s is larger than %d MB.", fh.Filename, maxFileBytes>>20))
		}
		f, err := fh.Open()
		if err != nil {
			return fail(err)
		}
		data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
		f.Close()
		if err != nil {
			return fail(err)
		}
		c, err := files.Classify(data, fh.Filename)
		if err != nil {
			return fail(fmt.Errorf("%s: %s", fh.Filename, err.Error()))
		}
		if c.Kind == store.KindImage {
			if !vision {
				return fail(errors.New("This chat's model can't read images. Pick a model that supports images, or attach a document instead."))
			}
			if len(data) > maxImageBytes {
				return fail(fmt.Errorf("%s is larger than %d MB.", fh.Filename, maxImageBytes>>20))
			}
		}
		att, err := s.st.SaveUpload(data, c.Mime, fh.Filename, c.Kind, c.Text)
		if err != nil {
			return fail(err)
		}
		saved = append(saved, att)
	}
	return saved, nil
}

// parseMessageForm parses url-encoded or multipart bodies.
func parseMessageForm(r *http.Request) error {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if r.MultipartForm != nil {
			return nil // already parsed (e.g. by the CSRF check)
		}
		return r.ParseMultipartForm(8 << 20)
	}
	return r.ParseForm()
}

func (s *Server) modelVision(t *store.Thread) bool {
	if !t.ModelID.Valid {
		return false
	}
	m, err := s.st.ModelByID(t.ModelID.Int64)
	return err == nil && m.Vision
}

// ---- pages ----

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	opts, _ := s.st.ModelOptions(u.ID)
	agents, _ := s.st.ListAgents(u.ID) // offered in the composer, to connect before the first message
	servers, _ := s.st.ListMCPServers(u.ID)
	hidden := false // "Getting started" was dismissed in this browser
	if c, err := r.Cookie("kn_start"); err == nil && c.Value == "off" {
		hidden = true
	}
	taken := s.takenFromGroup(r, u)          // "Take to my chat" on a message of a group
	data := s.toolsData(u, nil, taken.Group) // the Tools menu for a chat that does not exist yet
	data["Taken"] = taken
	for k, v := range map[string]any{
		"ModelOptions": opts, "DefaultModel": s.defaultModel(u.ID, opts), "MaxFiles": maxAttachments, "Agents": agents,
		"HasTools": len(servers) > 0, "StartHidden": hidden,
	} {
		data[k] = v
	}
	s.render(w, r, "chat", data)
}

func (s *Server) newThread(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	fail := func(code int, msg string) {
		if isFetch(r) {
			http.Error(w, msg, code)
			return
		}
		s.errorPage(w, r, code, msg)
	}
	if err := parseMessageForm(r); err != nil {
		fail(http.StatusBadRequest, "Could not read the upload (too large?).")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	opts, _ := s.st.ModelOptions(u.ID)
	modelID := formInt(r, "model_id", 0)
	opt := findOption(opts, modelID)
	if opt == nil {
		modelID = s.defaultModel(u.ID, opts)
		opt = findOption(opts, modelID)
	}
	uploads, err := s.readUploads(r, opt != nil && opt.Vision)
	if err != nil {
		fail(http.StatusBadRequest, err.Error())
		return
	}
	id, err := s.st.CreateThread(u.ID, modelID)
	if err != nil {
		s.st.RemoveUploads(uploads)
		s.serverError(w, r, err)
		return
	}
	// Agents ticked on the home screen are connected before the first message
	// runs, so that first turn can already hand them work.
	var agentIDs []int64
	for _, v := range r.Form["agent"] {
		if aid, err := parseInt(v); err == nil {
			agentIDs = append(agentIDs, aid)
		}
	}
	if len(agentIDs) > 0 {
		if err := s.st.SetThreadAgents(id, u.ID, agentIDs); err != nil {
			s.log.Warn("could not connect agents to the new chat", "thread", id, "err", err)
		}
	}
	s.newThreadTools(r, u, id) // before the first message runs, so that turn already honours the choice
	content := strings.TrimSpace(r.FormValue("content"))
	if len(content) > maxMessageLen {
		content = content[:maxMessageLen]
	}
	if content != "" || len(uploads) > 0 {
		if _, err := s.eng.Submit(id, u.ID, content, uploads); err != nil {
			s.st.RemoveUploads(uploads)
			s.serverError(w, r, err)
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/t/%d", id), http.StatusSeeOther)
}

func (s *Server) threadPage(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	u := currentUser(r)
	data, err := s.messagesData(t, u, role, 0)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data["Members"], _ = s.st.ThreadMembers(t.ID)
	data["ModelName"] = "No model selected"
	data["Vision"] = false
	if t.ModelID.Valid {
		if m, err := s.st.ModelByID(t.ModelID.Int64); err == nil {
			data["ModelName"] = m.Name
			data["Vision"] = m.Vision
		}
	}
	data["Pinned"], data["Archived"] = s.st.ThreadFlags(t.ID, u.ID)
	if role == "owner" { // the Tools menu next to Attach
		for k, v := range s.toolsData(u, t, 0) {
			if k != "Thread" && k != "Group" {
				data[k] = v
			}
		}
	}
	data["MaxFiles"] = maxAttachments
	if a, _ := s.st.AgentByThread(t.ID); a != nil { // an agent's page: same chat view, agent panel
		data["Agent"] = a
		data["OpenTasks"] = s.st.OpenTaskCount(a.ID)
		// A scheduled agent that used up today's runs stays quiet until midnight UTC: say so.
		data["AgentLimit"] = a.Enabled && a.Scheduled() && s.st.RunsToday(a.ThreadID) >= a.DailyRuns
	} else if t.IsGroup() {
		// What the group has, for everyone in it to see: its memory and how many tool servers.
		data["GroupMemoryOn"] = !s.st.ThreadToolsOff(t.ID)[store.ToolMemory]
		servers, _ := s.st.ThreadMCPServers(t)
		data["GroupServers"] = len(servers)
	} else if role == "owner" { // a personal chat: the Agents menu, and whether a read-only link exists
		data["Agents"], data["ConnectedCount"] = s.agentOptions(t)
		if _, err := s.st.ShareOf(t.ID); err == nil {
			data["Shared"] = true
		}
	}
	s.render(w, r, "chat", data)
}

// threadMessages returns the message list fragment.
//   - ?from=RUN     everything from that run on (refresh of a page that loaded history)
//   - ?before=RUN   the page of messages before that run ("load earlier")
func (s *Server) threadMessages(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	if before, _ := parseInt(q.Get("before")); before > 0 {
		msgs, err := s.st.ListMessagesBefore(t.ID, before, messagePageSize)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		data := s.viewMessages(t, currentUser(r), role, msgs, false)
		s.renderPartial(w, r, "chat", "older", data)
		return
	}
	from, _ := parseInt(q.Get("from"))
	data, err := s.messagesData(t, currentUser(r), role, from)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPartial(w, r, "chat", "messages", data)
}

// msgView adds presentation fields to a message.
type msgView struct {
	store.Message
	Mine       bool
	Queued     bool
	CanCancel  bool
	CanEdit    bool // user's own queued or latest message
	CanRegen   bool // final answer of the latest finished turn, asked by me
	CanRetry   bool // latest turn failed / stopped, asked by me
	Failed     bool
	Summarised bool // folded into the memory summary
	Calls      []callView
	Images     []store.Attachment
	Files      []store.Attachment
	Audio      *store.Attachment // the answer read aloud, once someone asked for it
	CanSpeak   bool              // the viewer has a voice model: offer "Read aloud"
	Approvals  []approvalView
	Footer     string // run outcome shown under the run's last message
	Down       string // tool servers the run could not reach, said in a sentence
	CanTake    bool   // a finished message of a group: "Take to my chat"

	// When the viewer does not want tool details (Settings → Appearance):
	HideBody   bool       // this message is only a tool call or a tool result: draw nothing for it
	NoThinking bool       // the viewer does not want the "Thinking" block
	ToolSteps  []toolStep // on a run's last message: every tool the run used, for the one line "Used 2 tools"
	ToolFailed int        // how many of them did not work
	Working    string     // the run is waiting for this tool right now ("" = it is not)
}

// toolStep is one tool call of a run with what the tool answered.
type toolStep struct {
	Name, Args, Result string
	Failed             bool
}

// toolSteps collects, per run, the tool calls in msgs and their answers.
func toolSteps(msgs []store.Message) map[int64][]toolStep {
	steps := map[int64][]toolStep{}
	type at struct {
		run int64
		i   int
	}
	byCall := map[string]at{}
	for _, m := range msgs {
		switch {
		case m.Role == "assistant" && m.ToolCalls != "":
			var calls []struct {
				ID       string                           `json:"id"`
				Function struct{ Name, Arguments string } `json:"function"`
			}
			json.Unmarshal([]byte(m.ToolCalls), &calls)
			for _, c := range calls {
				byCall[c.ID] = at{m.RunID, len(steps[m.RunID])}
				steps[m.RunID] = append(steps[m.RunID], toolStep{Name: c.Function.Name, Args: c.Function.Arguments})
			}
		case m.Role == "tool":
			if p, ok := byCall[m.ToolCallID]; ok && p.run == m.RunID {
				st := &steps[p.run][p.i]
				st.Result = m.Content
				st.Failed = strings.HasPrefix(m.Content, "Error:") || strings.HasPrefix(m.Content, "Tool reported an error:")
			}
		}
	}
	return steps
}

type callView struct{ Name, Args string }

type approvalView struct {
	store.ToolApproval
	CanDecide bool
	CanAlways bool // may switch the server to automatic
	Waiting   string
}

// viewMessages builds the view models for a slice of messages. latestRun
// controls edit/regenerate affordances (only meaningful for the newest page).
func (s *Server) viewMessages(t *store.Thread, u *store.User, role string, msgs []store.Message, newest bool) map[string]any {
	var userMsgIDs []int64 // every message that can carry files: uploads on a person's, made pictures and recordings on the AI's
	runAuthor := map[int64]int64{}
	var latest int64
	_, canSpeak := s.eng.CanMakeInChat(u.ID)
	for _, m := range msgs {
		if m.Role != "tool" {
			userMsgIDs = append(userMsgIDs, m.ID)
		}
		if m.Role == "user" {
			if m.AuthorUserID.Valid {
				runAuthor[m.RunID] = m.AuthorUserID.Int64
			}
		}
		latest = max(latest, m.RunID)
	}
	if newest {
		latest = max(latest, s.st.LastRunID(t.ID))
	} else {
		latest = -1
	}
	// What this viewer wants to see of the AI's work. An agent's page is its
	// log, so it always shows everything.
	showTools, showThinking := u.ShowTools, u.ShowThinking
	if a, _ := s.st.AgentByThread(t.ID); a != nil {
		showTools, showThinking = true, true
	}
	var steps map[int64][]toolStep
	if !showTools {
		steps = toolSteps(msgs)
	}
	atts, _ := s.st.AttachmentsByMessage(userMsgIDs)
	pending, _ := s.st.PendingApprovals(t.ID)
	canWrite := role == "owner" || role == "member"
	finished := func(status string) bool {
		return status == store.RunDone || status == store.RunFailed || status == store.RunCancelled || status == store.RunInterrupted
	}

	views := make([]msgView, len(msgs))
	for i, m := range msgs {
		v := msgView{Message: m}
		for _, a := range atts[m.ID] {
			switch {
			case a.IsImage():
				v.Images = append(v.Images, a)
			case a.IsAudio():
				v.Audio = &a
			default:
				v.Files = append(v.Files, a)
			}
		}
		v.CanSpeak = canSpeak && m.Role == "assistant" && m.Status == store.MsgDone
		v.CanTake = t.IsGroup() && m.Role != "tool" && m.Status == store.MsgDone && !v.Queued && strings.TrimSpace(m.Content) != ""
		v.Mine = m.AuthorUserID.Valid && m.AuthorUserID.Int64 == u.ID
		v.Queued = m.RunStatus == store.RunPending
		v.CanCancel = v.Queued && (v.Mine || role == "owner")
		v.Summarised = m.RunID <= t.CompactedUntilRun
		myRun := runAuthor[m.RunID] == u.ID && canWrite
		isLatest := m.RunID == latest
		lastOfRun := i == len(msgs)-1 || msgs[i+1].RunID != m.RunID

		if m.Role == "user" && v.Mine && canWrite {
			v.CanEdit = v.Queued || isLatest
		}
		if m.ToolCalls != "" {
			var calls []struct {
				Function struct{ Name, Arguments string } `json:"function"`
			}
			json.Unmarshal([]byte(m.ToolCalls), &calls)
			for _, c := range calls {
				v.Calls = append(v.Calls, callView{Name: c.Function.Name, Args: c.Function.Arguments})
			}
		}
		for _, a := range pending[m.ID] {
			av := approvalView{ToolApproval: a}
			av.CanDecide = canWrite && (runAuthor[a.RunID] == u.ID || u.ID == t.OwnerUserID)
			av.CanAlways = av.CanDecide && u.ID == t.OwnerUserID
			if !av.CanDecide {
				av.Waiting = "Waiting for the chat owner or the person who asked to approve this."
			}
			v.Approvals = append(v.Approvals, av)
		}
		if lastOfRun {
			if m.Role == "assistant" && m.RunStatus == store.RunDone && isLatest && myRun {
				v.CanRegen = true
			}
			if m.RunDown != "" {
				v.Down = engine.ToolsDownText(m.RunDown)
			}
			switch m.RunStatus {
			case store.RunFailed:
				v.Footer, v.Failed = m.RunError, true
			case store.RunCancelled:
				v.Footer = "Stopped"
			case store.RunInterrupted:
				v.Footer = "Interrupted: " + m.RunError
			}
			v.CanRetry = v.Footer != "" && isLatest && myRun && finished(m.RunStatus)
		}
		v.NoThinking = !showThinking
		if !showTools {
			// Tool calls and results are not rows of their own. What must stay
			// visible stays: a call that waits for permission, a picture a tool
			// made, text the AI wrote next to a call, and thinking if wanted.
			switch {
			case m.Role == "tool":
				v.HideBody = true
			case m.Role == "assistant" && m.ToolCalls != "":
				v.Calls = nil
				v.HideBody = strings.TrimSpace(m.Content) == "" && len(v.Images) == 0 && len(v.Approvals) == 0 &&
					m.Status != store.MsgStreaming && (m.Reasoning == "" || !showThinking)
			}
			if lastOfRun && len(steps[m.RunID]) > 0 {
				v.ToolSteps = steps[m.RunID]
				for _, st := range v.ToolSteps {
					if st.Failed {
						v.ToolFailed++
					}
				}
				// A sign of life while a tool runs: without it a slow tool looks like a frozen chat.
				if last := v.ToolSteps[len(v.ToolSteps)-1]; m.RunStatus == store.RunRunning && last.Result == "" && len(v.Approvals) == 0 {
					v.Working = last.Name
				}
			}
		}
		views[i] = v
	}
	var oldest int64
	if len(msgs) > 0 {
		oldest = msgs[0].RunID
	}
	return map[string]any{
		"Thread":    t,
		"Role":      role,
		"CanWrite":  canWrite,
		"Messages":  views,
		"OldestRun": oldest,
		"HasOlder":  oldest > 0 && s.st.HasMessagesBefore(t.ID, oldest),
		// app.js adds the "Thinking" block while an answer streams, unless told not to
		"ShowThinking": showThinking,
	}
}

// messagesData is the newest page (or, with from > 0, everything from that run).
func (s *Server) messagesData(t *store.Thread, u *store.User, role string, from int64) (map[string]any, error) {
	var msgs []store.Message
	var err error
	if from > 0 {
		msgs, err = s.st.ListMessagesFrom(t.ID, from, messageMaxLoad)
	} else {
		msgs, err = s.st.ListMessages(t.ID, messagePageSize)
	}
	if err != nil {
		return nil, err
	}
	data := s.viewMessages(t, u, role, msgs, true)
	used, window := s.eng.ContextEstimate(t)
	pct := 0
	if window > 0 {
		pct = min(100, used*100/window)
	}
	// An agent whose last run went wrong says so at the top of its page until a run works again.
	if a, _ := s.st.AgentByThread(t.ID); a != nil && role == "owner" && !s.st.ThreadBusy(t.ID) {
		if last, _ := s.st.LastRunOutcome(t.ID, 0); last.Bad() {
			data["AgentAlert"] = map[string]any{"AgentID": a.ID, "Failed": last.Status == store.RunFailed,
				"Text": strings.TrimPrefix(engine.AgentProblemText(last), "The last run could not finish: "), "When": last.FinishedAt}
		}
	}
	data["Pending"] = s.st.PendingCount(t.ID)
	data["Running"] = s.eng.ActiveRunID(t.ID) != 0
	data["Waiting"] = s.eng.Waiting(t.ID)
	data["Group"] = t.IsGroup()
	data["GroupNotice"] = t.IsGroup() && t.GroupNotice && role == "owner"
	data["ContextPct"] = pct
	data["ContextUsed"] = used
	data["ContextWindow"] = window
	return data, nil
}

// threadEvents is the SSE stream for a thread. Browsers connect here to watch
// progress; disconnecting never affects the run.
func (s *Server) threadEvents(w http.ResponseWriter, r *http.Request) {
	t, _, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // disable nginx buffering

	userID := currentUser(r).ID
	defer s.watchThread(t.ID, userID)()
	ch, unsubscribe := s.hub.Subscribe(t.ID)
	defer unsubscribe()

	fmt.Fprint(w, "retry: 3000\n\n")
	// Subscribe first, then snapshot: deltas carry a running length so the
	// client can discard ones already contained in the snapshot.
	if id, content, n, reasoning, rn, ok := s.eng.Snapshot(t.ID); ok {
		b, _ := json.Marshal(map[string]any{"type": engine.EvSnapshot, "data": map[string]any{
			"message_id": id, "content": content, "len": n, "reasoning": reasoning, "rlen": rn}})
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	flusher.Flush()

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-ping.C:
			if s.st.MemberRole(t.ID, userID) == "" { // removed from the chat
				return
			}
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	if role == "viewer" {
		http.Error(w, "Viewers cannot send messages.", http.StatusForbidden)
		return
	}
	if err := parseMessageForm(r); err != nil {
		http.Error(w, "Could not read the upload (too large?).", http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	content := strings.TrimSpace(r.FormValue("content"))
	if len(content) > maxMessageLen {
		http.Error(w, "Message is too long.", http.StatusRequestEntityTooLarge)
		return
	}
	uploads, err := s.readUploads(r, s.modelVision(t))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if content == "" && len(uploads) == 0 {
		http.Error(w, "Message is empty.", http.StatusBadRequest)
		return
	}
	if _, err := s.eng.Submit(t.ID, currentUser(r).ID, content, uploads); err != nil {
		s.st.RemoveUploads(uploads)
		if errors.Is(err, store.ErrQueueFull) {
			st, _ := s.st.LoadSettings()
			http.Error(w, fmt.Sprintf("Queue is full (%d/%d). Wait for the AI to catch up.", st.QueueMax, st.QueueMax), http.StatusTooManyRequests)
			return
		}
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// editRun edits the user's queued/latest message, or regenerates its answer
// (regenerate=1). The old answer is replaced.
func (s *Server) editRun(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	run, err := s.st.RunByID(pathID(r, "rid"))
	if err != nil || run.ThreadID != t.ID {
		http.Error(w, "Message not found.", http.StatusNotFound)
		return
	}
	if role == "viewer" || run.AuthorUserID != currentUser(r).ID {
		http.Error(w, "You can only edit your own messages.", http.StatusForbidden)
		return
	}
	var content *string
	if r.FormValue("regenerate") != "1" {
		c := strings.TrimSpace(r.FormValue("content"))
		if c == "" {
			msgs, _ := s.st.RunMessages(run.ID)
			hasFiles := false
			if len(msgs) > 0 {
				a, _ := s.st.AttachmentsByMessage([]int64{msgs[0].ID})
				hasFiles = len(a[msgs[0].ID]) > 0
			}
			if !hasFiles {
				http.Error(w, "Message is empty.", http.StatusBadRequest)
				return
			}
		}
		if len(c) > maxMessageLen {
			http.Error(w, "Message is too long.", http.StatusRequestEntityTooLarge)
			return
		}
		content = &c
	}
	if err := s.eng.Edit(t.ID, run.ID, content); err != nil {
		if errors.Is(err, engine.ErrNotLatest) {
			http.Error(w, "Only your latest message can be edited. Someone has replied since.", http.StatusConflict)
			return
		}
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decideApproval answers a pending tool call: allow | deny | always
// (always = allow and switch the server to automatic; owner only).
func (s *Server) decideApproval(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	u := currentUser(r)
	a, err := s.st.ApprovalByID(pathID(r, "aid"))
	if err != nil || a.ThreadID != t.ID {
		http.Error(w, "Approval not found.", http.StatusNotFound)
		return
	}
	run, err := s.st.RunByID(a.RunID)
	if err != nil || role == "viewer" || (run.AuthorUserID != u.ID && t.OwnerUserID != u.ID) {
		http.Error(w, "Only the chat owner or the person who asked can decide this.", http.StatusForbidden)
		return
	}
	decision := r.FormValue("decision")
	if decision == "always" {
		if u.ID != t.OwnerUserID {
			http.Error(w, "Only the owner of the tool server can change its mode.", http.StatusForbidden)
			return
		}
		if err := s.st.SetMCPApproval(u.ID, a.ServerID, store.ApprovalAuto); err != nil {
			s.serverError(w, r, err)
			return
		}
		decision = "allow"
	}
	if err := s.eng.Decide(a.ID, u.ID, decision == "allow"); err != nil {
		if errors.Is(err, engine.ErrAlreadyDecided) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// serveFile streams an attachment to members of its chat (see sendAttachment).
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	a, err := s.st.AttachmentByID(pathID(r, "aid"))
	if err != nil || s.st.MemberRole(a.ThreadID, currentUser(r).ID) == "" {
		http.NotFound(w, r)
		return
	}
	s.sendAttachment(w, r, a, "private, max-age=31536000, immutable")
}

// speakMessage answers with the address of an AI answer read aloud. The
// recording is made the first time someone asks, with that person's voice
// model, and kept on the message for everyone in the chat.
func (s *Server) speakMessage(w http.ResponseWriter, r *http.Request) {
	t, _, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	msg, err := s.st.ThreadMessage(t.ID, pathID(r, "mid"))
	if err != nil || msg.Role != "assistant" || msg.Status != store.MsgDone || strings.TrimSpace(msg.Content) == "" {
		http.Error(w, "That answer cannot be read aloud.", http.StatusNotFound)
		return
	}
	a, err := s.eng.SpeakMessage(r.Context(), currentUser(r).ID, msg)
	switch {
	case errors.Is(err, store.ErrNoMediaModel):
		http.Error(w, "No voice model is set up. Choose one under Feed → Settings → Pictures, voice and video.", http.StatusBadRequest)
		return
	case errors.Is(err, engine.ErrSpeaking):
		http.Error(w, "This answer is already being read. It will be ready in a moment.", http.StatusConflict)
		return
	case err != nil:
		msg := err.Error()
		http.Error(w, strings.ToUpper(msg[:1])+msg[1:]+".", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": fmt.Sprintf("/files/%d", a.ID)})
}

func safeFileName(n string) string {
	n = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' || r == '/' {
			return '_'
		}
		return r
	}, n)
	if n == "" {
		return "file"
	}
	return n
}

func (s *Server) stopRun(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	if role == "viewer" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	s.eng.Stop(t.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cancelQueued(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	run, err := s.st.RunByID(pathID(r, "rid"))
	if err != nil || run.ThreadID != t.ID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if run.AuthorUserID != currentUser(r).ID && role != "owner" {
		http.Error(w, "You can only cancel your own messages.", http.StatusForbidden)
		return
	}
	if err := s.eng.CancelQueued(t.ID, run.ID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// renameThread lets the owner rename a chat from the sidebar / top bar.
func (s *Server) renameThread(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return
	}
	title := strings.Join(strings.Fields(r.FormValue("title")), " ")
	if title == "" {
		http.Error(w, "Title can't be empty.", http.StatusBadRequest)
		return
	}
	if rs := []rune(title); len(rs) > 80 {
		title = string(rs[:80])
	}
	if err := s.st.SetThreadTitle(t.ID, title); err != nil {
		s.serverError(w, r, err)
		return
	}
	if isFetch(r) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, title)
		return
	}
	back(w, r, fmt.Sprintf("/t/%d", t.ID))
}

// threadFlag pins/unpins or archives/unarchives a chat for the current user.
func (s *Server) threadFlag(w http.ResponseWriter, r *http.Request) {
	t, _, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	flag := r.FormValue("flag")
	if err := s.st.SetThreadFlag(t.ID, currentUser(r).ID, flag, r.FormValue("on") == "1"); err != nil {
		http.Error(w, "Unknown flag.", http.StatusBadRequest)
		return
	}
	if isFetch(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	back(w, r, fmt.Sprintf("/t/%d", t.ID))
}

// exportThread downloads the whole chat as Markdown or JSON.
func (s *Server) exportThread(w http.ResponseWriter, r *http.Request) {
	t, _, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	msgs, err := s.st.AllMessages(t.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	name := safeFileName(strings.Join(strings.Fields(t.Title), "-"))
	if r.URL.Query().Get("format") == "json" {
		type out struct {
			ID        int64  `json:"id"`
			Run       int64  `json:"run"`
			Role      string `json:"role"`
			Author    string `json:"author,omitempty"`
			Content   string `json:"content"`
			Reasoning string `json:"reasoning,omitempty"`
			ToolCalls string `json:"tool_calls,omitempty"`
			ToolName  string `json:"tool_name,omitempty"`
			CreatedAt string `json:"created_at"`
		}
		list := make([]out, 0, len(msgs))
		for _, m := range msgs {
			list = append(list, out{m.ID, m.RunID, m.Role, m.AuthorName, m.Content, m.Reasoning, m.ToolCalls, m.ToolName, m.CreatedAt})
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".json"))
		json.NewEncoder(w).Encode(map[string]any{"title": t.Title, "created_at": t.CreatedAt, "summary": t.Summary, "messages": list})
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nExported from Knitnox on %s.\n\n", t.Title, time.Now().UTC().Format("2006-01-02 15:04 UTC"))
	if t.Summary != "" {
		fmt.Fprintf(&b, "> **Summary of earlier messages**\n>\n> %s\n\n", strings.ReplaceAll(t.Summary, "\n", "\n> "))
	}
	for _, m := range msgs {
		switch m.Role {
		case "user":
			who := m.AuthorName
			if who == "" {
				who = "User"
			}
			fmt.Fprintf(&b, "### %s · %s\n\n%s\n\n", who, m.CreatedAt, m.Content)
		case "assistant":
			if m.Content == "" {
				continue
			}
			fmt.Fprintf(&b, "### Assistant · %s\n\n%s\n\n", m.CreatedAt, m.Content)
		case "tool":
			fmt.Fprintf(&b, "<details><summary>Tool result: %s</summary>\n\n```\n%s\n```\n\n</details>\n\n", m.ToolName, m.Content)
		}
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".md"))
	io.WriteString(w, b.String())
}

// searchPage runs a full-text search across the user's chats.
func (s *Server) searchPage(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if r := []rune(q); len(r) > 200 {
		q = string(r[:200])
	}
	var hits []store.SearchHit
	var err error
	if q != "" {
		hits, err = s.st.SearchMessages(currentUser(r).ID, q, 100)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	type hitView struct {
		store.SearchHit
		Snippet template.HTML
	}
	views := make([]hitView, 0, len(hits))
	for _, h := range hits {
		esc := template.HTMLEscapeString(h.Snippet)
		esc = strings.ReplaceAll(strings.ReplaceAll(esc, "[[", "<mark>"), "]]", "</mark>")
		views = append(views, hitView{h, template.HTML(esc)})
	}
	s.render(w, r, "search", map[string]any{"Query": q, "Hits": views})
}

// ---- thread panel: members, invites, settings ----

func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// shareView is the read-only link of a personal chat, as the panel shows it.
type shareView struct {
	URL       string
	UpdatedAt string
	ShowTools bool
	Behind    bool // the chat has newer messages than the link shows
}

func (s *Server) panelData(r *http.Request, t *store.Thread, role string) (map[string]any, error) {
	members, err := s.st.ThreadMembers(t.ID)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"Thread": t, "Role": role, "Members": members, "BaseURL": baseURL(r), "Group": t.IsGroup()}
	if role != "owner" {
		return data, nil
	}
	data["ModelOptions"], _ = s.st.ModelOptions(t.OwnerUserID)
	if t.IsGroup() {
		data["Invites"], _ = s.st.ListInvites(t.ID)
		data["Servers"], _ = s.st.ListGroupServers(t.ID) // the group's own tool servers
		return data, nil
	}
	// A personal chat: its read-only link, and the owner's agents it can manage.
	data["ShareOff"] = !s.shareLinksOn()
	if sh, err := s.st.ShareOf(t.ID); err == nil {
		data["Share"] = shareView{URL: baseURL(r) + "/s/" + sh.Token, UpdatedAt: sh.UpdatedAt, ShowTools: sh.ShowTools,
			Behind: s.st.LastRunID(t.ID) > sh.UntilRun}
	}
	data["Agents"], _ = s.agentOptions(t)
	return data, nil
}

func (s *Server) renderPanel(w http.ResponseWriter, r *http.Request, t *store.Thread, role string) {
	data, err := s.panelData(r, t, role)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPartial(w, r, "chat", "panel", data)
}

func (s *Server) threadPanel(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	s.renderPanel(w, r, t, role)
}

func (s *Server) threadSettings(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = t.Title
	}
	if r := []rune(title); len(r) > 80 {
		title = string(r[:80])
	}
	modelID := formInt(r, "model_id", 0)
	opts, _ := s.st.ModelOptions(t.OwnerUserID)
	if findOption(opts, modelID) == nil {
		modelID = t.ModelID.Int64
	}
	instr := strings.TrimSpace(r.FormValue("instructions"))
	if r := []rune(instr); len(r) > 4000 {
		instr = string(r[:4000])
	}
	if err := s.st.UpdateThreadSettings(t.ID, title, modelID, instr); err != nil {
		s.serverError(w, r, err)
		return
	}
	if !t.IsGroup() { // a group never manages anybody's personal agents
		r.ParseForm()
		var agentIDs []int64
		for _, v := range r.Form["agent"] {
			if id, err := parseInt(v); err == nil {
				agentIDs = append(agentIDs, id)
			}
		}
		if err := s.st.SetThreadAgents(t.ID, t.OwnerUserID, agentIDs); err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	if isHX(r) {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	back(w, r, fmt.Sprintf("/t/%d", t.ID))
}

func (s *Server) deleteThread(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return
	}
	s.eng.Stop(t.ID)
	files := s.st.ThreadUploadFiles(t.ID)
	if err := s.st.DeleteThread(t.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.st.RemoveFiles(files)
	s.hub.Publish(t.ID, engine.EvMessages, nil)
	if isHX(r) {
		w.Header().Set("HX-Redirect", "/")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) leaveThread(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return
	}
	if role == "owner" {
		s.errorPage(w, r, http.StatusBadRequest, "The admin cannot leave; delete the group instead.")
		return
	}
	if err := s.st.RemoveMember(t.ID, currentUser(r).ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if isHX(r) {
		w.Header().Set("HX-Redirect", "/")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) createInvite(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return
	}
	if !t.IsGroup() {
		s.errorPage(w, r, http.StatusBadRequest, "A personal chat cannot be joined. Share a read-only link to it, or start a group.")
		return
	}
	role := r.FormValue("role")
	if role != "viewer" {
		role = "member"
	}
	days := min(max(formInt(r, "days", 7), 1), 90)
	uses := int(min(max(formInt(r, "uses", 5), 1), 100))
	if _, err := s.st.CreateInvite(t.ID, currentUser(r).ID, role, time.Duration(days)*24*time.Hour, uses); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPanel(w, r, t, "owner")
}

func (s *Server) revokeInvite(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return
	}
	if err := s.st.RevokeInvite(t.ID, r.PathValue("tok")); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPanel(w, r, t, "owner")
}

func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return
	}
	if err := s.st.RemoveMember(t.ID, pathID(r, "uid")); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.hub.Publish(t.ID, engine.EvMessages, nil)
	s.renderPanel(w, r, t, "owner")
}

func (s *Server) invitePage(w http.ResponseWriter, r *http.Request) {
	inv, err := s.st.InviteByToken(r.PathValue("tok"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "This invite link is invalid, expired or used up.")
		return
	}
	if s.st.MemberRole(inv.ThreadID, currentUser(r).ID) != "" {
		http.Redirect(w, r, fmt.Sprintf("/t/%d", inv.ThreadID), http.StatusSeeOther)
		return
	}
	s.render(w, r, "invite", map[string]any{"Invite": inv})
}

func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	threadID, err := s.st.AcceptInvite(r.PathValue("tok"), currentUser(r).ID)
	if err != nil {
		s.errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}
	s.hub.Publish(threadID, engine.EvMessages, nil)
	http.Redirect(w, r, fmt.Sprintf("/t/%d", threadID), http.StatusSeeOther)
}
