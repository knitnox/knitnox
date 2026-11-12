package secret

import (
	"path/filepath"
	"testing"
)

func TestSealAndOpen(t *testing.T) {
	box, err := New("a passphrase")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("sk-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := box.Open(sealed); err != nil || got != "sk-secret-value" {
		t.Fatalf("round trip: %q, %v", got, err)
	}
	again, _ := box.Seal("sk-secret-value")
	if string(again) == string(sealed) {
		t.Fatal("the same text sealed twice gave the same bytes")
	}
	// Empty stays empty in both directions.
	if b, err := box.Seal(""); b != nil || err != nil {
		t.Fatalf("empty seal: %v %v", b, err)
	}
	if s, err := box.Open(nil); s != "" || err != nil {
		t.Fatalf("empty open: %q %v", s, err)
	}
	// Another key, or changed bytes, must fail and never return wrong text.
	other, _ := New("another passphrase")
	if got, err := other.Open(sealed); err == nil {
		t.Fatalf("the wrong key opened it: %q", got)
	}
	sealed[len(sealed)-1] ^= 1
	if got, err := box.Open(sealed); err == nil {
		t.Fatalf("changed bytes were accepted: %q", got)
	}
	if _, err := box.Open([]byte{1, 2, 3}); err == nil {
		t.Fatal("three bytes were accepted")
	}
	if _, err := New(""); err == nil {
		t.Fatal("an empty passphrase was accepted")
	}
}

func TestKeyFileIsMadeOnceAndReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")
	first, err := LoadOrCreateKeyFile(path)
	if err != nil || len(first) != 64 {
		t.Fatalf("first: %q %v", first, err)
	}
	second, err := LoadOrCreateKeyFile(path)
	if err != nil || second != first {
		t.Fatalf("the key changed between two starts: %q vs %q (%v)", first, second, err)
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{"": "(none)", "short": "••••", "sk-abcdefghij1234": "sk-…1234"} {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}
