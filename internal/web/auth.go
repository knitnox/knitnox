package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"knitnox/internal/store"
)

const (
	sessionCookie   = "knitnox_session"
	oldCookie       = "parley_session" // accepted from before the rename
	sessionTTL      = 30 * 24 * time.Hour
	maxRequestBytes = 48 << 20 // one message with its attachments (up to 6 files; documents may be 20 MB each)
)

type ctxKey int

const (
	userKey ctxKey = iota
	csrfKey
)

func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

func csrfToken(r *http.Request) string {
	t, _ := r.Context().Value(csrfKey).(string)
	return t
}

// requireUser loads the session and enforces CSRF on unsafe methods.
func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := sessionFromRequest(r)
		if err != nil {
			s.toLogin(w, r)
			return
		}
		u, csrf, err := s.st.SessionUser(c.Value)
		if err != nil {
			s.toLogin(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes) // before anything parses the form
			got := r.Header.Get("X-CSRF-Token")
			if got == "" {
				got = r.FormValue("_csrf")
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(csrf)) != 1 {
				http.Error(w, "invalid CSRF token: reload the page", http.StatusForbidden)
				return
			}
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		ctx = context.WithValue(ctx, csrfKey, csrf)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin() {
			s.errorPage(w, r, http.StatusForbidden, "Admins only.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) toLogin(w http.ResponseWriter, r *http.Request) {
	if isHX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodGet && !strings.HasSuffix(r.URL.Path, "/events") {
		http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
		return
	}
	http.Error(w, "not signed in", http.StatusUnauthorized)
}

func (s *Server) setSession(w http.ResponseWriter, userID int64) error {
	tok, err := s.st.CreateSession(userID, sessionTTL)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true, Secure: s.secure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(sessionTTL.Seconds()),
	})
	return nil
}

func safeNext(n string) string {
	if strings.HasPrefix(n, "/") && !strings.HasPrefix(n, "//") && !strings.HasPrefix(n, "/\\") {
		return n
	}
	return "/"
}

// ---- pages ----

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	n, _ := s.st.CountUsers()
	if n == 0 {
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	st, _ := s.st.LoadSettings()
	s.render(w, r, "auth", map[string]any{"Mode": "login", "Next": r.URL.Query().Get("next"), "AllowSignup": st.AllowSignup})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	keys := loginKeys(clientIP(r), username)
	st, _ := s.st.LoadSettings()
	fail := func(code int, msg string) {
		w.WriteHeader(code)
		s.render(w, r, "auth", map[string]any{"Mode": "login", "Error": msg, "Username": username,
			"Next": r.FormValue("next"), "AllowSignup": st.AllowSignup})
	}
	if locked, left := s.logins.blocked(keys...); locked {
		fail(http.StatusTooManyRequests, fmt.Sprintf("Too many failed attempts. Try again in %d minutes.", int(left.Minutes())+1))
		return
	}
	u, hash, err := s.st.UserAuth(username)
	if err != nil {
		hash = dummyHash // spend the same time as for a real account, so a name cannot be tested by the clock
	}
	right := bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.FormValue("password"))) == nil && err == nil
	if right && u.Disabled { // said only to someone who knows the password
		fail(http.StatusUnauthorized, "This account is disabled. Ask an administrator.")
		return
	}
	if right {
		s.logins.reset(keys[0], keys[2]) // this person's own counters; the address keeps its count
		if err := s.setSession(w, u.ID); err != nil {
			s.serverError(w, r, err)
			return
		}
		http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
		return
	}
	s.logins.fail(keys...)
	fail(http.StatusUnauthorized, "Wrong username or password.")
}

// dummyHash is compared against when the username does not exist.
var dummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("no such account"), bcrypt.DefaultCost)
	return string(h)
}()

