package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"knitnox/internal/llm"
	"knitnox/internal/store"
)

// turnCtx is everything resolved once at the start of a run.
type turnCtx struct {
	run         *store.Run
	thread      *store.Thread
	settings    store.Settings
	model       *store.Model
	provider    *store.Provider
	billable    bool // global provider: counts toward the author's quota
	group       bool
	members     []store.Member
	ownerInstr  string         // thread owner's personal instructions (solo chats only)
	agent       *store.Agent   // set when the thread belongs to an agent
	connected   []store.Agent  // agents this chat manages (normal chats only)
	loc         *time.Location // time zone of the person this run works for
	memoryUser  int64          // whose long-term memory this run may use (0 = none)
	memoryWrite bool           // false for agents: they only read
	planOn      bool           // this run may use its owner's Plan Book
	planWrite   bool           // false = read only
	feedOn      bool           // this run may post to its owner's feed
	feedDaily   int            // posts per day (agents only; 0 = no limit)
	feedMedia   string         // what else a post can carry here, in a sentence ("" = nothing)
}

// execute runs the agent loop for one queued user turn. It returns the number
// of LLM steps taken.
func (e *Engine) execute(ctx context.Context, run *store.Run) (int, error) {
	tc, err := e.prepare(run)
	if err != nil {
		return 0, err
	}
	if tc.billable {
		if err := e.checkQuota(run.AuthorUserID); err != nil {
			return 0, err
		}
	}
	if tc.agent == nil { // agent runs start fresh: nothing to title or compact
		e.autoTitle(tc)
		if err := e.maybeCompact(ctx, tc); err != nil {
			if ctx.Err() != nil {
				return 0, err
			}
			e.log.Warn("compaction failed; continuing with full history", "thread", tc.thread.ID, "err", err)
		}
	}

	tools, err := e.openTools(ctx, tc)
	if err != nil {
		return 0, err
	}
	defer tools.close()

	var msgs []llm.Message
	maxSteps := max(tc.settings.AgentMaxSteps, 1)
	if tc.agent != nil {
		msgs, err = e.buildAgentContext(tc)
		maxSteps = max(tc.agent.MaxSteps, 1)
	} else {
		msgs, err = e.buildContext(tc)
	}
	if err != nil {
		return 0, err
	}
	if len(tc.connected) > 0 {
		msgs[0].Content += connectedAgentsNote(tc.connected)
	}
	if tc.memoryUser > 0 {
		msgs[0].Content += e.memoryNote(tc.memoryUser, tc.memoryWrite)
	}
	if tc.planOn {
		msgs[0].Content += planNote(tc.planWrite)
	}
	if tc.feedOn {
		msgs[0].Content += feedNote(tc.agent != nil, tc.feedDaily, tc.feedMedia)
	}

	steps := 0
	for {
		steps++
		final := steps > maxSteps // limit reached: one last call without tools
		if tc.billable && steps > 1 {
			if err := e.checkQuota(run.AuthorUserID); err != nil {
				return steps - 1, err
			}
		}
		var defs []llm.Tool
		if !final {
			defs = tools.defs
		} else if len(tools.defs) > 0 {
			msgs = append(msgs, llm.Message{Role: "system", Content: fmt.Sprintf(
				"Tool step limit (%d) reached. Do not call tools; answer with what you have.", maxSteps)})
		}

		res, msgID, err := e.streamStep(ctx, tc, msgs, defs)
		if err != nil {
			return steps, err
		}
		if len(res.ToolCalls) == 0 || final {
			return steps, nil
		}

		msgs = append(msgs, llm.Message{Role: "assistant", Content: res.Content, ToolCalls: res.ToolCalls})
		for _, call := range res.ToolCalls {
			var out string
			if route, ok := tools.routes[call.Function.Name]; ok && route.ask {
				status, err := e.awaitApproval(ctx, tc, msgID, call, route)
				if err != nil {
					return steps, err
				}
				switch status {
				case store.ApprovalDenied:
					out = "The user did not allow this tool call. Do not retry it; continue without it and tell the user what you could not do."
				case store.ApprovalExpired:
					out = "Nobody approved this tool call in time, so it was not run. Continue without it."
				}
			}
			if out == "" {
				out = tools.call(ctx, call, tc.settings.ToolMaxRetries)
			}
			if ctx.Err() != nil {
				return steps, ctx.Err()
			}
			if _, err := e.st.CreateMessage(&store.Message{
				ThreadID: tc.thread.ID, RunID: run.ID, Role: "tool",
				Content: out, ToolCallID: call.ID, ToolName: call.Function.Name,
			}); err != nil {
				return steps, err
			}
			e.hub.Publish(tc.thread.ID, EvMessages, nil)
			msgs = append(msgs, llm.Message{Role: "tool", Content: out, ToolCallID: call.ID})
		}
	}
}

