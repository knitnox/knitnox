package web

import (
	"fmt"
	"net/http"
	"time"

	"knitnox/internal/store"
)

// Automatic backups
//
// The hourly clean-up asks autoBackup whether a backup is due (Admin →
// Settings: every N hours, keep the newest M). A backup is the same folder
// "knitnox backup DIR" writes. The time and outcome of the last attempt are
// kept in the database and shown on the admin settings page, so a backup that
// silently stopped working is visible.

const (
	kvBackupAt     = "backup_last_at"     // when the last attempt ran
	kvBackupOK     = "backup_last_ok"     // "1" or "0"
	kvBackupDetail = "backup_last_detail" // the folder written, or why it failed
)

// runBackup writes one backup now, removes the ones beyond the kept number
// and records how it went.
func (s *Server) runBackup() (string, error) {
	if s.backupDir == "" {
		return "", fmt.Errorf("no backup folder is set")
	}
	out, err := s.st.Backup(s.dataDir, s.backupDir)
	s.st.SetValue(kvBackupAt, store.Now())
	if err != nil {
		s.st.SetValue(kvBackupOK, "0")
		s.st.SetValue(kvBackupDetail, err.Error())
		s.log.Error("backup failed", "dir", s.backupDir, "err", err)
		return "", err
	}
	s.st.SetValue(kvBackupOK, "1")
	s.st.SetValue(kvBackupDetail, out)
	s.log.Info("backup written", "dir", out)
	keep := store.DefaultSettings().BackupKeep
	if st, err := s.st.LoadSettings(); err == nil {
		keep = st.BackupKeep
	}
	if n, err := store.PruneBackups(s.backupDir, keep); err != nil {
		s.log.Warn("could not remove old backups", "err", err)
	} else if n > 0 {
		s.log.Info("removed old backups", "count", n, "kept", keep)
	}
	return out, nil
}

// autoBackup runs a backup when automatic backups are on and the last attempt
// is older than the chosen interval. A failed attempt is tried again an hour later.
func (s *Server) autoBackup() {
	if s.backupDir == "" {
		return
	}
	st, err := s.st.LoadSettings()
	if err != nil || st.BackupEveryHours <= 0 {
		return
	}
	wait := time.Duration(st.BackupEveryHours) * time.Hour
	if s.st.GetValue(kvBackupOK) == "0" {
		wait = time.Hour
	}
	if last, err := time.Parse("2006-01-02 15:04:05", s.st.GetValue(kvBackupAt)); err == nil && time.Since(last) < wait-5*time.Minute {
		return
	}
	s.runBackup()
}

// backupStatus is what the admin settings page shows.
type backupStatus struct {
	Dir     string
	At      string // "" = never
	OK      bool
	Detail  string
	Count   int
	Enabled bool
}

func (s *Server) backupStatus(st store.Settings) backupStatus {
	b := backupStatus{Dir: s.backupDir, At: s.st.GetValue(kvBackupAt), OK: s.st.GetValue(kvBackupOK) == "1",
		Detail: s.st.GetValue(kvBackupDetail), Enabled: s.backupDir != "" && st.BackupEveryHours > 0}
	if names, err := store.ListBackups(s.backupDir); err == nil {
		b.Count = len(names)
	}
	return b
}

// adminBackupNow is the "Back up now" button.
func (s *Server) adminBackupNow(w http.ResponseWriter, r *http.Request) {
	out, err := s.runBackup()
	if err != nil {
		s.audit(r, "backup.run", "", "failed: "+err.Error())
		s.flash(w, r, "The backup failed: "+err.Error())
		return
	}
	s.audit(r, "backup.run", out, "")
	s.flash(w, r, "Backup written to "+out)
}
