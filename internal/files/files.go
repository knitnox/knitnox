// Package files classifies uploads and extracts text from documents so the
// model can read them. Images are passed through; PDFs and text files are
// turned into plain text (capped).
package files

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"

	"knitnox/internal/store"
)

const (
	// MaxText is the number of characters of extracted text kept per file.
	MaxText = 120_000
)

// Known text-like extensions that http.DetectContentType reports as text/plain
// or application/octet-stream. Anything else that sniffs as text is accepted too.
var textExt = map[string]string{
	".md": "text/markdown", ".markdown": "text/markdown", ".csv": "text/csv", ".json": "application/json",
	".txt": "text/plain", ".log": "text/plain", ".xml": "text/xml", ".html": "text/html", ".htm": "text/html",
	".yaml": "text/plain", ".yml": "text/plain", ".toml": "text/plain", ".ini": "text/plain", ".cfg": "text/plain",
	".go": "text/plain", ".py": "text/plain", ".js": "text/plain", ".ts": "text/plain", ".tsx": "text/plain", ".jsx": "text/plain",
	".java": "text/plain", ".c": "text/plain", ".h": "text/plain", ".cpp": "text/plain", ".cs": "text/plain", ".rs": "text/plain",
	".rb": "text/plain", ".php": "text/plain", ".sh": "text/plain", ".sql": "text/plain", ".css": "text/plain", ".swift": "text/plain",
	".kt": "text/plain", ".scala": "text/plain", ".r": "text/plain", ".tex": "text/plain", ".env": "text/plain",
}

// Classified is the result of inspecting an upload.
type Classified struct {
	Mime string // normalised MIME type stored with the file
	Kind string // store.KindImage | store.KindFile
	Text string // extracted text for files
}

// Classify sniffs the bytes (never trusting the client's type), decides
// whether the upload is an image or a document, and extracts document text.
func Classify(data []byte, name string) (Classified, error) {
	sniffed := http.DetectContentType(data)
	base, _, _ := strings.Cut(sniffed, ";")
	base = strings.TrimSpace(base)
	if _, ok := store.AllowedImageTypes[base]; ok {
		return Classified{Mime: base, Kind: store.KindImage}, nil
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".tif", ".tiff", ".heic", ".svg":
		return Classified{}, errors.New("not a valid PNG, JPEG, WebP or GIF image")
	}
	switch {
	case base == "application/pdf":
		text, err := ExtractPDF(data)
		if err != nil {
			return Classified{}, fmt.Errorf("could not read the PDF (%v)", err)
		}
		if strings.TrimSpace(text) == "" {
			return Classified{}, errors.New("the PDF contains no selectable text (scanned pages are not supported)")
		}
		return Classified{Mime: base, Kind: store.KindFile, Text: cap(text)}, nil
	case strings.HasPrefix(base, "text/") || base == "application/json" || base == "application/xml" ||
		(base == "application/octet-stream" && textExt[ext] != "" && looksText(data)):
		mime := textExt[ext]
		if mime == "" {
			mime = base
		}
		if _, ok := store.AllowedFileTypes[mime]; !ok {
			mime = "text/plain"
		}
		if !looksText(data) {
			return Classified{}, errors.New("the file is not readable text")
		}
		return Classified{Mime: mime, Kind: store.KindFile, Text: cap(string(data))}, nil
	}
	return Classified{}, fmt.Errorf("unsupported file type (%s)", base)
}

// looksText rejects binary data that merely has a text-like extension.
func looksText(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	sample := b
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return false
	}
	return utf8.Valid(sample) || utf8.Valid(b)
}

func cap(s string) string {
	s = strings.ToValidUTF8(s, "")
	if r := []rune(s); len(r) > MaxText {
		return string(r[:MaxText]) + "\n…[truncated]"
	}
	return s
}

// ExtractPDF returns the text of every page, separated by form feeds.
func ExtractPDF(data []byte) (text string, err error) {
	defer func() { // the parser can panic on malformed files
		if r := recover(); r != nil {
			err = fmt.Errorf("malformed PDF: %v", r)
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		t, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		if i > 1 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "[page %d]\n%s", i, strings.TrimSpace(t))
		if b.Len() > MaxText*4 { // bytes; cap() trims to runes later
			break
		}
	}
	return b.String(), nil
}