// prepare loads the thread, model, provider and decides who pays.
func (e *Engine) prepare(run *store.Run) (*turnCtx, error) {
	tc := &turnCtx{run: run}
	var err error
	if tc.settings, err = e.st.LoadSettings(); err != nil {
		return nil, err
	}
	if tc.thread, err = e.st.ThreadByID(run.ThreadID); err != nil {
		return nil, err
	}
	if tc.members, err = e.st.ThreadMembers(run.ThreadID); err != nil {
		return nil, err
	}
	if tc.agent, err = e.st.AgentByThread(run.ThreadID); err != nil {
		return nil, err
	}
	tc.group = len(tc.members) > 1 && tc.agent == nil
	// Whose clock the run goes by: the owner's time zone in their own chat,
	// the server's in a group chat, where people may be in different zones.
	tc.loc = time.Local
	if !tc.group {
		tc.loc = time.UTC
		if u, err := e.st.UserByID(tc.thread.OwnerUserID); err == nil {
			tc.loc = u.Location()
		}
	}
	if !tc.group && tc.agent == nil {
		if owner, err := e.st.UserByID(tc.thread.OwnerUserID); err == nil {
			tc.ownerInstr = strings.TrimSpace(owner.Instructions)
		}
	}
	if !tc.thread.ModelID.Valid {
		return nil, errors.New("no model selected for this chat")
	}
	if tc.model, err = e.st.ModelByID(tc.thread.ModelID.Int64); err != nil {
		return nil, errors.New("the selected model no longer exists")
	}
	if !tc.model.Enabled {
		return nil, errors.New("the selected model is disabled")
	}
	if tc.provider, err = e.st.ProviderByID(tc.model.ProviderID); err != nil {
		return nil, err
	}
	if !tc.provider.Enabled {
		return nil, errors.New("the provider for this model is disabled")
	}
	tc.billable = tc.provider.IsGlobal()
	if !tc.billable {
		owner := tc.provider.OwnerUserID.Int64
		switch {
		case owner == run.AuthorUserID:
			// spending your own key
		case owner == tc.thread.OwnerUserID && tc.thread.AllowOwnerKey:
			// owner allowed members to use their key
		default:
			return nil, errors.New("this chat uses the owner's personal API key and the owner has not allowed members to use it")
		}
	}
	return tc, nil
}

func (e *Engine) checkQuota(userID int64) error {
	q, err := e.st.QuotaFor(userID)
	if err != nil || q.TokenLimit == 0 {
		return err
	}
	used, err := e.st.UsedTokens(userID, q.Period)
	if err != nil {
		return err
	}
	if used >= q.TokenLimit {
		return fmt.Errorf("token quota exceeded (%d of %d used this %s period)", used, q.TokenLimit, q.Period)
	}
	return nil
}

func (e *Engine) autoTitle(tc *turnCtx) {
	if tc.thread.Title != "New chat" {
		return
	}
	msgs, err := e.st.RunMessages(tc.run.ID)
	if err != nil || len(msgs) == 0 {
		return
	}
	title := strings.Join(strings.Fields(msgs[0].Content), " ")
	if title == "" {
		title = "Attachment"
		if atts, _ := e.st.AttachmentsByMessage([]int64{msgs[0].ID}); len(atts[msgs[0].ID]) > 0 {
			a := atts[msgs[0].ID][0]
			if a.IsImage() {
				title = "Image"
			} else if a.OrigName != "" {
				title = a.OrigName
			}
		}
	}
	if r := []rune(title); len(r) > 48 {
		title = string(r[:48]) + "…"
	}
	if title != "" {
		e.st.SetThreadTitle(tc.thread.ID, title)
		tc.thread.Title = title
	}
}

