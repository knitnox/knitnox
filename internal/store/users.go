package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	_ "time/tzdata" // time zone names work on every OS, also where none are installed (Windows)
)

type User struct {
	ID           int64
	Username     string
	Role         string
	Disabled     bool
	CreatedAt    string
	Instructions string // personal custom instructions (solo chats)
	Timezone     string // IANA name, e.g. "Asia/Dhaka"; "" = not known yet
	TimezoneAuto bool   // the browser keeps Timezone up to date
}

// Location is the user's time zone, or UTC while it is not known.
func (u *User) Location() *time.Location {
	if u != nil && u.Timezone != "" {
		if loc, err := time.LoadLocation(u.Timezone); err == nil {
			return loc
		}
	}
	return time.UTC
}

// MustLocation is the time zone of a user by id (UTC if unknown).
func (s *Store) MustLocation(userID int64) *time.Location {
	u, err := s.UserByID(userID)
	if err != nil {
		return time.UTC
	}
	return u.Location()
}

// ErrBadTimezone is returned for a name that is not a time zone.
var ErrBadTimezone = errors.New("unknown time zone")

// SetTimezone saves a user's time zone. tz must be an IANA name ("Europe/Lisbon")
// or empty; auto says whether the browser may keep changing it.
func (s *Store) SetTimezone(userID int64, tz string, auto bool) error {
	tz = strings.TrimSpace(tz)
	if tz != "" {
		if _, err := time.LoadLocation(tz); err != nil || len(tz) > 64 || strings.EqualFold(tz, "Local") {
			return ErrBadTimezone
		}
	}
	_, err := s.DB.Exec(`UPDATE users SET timezone = ?, timezone_auto = ? WHERE id = ?`, tz, b2i(auto), userID)
	return err
}

func (u *User) IsAdmin() bool { return u.Role == "admin" }

const userCols = `id, username, role, disabled, created_at, instructions, timezone, timezone_auto`

func scanUser(sc interface{ Scan(...any) error }) (*User, error) {
	u := &User{}
	err := sc.Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.Instructions, &u.Timezone, &u.TimezoneAuto)
	return u, err
}

// SetInstructions saves a user's custom instructions.
func (s *Store) SetInstructions(userID int64, text string) error {
	_, err := s.DB.Exec(`UPDATE users SET instructions = ? WHERE id = ?`, text, userID)
	return err
}

// DeleteUserSessions logs the user out everywhere.
func (s *Store) DeleteUserSessions(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// SessionCount returns how many active sessions a user has.
func (s *Store) SessionCount(userID int64) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id = ? AND expires_at > ?`, userID, Now()).Scan(&n)
	return n
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, passwordHash, role string) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO users(username, password_hash, role) VALUES (?, ?, ?)`,
		username, passwordHash, role)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UserByID(id int64) (*User, error) {
	u, err := scanUser(s.DB.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id))
	return u, notFound(err)
}

// UserAuth returns the user and password hash for login.
func (s *Store) UserAuth(username string) (*User, string, error) {
	u := &User{}
	var hash string
	err := s.DB.QueryRow(`SELECT `+userCols+`, password_hash FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.Instructions, &u.Timezone, &u.TimezoneAuto, &hash)
	return u, hash, notFound(err)
}

func (s *Store) PasswordHash(userID int64) (string, error) {
	var h string
	err := s.DB.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&h)
	return h, notFound(err)
}

func (s *Store) SetPassword(userID int64, hash string) error {
	_, err := s.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

func (s *Store) SetUserRole(userID int64, role string) error {
	_, err := s.DB.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, userID)
	return err
}

func (s *Store) SetUserDisabled(userID int64, disabled bool) error {
	_, err := s.DB.Exec(`UPDATE users SET disabled = ? WHERE id = ?`, b2i(disabled), userID)
	if err == nil && disabled {
		_, err = s.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	}
	return err
}

func (s *Store) DeleteUser(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM users WHERE id = ?`, userID)
	return err
}

// UserRow is a user plus quota information for the admin panel.
type UserRow struct {
	User
	Period     string
	TokenLimit int64
	Used       int64
}

func (s *Store) ListUsersWithQuota() ([]UserRow, error) {
	rows, err := s.DB.Query(`SELECT u.id, u.username, u.role, u.disabled, u.created_at,
		COALESCE(q.period, 'monthly'), COALESCE(q.token_limit, 0)
		FROM users u LEFT JOIN quotas q ON q.user_id = u.id ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserRow
	for rows.Next() {
		var r UserRow
		if err := rows.Scan(&r.ID, &r.Username, &r.Role, &r.Disabled, &r.CreatedAt, &r.Period, &r.TokenLimit); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Used, err = s.UsedTokens(out[i].ID, out[i].Period)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ---- sessions ----

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// RandomToken returns 64 hex chars from crypto/rand.
func RandomToken() string { return randomToken() }

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// CreateSession returns the raw cookie token. Only its hash is stored.
func (s *Store) CreateSession(userID int64, ttl time.Duration) (string, error) {
	tok := randomToken()
	_, err := s.DB.Exec(`INSERT INTO sessions(token_hash, user_id, csrf, expires_at) VALUES (?, ?, ?, ?)`,
		hashToken(tok), userID, randomToken(), FormatTime(time.Now().Add(ttl)))
	return tok, err
}

// SessionUser resolves a cookie token to an active user and its CSRF token.
func (s *Store) SessionUser(token string) (*User, string, error) {
	u := &User{}
	var csrf string
	err := s.DB.QueryRow(`SELECT u.id, u.username, u.role, u.disabled, u.created_at, u.instructions, u.timezone, u.timezone_auto, s.csrf
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ? AND u.disabled = 0`, hashToken(token), Now()).
		Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.Instructions, &u.Timezone, &u.TimezoneAuto, &csrf)
	return u, csrf, notFound(err)
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

func (s *Store) PurgeExpiredSessions() error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, Now())
	return err
}
