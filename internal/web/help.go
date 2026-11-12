package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"knitnox/docs"
	"knitnox/internal/store"
)

// Help: the guides from docs/*.md (rendered as Markdown in the browser) and a
// small issue tracker that every signed-in user can read and write.

type helpDoc struct {
	Slug, Title, File string
}

var helpDocs = []helpDoc{
	{"guide", "User guide", "USER_GUIDE.md"},
	{"features", "Features", "FEATURES.md"},
	{"developers", "Developer guide", "DEVELOPER_GUIDE.md"},
	{"build", "Build guide", "BUILD_GUIDE.md"},
}

func (s *Server) helpPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("doc")
	for _, d := range helpDocs {
		if d.Slug != slug {
			continue
		}
		body, err := docs.FS.ReadFile(d.File)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		// The page shows the title itself, so drop the file's first "# Title" line.
		text := strings.ReplaceAll(string(body), "\r\n", "\n")
		if rest, ok := strings.CutPrefix(text, "# "); ok {
			if _, after, found := strings.Cut(rest, "\n"); found {
				text = strings.TrimLeft(after, "\n")
			}
		}
		open, _ := s.st.IssueCounts()
		s.render(w, r, "help", map[string]any{"Tab": d.Slug, "Docs": helpDocs, "DocTitle": d.Title, "Body": text, "OpenIssues": open})
		return
	}
	s.errorPage(w, r, http.StatusNotFound, "That guide does not exist.")
}

// issueView adds what the current user may do with an issue.
type issueView struct {
	store.Issue
	CanChange bool // author or admin: close, reopen, delete
}

func (s *Server) issuesPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	show := r.URL.Query().Get("show")
	var list []store.Issue
	var err error
	switch show {
	case "done":
		list, err = s.st.ListIssues(store.IssueDone, 0)
	case "mine":
		list, err = s.st.ListIssues("", u.ID)
	default:
		show = "open"
		list, err = s.st.ListIssues(store.IssueOpen, 0)
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	views := make([]issueView, 0, len(list))
	for _, i := range list {
		views = append(views, issueView{Issue: i, CanChange: i.UserID == u.ID || u.IsAdmin()})
	}
	open, done := s.st.IssueCounts()
	s.render(w, r, "help", map[string]any{"Tab": "issues", "Docs": helpDocs, "Issues": views, "Show": show,
		"OpenIssues": open, "DoneIssues": done})
}

func (s *Server) issueCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if strings.TrimSpace(r.FormValue("title")) == "" {
		s.errorPage(w, r, http.StatusBadRequest, "Give the issue a short title.")
		return
	}
	if _, err := s.st.CreateIssue(u.ID, u.Username, r.FormValue("kind"), r.FormValue("title"), r.FormValue("body")); err != nil {
		if errors.Is(err, store.ErrTooManyIssues) {
			s.errorPage(w, r, http.StatusTooManyRequests,
				fmt.Sprintf("You already have %d open issues. Close some before adding more.", store.MaxOpenIssuesPerUser))
			return
		}
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/issues", http.StatusSeeOther)
}

// issueAction handles: done | reopen | delete (author or admin), reply (admin only).
func (s *Server) issueAction(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	issue, err := s.st.IssueByID(pathID(r, "iid"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "That issue no longer exists.")
		return
	}
	mine := issue.UserID == u.ID
	action := r.FormValue("action")
	if (action == "reply" && !u.IsAdmin()) || (action != "reply" && !mine && !u.IsAdmin()) {
		s.errorPage(w, r, http.StatusForbidden, "Only the person who reported this or an admin can do that.")
		return
	}
	switch action {
	case "done":
		err = s.st.SetIssueStatus(issue.ID, store.IssueDone)
	case "reopen":
		err = s.st.SetIssueStatus(issue.ID, store.IssueOpen)
	case "reply":
		if err = s.st.SetIssueReply(issue.ID, r.FormValue("reply"), u.Username); err == nil && r.FormValue("close") == "1" {
			err = s.st.SetIssueStatus(issue.ID, store.IssueDone)
		}
	case "delete":
		if err = s.st.DeleteIssue(issue.ID); err == nil && !mine {
			s.audit(r, "issue.delete", issue.Title, "by "+issue.AuthorName)
		}
	default:
		s.errorPage(w, r, http.StatusBadRequest, "Unknown action.")
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	back(w, r, "/issues")
}