// SystemPrompt assembles the system message for a thread: the global prompt,
// how to read the time stamps on user messages (their clock is loc; nil = UTC),
// custom instructions and group-chat guidance. It contains no date or time
// itself, so it stays identical between turns (see clock.go).
func SystemPrompt(global string, t *store.Thread, ownerInstr string, members []store.Member, loc *time.Location) string {
	var sys strings.Builder
	sys.WriteString(global)
	sys.WriteString("\n\n" + clockGuide(loc, len(members) > 1))
	if ownerInstr != "" {
		sys.WriteString("\n\nThe user's custom instructions:\n" + ownerInstr)
	}
	if ti := strings.TrimSpace(t.Instructions); ti != "" {
		sys.WriteString("\n\nInstructions for this chat:\n" + ti)
	}
	if len(members) > 1 {
		names := make([]string, 0, len(members))
		for _, m := range members {
			names = append(names, m.Username)
		}
		fmt.Fprintf(&sys, "\n\nThis is a group conversation between several people (%s) and you. "+
			"After the time line, each user message gives the sender's name in brackets, like [name]. "+
			"Address people by name when it helps. Never start your own reply with a bracketed name.",
			strings.Join(names, ", "))
	}
	return sys.String()
}

// Turns whose attachments are still sent in full. Older ones become a short
// note so long chats do not resend every image and document each time.
const (
	imageTurns = 3
	fileTurns  = 6
)

// buildContext assembles: system prompt, summary, recent history, current turn.
func (e *Engine) buildContext(tc *turnCtx) ([]llm.Message, error) {
	msgs := []llm.Message{{Role: "system", Content: SystemPrompt(tc.settings.SystemPrompt, tc.thread, tc.ownerInstr, tc.members, tc.loc)}}
	if tc.thread.Summary != "" {
		msgs = append(msgs, llm.Message{Role: "system",
			Content: "Summary of the earlier conversation (older messages were compacted):\n\n" + tc.thread.Summary})
	}

	history, err := e.st.HistoryMessages(tc.thread.ID, tc.thread.CompactedUntilRun, tc.run.ID)
	if err != nil {
		return nil, err
	}
	current, err := e.st.RunMessages(tc.run.ID)
	if err != nil {
		return nil, err
	}
	all := append(history, current...)

	var userIDs []int64
	for _, m := range all {
		if m.Role == "user" {
			userIDs = append(userIDs, m.ID)
		}
	}
	atts, err := e.st.AttachmentsByMessage(userIDs)
	if err != nil {
		return nil, err
	}
	// Which runs are "recent" for images / files, counting back from the current one.
	imgRecent := map[int64]bool{tc.run.ID: true}
	fileRecent := map[int64]bool{tc.run.ID: true}
	for i := len(history) - 1; i >= 0; i-- {
		if len(imgRecent) <= imageTurns {
			imgRecent[history[i].RunID] = true
		}
		if len(fileRecent) <= fileTurns {
			fileRecent[history[i].RunID] = true
		}
	}

	now := timeStamp(time.Now(), tc.loc)
	for i, m := range all {
		// Earlier messages keep the time they were sent, so their text never
		// changes; the turn being answered carries the time right now.
		stamp := now
		if i < len(history) {
			stamp = messageStamp(m.CreatedAt, tc.loc)
		}
		lm := toLLM(m, tc.group, stamp)
		files := atts[m.ID]
		if len(files) > 0 {
			lm.Content = attachContext(e, lm, files, tc.model.Vision, imgRecent[m.RunID], fileRecent[m.RunID])
		}
		msgs = append(msgs, *lm)
	}
	return msgs, nil
}

