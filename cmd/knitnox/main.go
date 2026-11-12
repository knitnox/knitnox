// Command knitnox runs the Knitnox server.
//
//	knitnox                         start the server (same as "knitnox serve")
//	knitnox create-admin USER PASS  create an admin account
//	knitnox reset-password USER PASS
//	knitnox list-users              show every account (for a forgotten username)
//	knitnox rename-user OLD NEW     change a username
//	knitnox backup DIR              snapshot database, secret key and uploads into DIR/knitnox-<time>/
//
// Configuration (environment variables):
//
//	KNITNOX_ADDR            listen address            (default ":8080")
//	KNITNOX_DATA            data directory            (default "./data")
//	KNITNOX_SECRET          encryption passphrase     (default: generated into DATA/secret.key)
//	KNITNOX_SECURE_COOKIES  "1" when served over HTTPS
//	KNITNOX_LOG_FILE        also write the log to this file (10 MB, one older file kept)
//	KNITNOX_BACKUP_DIR      automatic backups         (default DATA/backups; "off" disables them)
//	KNITNOX_LOG             "debug" | "info" | "warn"  (default "info")
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"knitnox/internal/engine"
	"knitnox/internal/hub"
	"knitnox/internal/secret"
	"knitnox/internal/store"
	"knitnox/internal/web"
)

// env reads KNITNOX_* and, for installs from before the rename, falls back
// to the matching PARLEY_* variable.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if old, ok := strings.CutPrefix(key, "KNITNOX_"); ok {
		if v := os.Getenv("PARLEY_" + old); v != "" {
			return v
		}
	}
	return def
}

// dbPath returns DATA/knitnox.db. A database from before the rename
// (parley.db, plus its -wal/-shm files) is renamed once, at startup.
func dbPath(dataDir string, log *slog.Logger) string {
	newPath := filepath.Join(dataDir, "knitnox.db")
	oldPath := filepath.Join(dataDir, "parley.db")
	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	if _, err := os.Stat(oldPath); err != nil {
		return newPath
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(oldPath + suffix); err != nil {
			continue
		}
		if err := os.Rename(oldPath+suffix, newPath+suffix); err != nil {
			if log != nil {
				log.Warn("could not rename parley.db to knitnox.db; keeping the old name", "err", err)
			}
			return oldPath
		}
	}
	if log != nil {
		log.Info("renamed database parley.db to knitnox.db")
	}
	return newPath
}

