package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"knitnox/internal/engine"
	"knitnox/internal/hub"
	"knitnox/internal/store"
)

// What a person can be notified about. Each kind has a switch under
// Settings → Notifications; all are on until switched off. Every notification
// opens the exact place it is about: the answer in its chat, the post in the
// Feed, the task in the Plan Book.
const (
	pushReply    = "reply"    // an answer in one of your chats is ready, or failed
	pushApproval = "approval" // a tool waits for your yes or no
	pushAgent    = "agent"    // one of your agents finished a run, or could not
	pushFeed     = "feed"     // a new post in your feed that you did not write
	pushPlan     = "plan"     // the AI or an agent changed your Plan Book; tasks due today
)

type pushKindView struct {
	Key, Label, Hint string
	On               bool
}

var pushKinds = []pushKindView{
	{Key: pushReply, Label: "Answers in my chats", Hint: "An answer is ready, or it failed, after you left the chat."},
	{Key: pushApproval, Label: "Tool approvals", Hint: "The AI wants to run a tool and waits for your yes or no."},
	{Key: pushAgent, Label: "My agents", Hint: "An agent finished a run, or could not finish it."},
	{Key: pushFeed, Label: "Feed", Hint: "The AI or an agent posted to your feed, or someone shared a post with everyone."},
	{Key: pushPlan, Label: "Plan Book", Hint: "The AI or an agent added a task or ticked one off, and a reminder in the morning when tasks are due today."},
}

// The kinds a person switched off are kept as one line of text, "feed,plan".
func pushOffKey(userID int64) string { return "push_off:" + strconv.FormatInt(userID, 10) }

func (s *Server) pushWanted(userID int64, kind string) bool {
	for _, off := range strings.Split(s.st.GetValue(pushOffKey(userID)), ",") {
		if off == kind {
			return false
		}
	}
	return true
}

func (s *Server) pushKindViews(userID int64) []pushKindView {
	out := make([]pushKindView, len(pushKinds))
	for i, k := range pushKinds {
		k.On = s.pushWanted(userID, k.Key)
		out[i] = k
	}
	return out
}

// pushKindsSave keeps which kinds of notification the person wants.
func (s *Server) pushKindsSave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	on := map[string]bool{}
	for _, k := range r.Form["kind"] {
		on[k] = true
	}
	var off []string
	for _, k := range pushKinds {
		if !on[k.Key] {
			off = append(off, k.Key)
		}
	}
	if err := s.st.SetValue(pushOffKey(currentUser(r).ID), strings.Join(off, ",")); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings#notifications", http.StatusSeeOther)
}

// pushKind sends a notification of one kind, unless the person switched that kind off.
func (s *Server) pushKind(userID int64, kind string, p pushPayload) {
	if s.pushWanted(userID, kind) {
		s.sendPush(userID, p)
	}
}

// onRunFinished notifies the author when a reply finishes while they are away.
// It opens the chat at the answer.
func (s *Server) onRunFinished(ev engine.RunEvent) {
	if ev.AuthorID == 0 || s.isWatching(ev.ThreadID, ev.AuthorID) {
		return
	}
	t, err := s.st.ThreadByID(ev.ThreadID)
	if err != nil {
		return
	}
	kind := pushReply
	agent, _ := s.st.AgentByThread(ev.ThreadID) // the title already is the agent's name
	if agent != nil {
		kind = pushAgent
	}
	p := pushPayload{Title: t.Title, URL: fmt.Sprintf("/t/%d", ev.ThreadID), Tag: fmt.Sprintf("run-%d", ev.ThreadID)}
	switch ev.Status {
	case store.RunDone:
		p.Body = "Your reply is ready."
		if agent != nil {
			p.Body = "Finished a run."
		}
		if msgs, err := s.st.RunMessages(ev.RunID); err == nil {
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == "assistant" && msgs[i].Content != "" {
					p.Body = preview(msgs[i].Content, 120)
					p.URL += fmt.Sprintf("#m%d", msgs[i].ID)
					break
				}
			}
		}
	case store.RunFailed:
		p.Body = "The reply failed: " + preview(ev.Error, 100)
		if agent != nil {
			p.Body = "Could not finish its run: " + preview(ev.Error, 100)
		}
	default:
		return
	}
	s.pushKind(ev.AuthorID, kind, p)
}

// onApproval asks the people who may approve a tool call, if they are away.
// It opens the chat at the question.
func (s *Server) onApproval(ev engine.ApprovalEvent) {
	t, err := s.st.ThreadByID(ev.ThreadID)
	if err != nil {
		return
	}
	for _, uid := range ev.Approvers {
		if s.isWatching(ev.ThreadID, uid) {
			continue
		}
		s.pushKind(uid, pushApproval, pushPayload{
			Title: "Tool approval needed · " + t.Title,
			Body:  fmt.Sprintf("Knitnox wants to run %s on %s. Open the chat to allow or deny it.", ev.ToolName, ev.ServerName),
			URL:   fmt.Sprintf("/t/%d#approval-%d", ev.ThreadID, ev.ApprovalID), Tag: fmt.Sprintf("approval-%d", ev.ApprovalID),
		})
	}
}