// attachContext adds a message's attachments to its LLM form: images become
// image parts (vision models, recent turns), documents become inline text.
func attachContext(e *Engine, lm *llm.Message, files []store.Attachment, vision, imgRecent, fileRecent bool) string {
	var notes []string
	var docs strings.Builder
	nImages := 0
	for i := range files {
		f := &files[i]
		if f.IsImage() {
			nImages++
			if vision && imgRecent {
				url, err := e.st.DataURL(f)
				if err != nil {
					e.log.Warn("attachment missing", "file", f.FileName, "err", err)
					continue
				}
				lm.Images = append(lm.Images, url)
			}
			continue
		}
		if fileRecent && f.Text != "" {
			fmt.Fprintf(&docs, "\n\n<attached_file name=%q>\n%s\n</attached_file>", f.OrigName, f.Text)
		} else {
			notes = append(notes, fmt.Sprintf("[file attached earlier: %s]", f.OrigName))
		}
	}
	if nImages > 0 && !(vision && imgRecent) {
		note := fmt.Sprintf("[%d image(s) were attached here", nImages)
		if !vision {
			note += "; this model cannot see images"
		}
		notes = append(notes, note+"]")
	}
	out := lm.Content
	if len(notes) > 0 {
		out = strings.TrimSpace(out + "\n" + strings.Join(notes, "\n"))
	}
	return out + docs.String()
}

// toLLM turns a stored message into its form for the model. A user message
// gets stamp (the time it was sent, "" for none) on a line of its own.
func toLLM(m store.Message, group bool, stamp string) *llm.Message {
	switch m.Role {
	case "user":
		c := m.Content
		if group {
			c = "[" + m.AuthorName + "]: " + c
		}
		if stamp != "" {
			c = stamp + "\n" + c
		}
		return &llm.Message{Role: "user", Content: c}
	case "tool":
		return &llm.Message{Role: "tool", Content: m.Content, ToolCallID: m.ToolCallID}
	default:
		out := &llm.Message{Role: "assistant", Content: m.Content}
		if m.ToolCalls != "" {
			json.Unmarshal([]byte(m.ToolCalls), &out.ToolCalls)
		}
		return out
	}
}

// streamStep makes one LLM call, streaming into a new assistant message.
// It retries 429/5xx/network errors, but only before the first token arrives.
// It returns the result and the id of the assistant message it created.
func (e *Engine) streamStep(ctx context.Context, tc *turnCtx, msgs []llm.Message, defs []llm.Tool) (*llm.Result, int64, error) {
	threadID := tc.thread.ID
	msgID, err := e.st.CreateMessage(&store.Message{
		ThreadID: threadID, RunID: tc.run.ID, Role: "assistant", Status: store.MsgStreaming,
	})
	if err != nil {
		return nil, 0, err
	}
	e.hub.Publish(threadID, EvMessages, nil)
	lm := e.beginLive(threadID, msgID)
	defer e.endLive(threadID)

	lastSave := time.Now()
	gotTokens := false
	onDelta := func(d string) {
		gotTokens = true
		n, full := lm.append(d)
		e.hub.Publish(threadID, EvToken, map[string]any{"message_id": msgID, "delta": d, "len": n})
		if time.Since(lastSave) > time.Second { // persist so a reload shows progress
			_, reasoning := lm.text()
			e.st.UpdateMessageFull(msgID, full, reasoning, "", store.MsgStreaming)
			lastSave = time.Now()
		}
	}
	onReasoning := func(d string) {
		gotTokens = true
		n := lm.appendReasoning(d)
		e.hub.Publish(threadID, EvToken, map[string]any{"message_id": msgID, "delta": d, "len": n, "kind": "reasoning"})
	}

	req := llm.Request{BaseURL: tc.provider.BaseURL, APIKey: tc.provider.APIKey, Model: tc.model.Name,
		Messages: msgs, Tools: defs, OnReasoning: onReasoning, IdleTimeout: idleTimeout(tc.settings)}
	var res *llm.Result
	for attempt := 0; ; attempt++ {
		res, err = llm.Stream(ctx, req, onDelta)
		if err == nil || gotTokens || attempt >= tc.settings.LLMMaxRetries || !llm.IsRetryable(err) {
			break
		}
		wait := llm.RetryDelay(err, attempt)
		e.log.Info("retrying LLM call", "thread", threadID, "attempt", attempt+1, "wait", wait, "err", err)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			err = ctx.Err()
		}
		if ctx.Err() != nil {
			break
		}
	}

	if err != nil {
		partial, reasoning := lm.text()
		if partial == "" && reasoning == "" {
			e.st.DeleteMessage(msgID)
		} else {
			status := store.MsgFailed
			if ctx.Err() != nil {
				status = store.MsgInterrupted
			}
			e.st.UpdateMessageFull(msgID, partial, reasoning, "", status)
		}
		e.hub.Publish(threadID, EvMessages, nil)
		return nil, msgID, err
	}

	toolJSON := ""
	if len(res.ToolCalls) > 0 {
		b, _ := json.Marshal(res.ToolCalls)
		toolJSON = string(b)
	}
	if res.Content == "" && toolJSON == "" {
		res.Content = "_(empty response)_"
	}
	if err := e.st.UpdateMessageFull(msgID, res.Content, res.Reasoning, toolJSON, store.MsgDone); err != nil {
		return nil, msgID, err
	}
	e.recordUsage(tc, "chat", tc.run.AuthorUserID, tc.model, tc.provider, msgs, res)
	e.hub.Publish(threadID, EvMessages, nil)
	return res, msgID, nil
}