// logoutEverywhere deletes every session of the user, including this one.
func (s *Server) logoutEverywhere(w http.ResponseWriter, r *http.Request) {
	s.st.DeleteUserSessions(currentUser(r).ID)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: oldCookie, Value: "", Path: "/", MaxAge: -1})
	if isHX(r) {
		w.Header().Set("HX-Redirect", "/login")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) registerPage(w http.ResponseWriter, r *http.Request) {
	n, _ := s.st.CountUsers()
	st, _ := s.st.LoadSettings()
	if n > 0 && !st.AllowSignup {
		s.render(w, r, "auth", map[string]any{"Mode": "login", "Error": "Sign-up is closed. Ask an admin for an account."})
		return
	}
	s.render(w, r, "auth", map[string]any{"Mode": "register", "First": n == 0, "Next": r.URL.Query().Get("next")})
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

func validateCredentials(username, password string) error {
	if !usernameRe.MatchString(username) {
		return errors.New("Username must be 3–32 characters: letters, digits, _ . -")
	}
	if len(password) < 8 {
		return errors.New("Password must be at least 8 characters.")
	}
	return nil
}

func hashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(h), err
}

// passwordProblem turns a hashing error into words for the person typing.
func passwordProblem(err error) string {
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return "That password is too long. Use at most 72 characters."
	}
	return "That password could not be used: " + err.Error()
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	n, _ := s.st.CountUsers()
	st, _ := s.st.LoadSettings()
	if n > 0 && !st.AllowSignup {
		s.errorPage(w, r, http.StatusForbidden, "Sign-up is closed.")
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	fail := func(msg string) {
		w.WriteHeader(http.StatusBadRequest)
		s.render(w, r, "auth", map[string]any{"Mode": "register", "First": n == 0, "Error": msg, "Username": username, "Next": r.FormValue("next")})
	}
	if err := validateCredentials(username, password); err != nil {
		fail(err.Error())
		return
	}
	if r.FormValue("password2") != password {
		fail("The two passwords are not the same. Type the same password in both boxes.")
		return
	}
	hash, err := hashPassword(password)
	if err != nil {
		fail(passwordProblem(err))
		return
	}
	// Sign-ups are counted per address, so an open sign-up page cannot be used
	// to fill the server with accounts.
	signupKey := "signup:" + clientIP(r)
	if locked, left := s.logins.blocked(signupKey); locked {
		fail(fmt.Sprintf("Too many accounts were made from your network. Try again in %d minutes.", int(left.Minutes())+1))
		return
	}
	id, role, err := s.st.CreateSignupUser(username, hash) // the first account administers the instance
	if err != nil {
		fail("That username is taken.")
		return
	}
	s.logins.fail(signupKey)
	if role != "admin" && st.SignupTokenLimit > 0 { // the allowance the admin chose for people who sign up
		if err := s.st.SetQuota(id, store.Quota{Period: st.SignupPeriod, TokenLimit: st.SignupTokenLimit}); err != nil {
			s.log.Error("could not set the sign-up allowance", "user", id, "err", err)
		}
	}
	if err := s.setSession(w, id); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := sessionFromRequest(r); err == nil {
		s.st.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: oldCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	hash, err := s.st.PasswordHash(u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.FormValue("current"))) != nil {
		s.flash(w, r, "Current password is wrong.")
		return
	}
	if len(r.FormValue("new")) < 8 {
		s.flash(w, r, "New password must be at least 8 characters.")
		return
	}
	if r.FormValue("new2") != r.FormValue("new") {
		s.flash(w, r, "The two new passwords are not the same.")
		return
	}
	nh, err := hashPassword(r.FormValue("new"))
	if err != nil {
		s.flash(w, r, passwordProblem(err))
		return
	}
	if err := s.st.SetPassword(u.ID, nh); err != nil {
		s.serverError(w, r, err)
		return
	}
	// Whoever else was signed in with the old password is signed out; this device stays.
	if c, err := sessionFromRequest(r); err == nil {
		if err := s.st.DeleteOtherSessions(u.ID, c.Value); err != nil {
			s.log.Warn("could not sign out other devices after a password change", "user", u.ID, "err", err)
		}
	}
	s.flash(w, r, "Password changed. Other devices were signed out.")
}

