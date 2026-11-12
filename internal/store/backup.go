package store

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

var backupMu sync.Mutex // one backup at a time

// Backup writes a consistent snapshot of everything Knitnox needs into
// destDir/knitnox-<timestamp>/: the database (VACUUM INTO), the secret key
// file next to the database (if any), the uploads and the feed media. The
// database copy is then opened and checked, so a backup that is reported as
// written can also be read. A backup that fails half-way is removed again.
// It returns the created folder.
func (s *Store) Backup(dataDir, destDir string) (out string, err error) {
	backupMu.Lock()
	defer backupMu.Unlock()
	out = filepath.Join(destDir, "knitnox-"+time.Now().UTC().Format("20060102-150405"))
	if _, statErr := os.Stat(out); statErr == nil {
		return "", fmt.Errorf("%s already exists; wait a second and try again", out)
	}
	if err := os.MkdirAll(out, 0o700); err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(out) // never leave a half backup that looks like a whole one
		}
	}()
	dbCopy := filepath.Join(out, "knitnox.db")
	if _, err := s.DB.Exec(`VACUUM INTO ?`, filepath.ToSlash(dbCopy)); err != nil {
		return "", fmt.Errorf("vacuum into: %w", err)
	}
	if err := CheckDatabaseFile(dbCopy); err != nil {
		return "", fmt.Errorf("the copy of the database did not pass its check: %w", err)
	}
	if key := filepath.Join(dataDir, "secret.key"); fileExists(key) {
		if err := copyFile(key, filepath.Join(out, "secret.key")); err != nil {
			return "", err
		}
	}
	if err := copyDir(s.UploadDir, filepath.Join(out, "uploads")); err != nil {
		return "", err
	}
	if info, err := os.Stat(s.FeedMediaDir()); err == nil && info.IsDir() { // pictures, voice and video of feed posts
		if err := copyDir(s.FeedMediaDir(), filepath.Join(out, "feedmedia")); err != nil {
			return "", err
		}
	}
	return out, nil
}

// CheckDatabaseFile opens a database file read-only and lets SQLite verify
// it from end to end (PRAGMA integrity_check).
func CheckDatabaseFile(path string) error {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity check said: %s", result)
	}
	var users int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		return fmt.Errorf("cannot read the users table: %w", err)
	}
	return nil
}

var backupName = regexp.MustCompile(`^knitnox-\d{8}-\d{6}$`)

// ListBackups returns the backup folders in dir, newest first.
func ListBackups(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && backupName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names))) // the name is the time, so this is newest first
	return names, nil
}

// PruneBackups deletes all but the newest keep backups in dir and returns how
// many it removed. Only folders named like a Knitnox backup are touched.
func PruneBackups(dir string, keep int) (int, error) {
	if keep < 1 {
		keep = 1
	}
	names, err := ListBackups(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for i, name := range names {
		if i < keep {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	outF, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(outF, in); err != nil {
		outF.Close()
		return err
	}
	if err := outF.Sync(); err != nil { // on the disk, not only in the cache
		outF.Close()
		return err
	}
	return outF.Close()
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			if os.IsNotExist(err) { // deleted while we were copying
				continue
			}
			return err
		}
	}
	return nil
}
