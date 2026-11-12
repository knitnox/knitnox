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
	maxRequestBytes = 48 << 20 // up to 4 images × 10 MB + text
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
				http.Error(w, "invalid CSRF token — reload the page", http.StatusForbidden)
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
	keys := []string{"ip:" + clientIP(r), "user:" + strings.ToLower(username)}
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
	if err == nil && bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.FormValue("password"))) == nil && !u.Disabled {
		s.logins.reset(keys...)
		if err := s.setSession(w, u.ID); err != nil {
			s.serverError(w, r, err)
			return
		}
		http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
		return
	}
	s.logins.fail(keys...)
	if err == nil && u.Disabled {
		fail(http.StatusUnauthorized, "This account is disabled.")
		return
	}
	fail(http.StatusUnauthorized, "Wrong username or password.")
}

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
	hash, err := hashPassword(password)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	role := "user"
	if n == 0 {
		role = "admin" // first account administers the instance
	}
	id, err := s.st.CreateUser(username, hash, role)
	if err != nil {
		fail("That username is taken.")
		return
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
	nh, err := hashPassword(r.FormValue("new"))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.st.SetPassword(u.ID, nh)
	s.flash(w, r, "Password changed.")
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
