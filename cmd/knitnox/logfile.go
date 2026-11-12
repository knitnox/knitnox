package main

import (
	"os"
	"sync"
)

// logFile appends log lines to a file and keeps it from growing for ever:
// when it passes maxBytes it is renamed to NAME.1 (replacing the previous
// NAME.1) and a new file is started. So at most two files of that size exist.
//
// It is for places where nothing collects what a program prints: a console
// window on Windows that is closed, or a double-clicked knitnox.exe.
type logFile struct {
	mu       sync.Mutex
	path     string
	f        *os.File
	size     int64
	maxBytes int64
}

func openLogFile(path string, maxBytes int64) (*logFile, error) {
	l := &logFile{path: path, maxBytes: maxBytes}
	if err := l.open(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *logFile) open() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	l.f, l.size = f, info.Size()
	return nil
}

func (l *logFile) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil { // an earlier turn-over failed (disk full, file locked): try again now
		if err := l.open(); err != nil {
			return 0, err
		}
	}
	if l.size+int64(len(p)) > l.maxBytes && l.size > 0 {
		l.f.Close()
		l.f = nil
		os.Remove(l.path + ".1")
		os.Rename(l.path, l.path+".1") // if this fails the file simply keeps growing; logging goes on
		if err := l.open(); err != nil {
			return 0, err
		}
	}
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}