func main() {
	level := slog.LevelInfo
	switch env("KNITNOX_LOG", "info") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	}
	var out io.Writer = os.Stderr
	// Only the server writes the log file; the short commands print to the screen.
	if path := env("KNITNOX_LOG_FILE", ""); path != "" && (len(os.Args) < 2 || os.Args[1] == "serve") {
		lf, err := openLogFile(path, 10<<20)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot open the log file:", err)
			os.Exit(1)
		}
		out = io.MultiWriter(os.Stderr, lf)
	}
	log := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve(log)
	case "create-admin", "reset-password":
		if len(os.Args) != 4 {
			fmt.Fprintf(os.Stderr, "usage: knitnox %s USERNAME PASSWORD\n", cmd)
			os.Exit(2)
		}
		err = userCommand(cmd, os.Args[2], os.Args[3])
	case "list-users":
		err = listUsers()
	case "rename-user":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: knitnox rename-user OLD_USERNAME NEW_USERNAME")
			os.Exit(2)
		}
		err = renameUser(os.Args[2], os.Args[3])
	case "backup":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: knitnox backup DIR")
			os.Exit(2)
		}
		err = backup(os.Args[2])
	default:
		fmt.Fprintln(os.Stderr, "usage: knitnox [serve | list-users | create-admin USER PASS | reset-password USER PASS | rename-user OLD NEW | backup DIR]")
		os.Exit(2)
	}
	if err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func openStore() (*store.Store, error) {
	dataDir := env("KNITNOX_DATA", "./data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	pass := env("KNITNOX_SECRET", "")
	if pass == "" {
		var err error
		if pass, err = secret.LoadOrCreateKeyFile(filepath.Join(dataDir, "secret.key")); err != nil {
			return nil, fmt.Errorf("secret key: %w", err)
		}
	}
	box, err := secret.New(pass)
	if err != nil {
		return nil, err
	}
	return store.Open(dbPath(dataDir, slog.Default()), box)
}

func serve(log *slog.Logger) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	if env("KNITNOX_SECRET", "") == "" {
		log.Warn("KNITNOX_SECRET not set; using generated key file in the data directory. Back it up — losing it makes stored API keys unreadable.")
	}

	h := hub.New()
	eng := engine.New(st, h, log)
	if err := eng.Start(); err != nil {
		return err
	}
	dataDir := env("KNITNOX_DATA", "./data")
	backupDir := env("KNITNOX_BACKUP_DIR", filepath.Join(dataDir, "backups"))
	if backupDir == "off" {
		backupDir = ""
	} else {
		log.Info("automatic backups", "dir", backupDir, "hint", "keep this folder on another disk than the data, or copy it elsewhere")
	}
	srv, err := web.New(st, eng, h, log, web.Options{SecureCookies: env("KNITNOX_SECURE_COOKIES", "") == "1",
		DataDir: dataDir, BackupDir: backupDir})
	if err != nil {
		return err
	}

	// Cancelling baseCtx ends long-lived SSE requests during shutdown.
	baseCtx, cancelBase := context.WithCancel(context.Background())
	defer cancelBase()
	srv.StartJanitor(baseCtx)

	httpSrv := &http.Server{
		Addr:              env("KNITNOX_ADDR", ":8080"),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: SSE streams are long-lived.
		BaseContext: func(net.Listener) context.Context { return baseCtx },
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("knitnox listening", "addr", httpSrv.Addr)
		errCh <- httpSrv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case s := <-sig:
		log.Info("shutting down", "signal", s.String())
	}

	cancelBase()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
	eng.Shutdown(10 * time.Second)
	log.Info("bye")
	return nil
}

// backup writes a consistent snapshot while the server may keep running.
func backup(dir string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	out, err := st.Backup(env("KNITNOX_DATA", "./data"), dir)
	if err != nil {
		return err
	}
	fmt.Println("backup written to", out)
	return nil
}

func userCommand(cmd, username, password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if cmd == "create-admin" {
		if _, err := st.CreateUser(username, string(hash), "admin"); err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		fmt.Println("admin created:", username)
		return nil
	}
	u, _, err := st.UserAuth(username)
	if err != nil {
		return fmt.Errorf("user %q not found", username)
	}
	if err := st.SetPassword(u.ID, string(hash)); err != nil {
		return err
	}
	if err := st.SetUserDisabled(u.ID, false); err != nil {
		return err
	}
	if err := st.DeleteUserSessions(u.ID); err != nil { // whoever knew the old password is signed out
		return err
	}
	fmt.Println("password reset, account enabled and signed out everywhere:", u.Username)
	return nil
}

// listUsers prints every account, so a forgotten username can be looked up.
func listUsers() error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	users, err := st.ListUsers()
	if err != nil {
		return err
	}
	if len(users) == 0 {
		fmt.Println("no accounts yet; the first one to sign up becomes the admin")
		return nil
	}
	fmt.Printf("%-4s  %-32s  %-6s  %s\n", "ID", "USERNAME", "ROLE", "STATE")
	for _, u := range users {
		state := "active"
		if u.Disabled {
			state = "disabled"
		}
		fmt.Printf("%-4d  %-32s  %-6s  %s\n", u.ID, u.Username, u.Role, state)
	}
	return nil
}

// renameUser changes a username from the command line.
func renameUser(oldName, newName string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	u, _, err := st.UserAuth(oldName)
	if err != nil {
		return fmt.Errorf("user %q not found (see: knitnox list-users)", oldName)
	}
	if err := st.SetUsername(u.ID, newName); err != nil {
		return err
	}
	fmt.Printf("renamed %s to %s\n", u.Username, strings.TrimSpace(newName))
	return nil
}