// feedOpen reports whether the person has the Feed page open right now.
func (s *Server) feedOpen(userID int64) bool { return s.isWatching(hub.FeedUser(userID), userID) }

// onFeedPost tells the owner about a post the AI or an agent put on their
// feed, and everyone else about a post that was shared. It opens the Feed at the post.
func (s *Server) onFeedPost(ev engine.FeedEvent) {
	if ev.By.Kind != store.FeedByUser && !s.feedOpen(ev.UserID) &&
		!(ev.By.ThreadID != 0 && s.isWatching(ev.By.ThreadID, ev.UserID)) { // asked for in a chat that is open: the answer says so
		if p, err := s.st.FeedPostByID(ev.UserID, ev.PostID); err == nil {
			title := "New post in your feed"
			if ev.By.Kind == store.FeedByAgent && ev.By.Name != "" {
				title = ev.By.Name + " posted to your feed"
			}
			s.pushKind(ev.UserID, pushFeed, pushPayload{Title: title, Body: preview(p.Body, 140),
				URL: fmt.Sprintf("/feed#fp-%d", p.ID), Tag: fmt.Sprintf("feed-%d", p.ID)})
		}
	}
	if ev.Public {
		s.pushShared(ev.UserID, ev.PostID)
	}
}

// pushShared tells everyone but the author that a post was shared with them.
func (s *Server) pushShared(authorID, postID int64) {
	if !s.feedPublicOn() {
		return
	}
	p, err := s.st.FeedPostByID(authorID, postID)
	if err != nil {
		return
	}
	for _, uid := range s.st.PushUserIDs() {
		if uid == authorID || s.feedOpen(uid) {
			continue
		}
		s.pushKind(uid, pushFeed, pushPayload{Title: p.Username + " shared a post", Body: preview(p.Body, 140),
			URL: fmt.Sprintf("/feed?view=%s#fp-%d", store.FeedViewPublic, p.ID), Tag: fmt.Sprintf("feed-%d", p.ID)})
	}
}

// onPlanTask tells the owner that the AI or an agent added a task to their
// Plan Book or ticked one off. It opens the Plan Book at the task.
func (s *Server) onPlanTask(ev engine.PlanEvent) {
	if ev.By.Kind == store.PlanByUser || (ev.By.ThreadID != 0 && s.isWatching(ev.By.ThreadID, ev.UserID)) {
		return
	}
	t, err := s.st.PlanTaskByID(ev.UserID, ev.TaskID)
	if err != nil {
		return
	}
	who := "The AI"
	if ev.By.Kind == store.PlanByAgent && ev.By.Name != "" {
		who = ev.By.Name
	}
	p := pushPayload{Title: "Plan Book", Body: who + " added a task: " + preview(t.Title, 100),
		URL: fmt.Sprintf("/plan#pt-%d", t.ID), Tag: fmt.Sprintf("plan-%d", t.ID)}
	if ev.Done {
		p.Body = who + " ticked off: " + preview(t.Title, 100)
		p.URL = fmt.Sprintf("/plan?view=%s#pt-%d", store.PlanViewDone, t.ID)
	} else if t.Due != "" {
		p.Body += " (due " + t.Due + ")"
	}
	s.pushKind(ev.UserID, pushPlan, p)
}

// planReminders tells each person once a day, in their morning, how many
// tasks are due today. It runs every hour; the day it last spoke is kept per person.
// It opens the Plan Book on "Today".
func (s *Server) planReminders(now time.Time) {
	for _, uid := range s.st.PushUserIDs() {
		if !s.pushWanted(uid, pushPlan) {
			continue
		}
		local := now.In(s.st.MustLocation(uid))
		if h := local.Hour(); h < 8 || h >= 21 { // not at night, also when the server starts late
			continue
		}
		today, key := local.Format("2006-01-02"), "push_due:"+strconv.FormatInt(uid, 10)
		if s.st.GetValue(key) == today {
			continue
		}
		s.st.SetValue(key, today)
		c := s.st.PlanCounts(uid, 0, today)
		if c.Today == 0 {
			continue
		}
		body := fmt.Sprintf("%d tasks are due today.", c.Today)
		if c.Today == 1 {
			body = "1 task is due today."
		}
		if c.Overdue > 0 {
			body = fmt.Sprintf("%d open tasks for today, %d of them late.", c.Today, c.Overdue)
			if c.Today == 1 {
				body = "1 task is late."
			}
		}
		s.sendPush(uid, pushPayload{Title: "Plan Book", Body: body,
			URL: "/plan?view=" + store.PlanViewToday, Tag: "plan-due-" + today})
	}
}
