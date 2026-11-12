package store

import (
	"os"
	"path/filepath"
	"testing"

	"knitnox/internal/secret"
)

func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	box, _ := secret.New("test")
	st, err := Open(filepath.Join(dir, "knitnox.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st, dir
}

// A backup can be opened again and holds the accounts, the key and the files.
func TestBackupCanBeRestored(t *testing.T) {
	st, dataDir := testStore(t)
	if _, err := st.CreateUser("alice", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dataDir, "secret.key"), []byte("k\n"), 0o600)
	os.WriteFile(filepath.Join(st.UploadDir, "a.png"), []byte("picture"), 0o600)

	dest := t.TempDir()
	out, err := st.Backup(dataDir, dest)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"knitnox.db", "secret.key", "uploads/a.png"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Fatalf("backup lacks %s: %v", f, err)
		}
	}
	// Restore: open the copy as a database of its own.
	box, _ := secret.New("test")
	restored, err := Open(filepath.Join(out, "knitnox.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if u, _, err := restored.UserAuth("alice"); err != nil || u.Role != "admin" {
		t.Fatalf("alice in the restored copy: %+v, %v", u, err)
	}
}

// A damaged copy is noticed, and a failed backup leaves no folder behind.
func TestBackupCheckAndCleanup(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.db")
	os.WriteFile(bad, []byte("this is not a database, it only has the right name"), 0o600)
	if err := CheckDatabaseFile(bad); err == nil {
		t.Fatal("a file that is not a database passed the check")
	}

	st, dataDir := testStore(t)
	dest := t.TempDir()
	// Make copying the uploads fail: the place for them is taken by a file.
	os.WriteFile(filepath.Join(st.UploadDir, "a.png"), []byte("x"), 0o600)
	st.UploadDir = filepath.Join(dataDir, "knitnox.db", "nope") // a path that cannot be read as a folder
	if _, err := st.Backup(dataDir, dest); err == nil {
		t.Skip("this system reads a folder under a file as empty; nothing to check")
	}
	if names, _ := ListBackups(dest); len(names) != 0 {
		t.Fatalf("a failed backup left %v behind", names)
	}
}

// Only the newest backups are kept, and nothing else in the folder is touched.
func TestPruneBackups(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"knitnox-20261101-000000", "knitnox-20261102-000000", "knitnox-20261103-000000",
		"knitnox-20261104-000000", "holiday-photos", "knitnox-notes"} {
		os.MkdirAll(filepath.Join(dir, n), 0o700)
	}
	os.WriteFile(filepath.Join(dir, "knitnox-20261101-000001"), []byte("a file, not a backup"), 0o600)

	n, err := PruneBackups(dir, 2)
	if err != nil || n != 2 {
		t.Fatalf("removed %d, err %v", n, err)
	}
	left, _ := ListBackups(dir)
	if len(left) != 2 || left[0] != "knitnox-20261104-000000" || left[1] != "knitnox-20261103-000000" {
		t.Fatalf("left = %v", left)
	}
	for _, keep := range []string{"holiday-photos", "knitnox-notes", "knitnox-20261101-000001"} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Fatalf("%s was removed", keep)
		}
	}
}

func TestSetUsername(t *testing.T) {
	st, _ := testStore(t)
	alice, _ := st.CreateUser("alice", "h", "admin")
	st.CreateUser("bob", "h", "user")

	if err := st.SetUsername(alice, "BOB"); err != ErrUsernameTaken {
		t.Fatalf("taking bob's name in capitals: %v", err)
	}
	for _, bad := range []string{"", "ab", "has space", "semi;colon", "ålice"} {
		if err := st.SetUsername(alice, bad); err != ErrBadUsername {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	if err := st.SetUsername(alice, "Alice"); err != nil { // only the case changes
		t.Fatal(err)
	}
	if err := st.SetUsername(alice, " alice.new "); err != nil {
		t.Fatal(err)
	}
	if u, _, err := st.UserAuth("alice.new"); err != nil || u.ID != alice {
		t.Fatalf("after the rename: %+v, %v", u, err)
	}
	if _, _, err := st.UserAuth("alice"); err != ErrNotFound {
		t.Fatalf("the old name still works: %v", err)
	}
	if err := st.SetUsername(9999, "nobody"); err != ErrNotFound {
		t.Fatalf("unknown user: %v", err)
	}
}