// idleTimeout turns the admin setting into the llm package's form.
func idleTimeout(st store.Settings) time.Duration {
	if st.LLMIdleSeconds <= 0 {
		return -1 // never give up
	}
	return time.Duration(st.LLMIdleSeconds) * time.Second
}

func (e *Engine) recordUsage(tc *turnCtx, kind string, userID int64, m *store.Model, p *store.Provider, in []llm.Message, res *llm.Result) {
	entry := store.UsageEntry{
		UserID: userID, ThreadID: tc.thread.ID, ProviderID: p.ID, ModelName: m.Name,
		Kind: kind, Billable: p.IsGlobal(),
	}
	if res.Usage != nil {
		entry.PromptTokens, entry.CompletionTokens = res.Usage.PromptTokens, res.Usage.CompletionTokens
	} else {
		entry.Estimated = true
		for _, msg := range in {
			entry.PromptTokens += llm.EstimateTokens(msg.Content) + 4 + 800*len(msg.Images) // ~800 tokens/image
		}
		out := res.Content + res.Reasoning
		for _, c := range res.ToolCalls {
			out += c.Function.Name + c.Function.Arguments
		}
		entry.CompletionTokens = llm.EstimateTokens(out)
	}
	entry.Cost = (float64(entry.PromptTokens)*m.InputPrice + float64(entry.CompletionTokens)*m.OutputPrice) / 1e6
	if err := e.st.RecordUsage(entry); err != nil {
		e.log.Error("record usage", "err", err)
	}
}

// ContextEstimate reports roughly how full the model's context window is for
// the next turn of a thread (tokens used, window size). Used by the UI meter.
func (e *Engine) ContextEstimate(t *store.Thread) (used, window int) {
	st, err := e.st.LoadSettings()
	if err != nil || !t.ModelID.Valid {
		return 0, 0
	}
	m, err := e.st.ModelByID(t.ModelID.Int64)
	if err != nil {
		return 0, 0
	}
	if a, _ := e.st.AgentByThread(t.ID); a != nil { // agents rebuild their context every run
		return llm.EstimateTokens(e.agentPrompt(a)), m.ContextWindow
	}
	members, _ := e.st.ThreadMembers(t.ID)
	used = llm.EstimateTokens(SystemPrompt(st.SystemPrompt, t, "", members, nil)) + llm.EstimateTokens(t.Summary)
	history, err := e.st.HistoryMessages(t.ID, t.CompactedUntilRun, 1<<62)
	if err != nil {
		return 0, 0
	}
	for _, h := range history {
		used += llm.EstimateTokens(h.Content) + 4
	}
	return used, m.ContextWindow
}
