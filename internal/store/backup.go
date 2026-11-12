package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Backup writes a consistent snapshot of everything Knitnox needs into
// destDir/knitnox-<timestamp>/: the database (VACUUM INTO), the secret key
// file next to the database (if any) and the uploads directory.
// It returns the created folder.
func (s *Store) Backup(dataDir, destDir string) (string, error) {
	out := filepath.Join(destDir, "knitnox-"+time.Now().UTC().Format("20060102-150405"))
	if err := os.MkdirAll(out, 0o700); err != nil {
		return "", err
	}
	dbCopy := filepath.Join(out, "knitnox.db")
	if _, err := s.DB.Exec(`VACUUM INTO ?`, filepath.ToSlash(dbCopy)); err != nil {
		return "", fmt.Errorf("vacuum into: %w", err)
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
			return err
		}
	}
	return nil
}