// usernameProblem turns a rename error into words for the person typing ("" = not one of ours).
func usernameProblem(err error) string {
	switch {
	case errors.Is(err, store.ErrBadUsername):
		return "A username is 3 to 32 characters: letters, digits, _ . -"
	case errors.Is(err, store.ErrUsernameTaken):
		return "That username is taken."
	}
	return ""
}

// changeUsername lets a signed-in person pick a new username. The current
// password is asked for, so a device left unlocked cannot be used to do it.
func (s *Server) changeUsername(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	hash, err := s.st.PasswordHash(u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.FormValue("current"))) != nil {
		s.flash(w, r, "Current password is wrong.")
		return
	}
	name := strings.TrimSpace(r.FormValue("username"))
	if name == u.Username {
		s.flash(w, r, "That is already your username.")
		return
	}
	if err := s.st.SetUsername(u.ID, name); err != nil {
		if msg := usernameProblem(err); msg != "" {
			s.flash(w, r, msg)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.st.Audit(u.ID, name, "user.rename_self", u.Username, "now "+name)
	if isHX(r) {
		w.Header().Set("HX-Refresh", "true") // the name is shown all over the page
		return
	}
	back(w, r, "/settings")
}

// saveInstructions stores the user's personal custom instructions.
func (s *Server) saveInstructions(w http.ResponseWriter, r *http.Request) {
	text := strings.TrimSpace(r.FormValue("instructions"))
	if rs := []rune(text); len(rs) > 4000 {
		text = string(rs[:4000])
	}
	if err := s.st.SetInstructions(currentUser(r).ID, text); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.flash(w, r, "Instructions saved.")
}

// saveDisplay switches what the user sees of the AI's work in a chat: the
// "Thinking" block, or every tool call as its own row (Settings → Appearance).
// It changes the page only, never what is sent to the model.
func (s *Server) saveDisplay(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	thinking, tools := u.ShowThinking, u.ShowTools
	on := r.FormValue("on") == "1"
	switch r.FormValue("key") {
	case "thinking":
		thinking = on
	case "tools":
		tools = on
	default:
		s.errorPage(w, r, http.StatusBadRequest, "Unknown setting.")
		return
	}
	if err := s.st.SetDisplay(u.ID, thinking, tools); err != nil {
		s.serverError(w, r, err)
		return
	}
	if !isHX(r) {
		back(w, r, "/settings#appearance")
		return
	}
	s.renderPartial(w, r, "settings", "display-prefs", map[string]any{"ShowThinking": thinking, "ShowTools": tools})
}

// saveTimezone stores the user's time zone. app.js calls it quietly when the
// device's zone differs from the saved one; Settings → Account posts the form.
func (s *Server) saveTimezone(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	auto := r.FormValue("auto") == "1"
	tz := strings.TrimSpace(r.FormValue("tz"))
	if isFetch(r) && !u.TimezoneAuto { // the user fixed it by hand: the device no longer decides
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.st.SetTimezone(u.ID, tz, auto); err != nil {
		if errors.Is(err, store.ErrBadTimezone) {
			if isFetch(r) {
				http.Error(w, "unknown time zone", http.StatusBadRequest)
				return
			}
			s.flash(w, r, "That is not a time zone we know. Pick one from the list, like Europe/Lisbon.")
			return
		}
		s.serverError(w, r, err)
		return
	}
	if isFetch(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if tz == "" {
		s.flash(w, r, "Saved. Open any page and your device will fill it in.")
		return
	}
	loc, _ := time.LoadLocation(tz)
	s.flash(w, r, "Saved. It is now "+time.Now().In(loc).Format("15:04 on Monday")+" there.")
}

// flash shows a short message: into #flash for htmx, or as a redirect otherwise.
func (s *Server) flash(w http.ResponseWriter, r *http.Request, msg string) {
	if isHX(r) {
		w.Header().Set("HX-Retarget", "#flash")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.Write([]byte(templateEscape(msg)))
		return
	}
	back(w, r, "/")
}

// sessionFromRequest returns the session cookie, accepting the cookie name
// used before the app was renamed so existing logins keep working.
func sessionFromRequest(r *http.Request) (*http.Cookie, error) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c, nil
	}
	return r.Cookie(oldCookie)
}
