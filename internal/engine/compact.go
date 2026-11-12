package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"knitnox/internal/llm"
)

const compactionPrompt = `You maintain the running memory of a long chat. Merge the existing summary and the new
conversation excerpt into ONE updated summary. Keep: facts about the participants, decisions,
open questions, requirements, names, numbers, code identifiers and anything the assistant
promised. Drop greetings and repetition. Write compact bullet points grouped by topic.
Return only the summary.`

// maybeCompact summarises older turns when the thread has more finished
// pairs than CompactionPairs, or when the estimated context exceeds
// CompactionContextRatio of the model's window. The most recent
// CompactionKeep pairs always stay verbatim. Original messages are never
// deleted; only threads.summary / compacted_until_run change.
func (e *Engine) maybeCompact(ctx context.Context, tc *turnCtx) error {
	st := tc.settings
	t := tc.thread
	keep := max(st.CompactionKeep, 1)

	runIDs, err := e.st.FinishedRunIDs(t.ID, t.CompactedUntilRun, tc.run.ID)
	if err != nil {
		return err
	}
	pairs := len(runIDs)
	if pairs <= keep {
		return nil
	}
	need := st.CompactionPairs > 0 && pairs > st.CompactionPairs
	if !need && tc.model.ContextWindow > 0 && st.CompactionContextRatio > 0 {
		history, err := e.st.HistoryMessages(t.ID, t.CompactedUntilRun, tc.run.ID)
		if err != nil {
			return err
		}
		est := llm.EstimateTokens(t.Summary) + llm.EstimateTokens(st.SystemPrompt)
		for _, m := range history {
			est += llm.EstimateTokens(m.Content) + 4
		}
		need = float64(est) > st.CompactionContextRatio*float64(tc.model.ContextWindow)
	}
	if !need {
		return nil
	}

	cut := runIDs[pairs-keep-1] // last run folded into the summary
	old, err := e.st.HistoryMessages(t.ID, t.CompactedUntilRun, cut+1)
	if err != nil {
		return err
	}

	var transcript strings.Builder
	for _, m := range old {
		who := "Assistant"
		if m.Role == "user" {
			who = "User"
			if m.AuthorName != "" {
				who = m.AuthorName
			}
		}
		c := m.Content
		if c == "" {
			c = "(sent an image)"
		}
		if r := []rune(c); len(r) > 6000 {
			c = string(r[:6000]) + " …[truncated]"
		}
		fmt.Fprintf(&transcript, "%s: %s\n\n", who, c)
	}

	model, provider := tc.model, tc.provider
	if st.CompactionModelID > 0 {
		if m, err := e.st.ModelByID(st.CompactionModelID); err == nil && m.Enabled {
			if p, err := e.st.ProviderByID(m.ProviderID); err == nil && p.Enabled && p.IsGlobal() {
				model, provider = m, p
			}
		}
	}

	existing := t.Summary
	if existing == "" {
		existing = "(none yet)"
	}
	msgs := []llm.Message{
		{Role: "system", Content: compactionPrompt},
		{Role: "user", Content: "EXISTING SUMMARY:\n" + existing + "\n\nNEW CONVERSATION TO MERGE:\n\n" + transcript.String()},
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	var res *llm.Result
	for attempt := 0; ; attempt++ {
		res, err = llm.Complete(cctx, llm.Request{BaseURL: provider.BaseURL, APIKey: provider.APIKey, Model: model.Name, Messages: msgs, IdleTimeout: idleTimeout(st)})
		if err == nil || attempt >= st.LLMMaxRetries || !llm.IsRetryable(err) {
			break
		}
		select {
		case <-time.After(llm.RetryDelay(err, attempt)):
		case <-cctx.Done():
			return cctx.Err()
		}
	}
	if err != nil {
		return err
	}
	summary := strings.TrimSpace(res.Content)
	if summary == "" {
		return errors.New("compaction returned an empty summary")
	}
	// Compaction is paid by the thread owner.
	e.recordUsage(tc, "compaction", t.OwnerUserID, model, provider, msgs, res)
	if err := e.st.SetSummary(t.ID, summary, cut); err != nil {
		return err
	}
	t.Summary, t.CompactedUntilRun = summary, cut
	e.log.Info("thread compacted", "thread", t.ID, "pairs_folded", pairs-keep)
	e.hub.Publish(t.ID, EvCompacted, map[string]any{"until_run": cut})
	return nil
}
