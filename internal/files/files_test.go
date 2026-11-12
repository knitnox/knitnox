package files

import (
	"strings"
	"testing"

	"knitnox/internal/store"
)

var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

// The kind of a file comes from its bytes, never from its name.
func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		kind string // "" = refused
		mime string
	}{
		{"photo.png", png, store.KindImage, "image/png"},
		{"notes.txt", png, store.KindImage, "image/png"}, // a picture whatever it is called
		{"fake.png", []byte("just some text"), "", ""},   // text called .png is not a picture
		{"readme.md", []byte("# Title\n\nHello."), store.KindFile, "text/markdown"},
		{"data.csv", []byte("a,b\n1,2\n"), store.KindFile, "text/csv"},
		{"main.go", []byte("package main\n"), store.KindFile, "text/plain"},
		{"page.html", []byte("<html><body><script>alert(1)</script></body></html>"), store.KindFile, "text/html"},
		{"program.exe", []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff"), "", ""},
		{"binary.txt", []byte("abc\x00\x01\x02def"), "", ""},
		{"empty.txt", nil, "", ""},
		{"broken.pdf", []byte("%PDF-1.4\nthis is not really a pdf"), "", ""},
	}
	for _, c := range cases {
		got, err := Classify(c.data, c.name)
		if c.kind == "" {
			if err == nil {
				t.Errorf("%s: accepted as %+v, want it refused", c.name, got)
			}
			continue
		}
		if err != nil || got.Kind != c.kind || got.Mime != c.mime {
			t.Errorf("%s: got %+v, %v; want %s %s", c.name, got, err, c.kind, c.mime)
		}
		if c.kind == store.KindFile && got.Text == "" {
			t.Errorf("%s: no text was taken from it", c.name)
		}
	}
}

func TestLongTextIsCut(t *testing.T) {
	got, err := Classify([]byte(strings.Repeat("word ", MaxText)), "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(got.Text)); n > MaxText+20 || !strings.HasSuffix(got.Text, "[truncated]") {
		t.Fatalf("kept %d characters, ends %q", n, got.Text[len(got.Text)-15:])
	}
}

// A damaged PDF gives an error; it must never bring the server down.
func TestBrokenPDFDoesNotPanic(t *testing.T) {
	for _, data := range []string{"%PDF-1.7\n1 0 obj\n<< /Type /Catalog /Pages 9 0 R >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF", "%PDF-", "%PDF-1.4\nxref\n0 1\ntrailer<</Root 5 0 R>>startxref\n9\n%%EOF"} {
		if text, err := ExtractPDF([]byte(data)); err == nil && strings.TrimSpace(text) != "" {
			t.Errorf("text %q came out of a broken PDF", text)
		}
	}
}
