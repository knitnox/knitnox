package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Stand-ins for the picture, voice and video endpoints, so the Feed's media
// can be tried without real models:
//
//	POST /v1/images/generations  a PNG whose colours depend on the prompt
//	POST /v1/audio/speech        a short WAV: one soft tone per word
//	POST /v1/videos, GET /v1/videos/{id}, GET /v1/videos/{id}/content
//	                             a job that is "completed" after three seconds;
//	                             its file is a placeholder that does not play
//
// A prompt that contains the word "fail" makes each of them answer with an error.

func registerMedia() {
	http.HandleFunc("POST /v1/images/generations", mockImage)
	http.HandleFunc("POST /v1/audio/speech", mockSpeech)
	http.HandleFunc("POST /v1/videos", mockVideoStart)
	http.HandleFunc("GET /v1/videos/{id}", mockVideoStatus)
	http.HandleFunc("GET /v1/videos/{id}/content", mockVideoFile)
}

func mediaPrompt(w http.ResponseWriter, r *http.Request, field string) (string, bool) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return "", false
	}
	prompt, _ := req[field].(string)
	if strings.Contains(strings.ToLower(prompt), "fail") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "the mock was asked to fail"}})
		return "", false
	}
	return prompt, true
}

// mockImage paints soft diagonal bands and a sun; the prompt picks the colours.
func mockImage(w http.ResponseWriter, r *http.Request) {
	prompt, ok := mediaPrompt(w, r, "prompt")
	if !ok {
		return
	}
	h := fnv.New32a()
	h.Write([]byte(prompt))
	seed := h.Sum32()
	hue := float64(seed % 360)
	const width, height = 768, 512
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			t := float64(x+y) / float64(width+height)
			band := math.Floor(t*7) / 7
			c := hsl(hue+band*70, 0.55, 0.78-band*0.28)
			dx, dy := float64(x)-float64(width)*0.72, float64(y)-float64(height)*0.3
			if math.Hypot(dx, dy) < 70 {
				c = hsl(hue+180, 0.75, 0.86)
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString(buf.Bytes())}}})
}

func hsl(h, s, l float64) color.RGBA {
	h = math.Mod(h, 360) / 360
	f := func(n float64) uint8 {
		k := math.Mod(n+h*12, 12)
		a := s * math.Min(l, 1-l)
		return uint8(255 * (l - a*math.Max(-1, math.Min(math.Min(k-3, 9-k), 1))))
	}
	return color.RGBA{f(0), f(8), f(4), 255}
}

// mockSpeech answers with a WAV file: one short tone per word of the text.
func mockSpeech(w http.ResponseWriter, r *http.Request) {
	text, ok := mediaPrompt(w, r, "input")
	if !ok {
		return
	}
	const rate = 16000
	words := min(max(len(strings.Fields(text)), 1), 40)
	var samples []int16
	for i := 0; i < words; i++ {
		freq := 330 + float64((i*7)%5)*55
		n := rate / 6
		for j := 0; j < n; j++ {
			fade := math.Sin(math.Pi * float64(j) / float64(n))
			samples = append(samples, int16(6000*fade*math.Sin(2*math.Pi*freq*float64(j)/rate)))
		}
		samples = append(samples, make([]int16, rate/20)...)
	}
	var buf bytes.Buffer
	size := uint32(len(samples) * 2)
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, 36+size)
	buf.WriteString("WAVEfmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&buf, binary.LittleEndian, uint32(rate))
	binary.Write(&buf, binary.LittleEndian, uint32(rate*2))
	binary.Write(&buf, binary.LittleEndian, uint16(2))
	binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, size)
	binary.Write(&buf, binary.LittleEndian, samples)
	w.Header().Set("Content-Type", "audio/wav")
	w.Write(buf.Bytes())
}

var (
	videoMu   sync.Mutex
	videoJobs = map[string]time.Time{} // id -> when it is ready
)

func mockVideoStart(w http.ResponseWriter, r *http.Request) {
	if _, ok := mediaPrompt(w, r, "prompt"); !ok {
		return
	}
	id := fmt.Sprintf("video_%d", time.Now().UnixNano())
	videoMu.Lock()
	videoJobs[id] = time.Now().Add(3 * time.Second)
	videoMu.Unlock()
	json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "queued"})
}

func mockVideoStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	videoMu.Lock()
	ready, ok := videoJobs[id]
	videoMu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	status := "in_progress"
	if time.Now().After(ready) {
		status = "completed"
	}
	json.NewEncoder(w).Encode(map[string]any{"id": id, "status": status})
}

// mockVideoFile hands out a placeholder: it has the start of an MP4 file so it
// is recognised as a video, but there is no film in it and players show nothing.
func mockVideoFile(w http.ResponseWriter, r *http.Request) {
	videoMu.Lock()
	_, ok := videoJobs[r.PathValue("id")]
	videoMu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Write([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom\x00\x00\x00\x08free"))
}

// mockDiagram is the picture the mock model "draws" for a post, following the
// advice Knitnox gives models: viewBox 640x360, light background, large text.
func mockDiagram(title string) string {
	title = strings.NewReplacer("&", "and", "<", "", ">", "").Replace(title)
	if r := []rune(title); len(r) > 44 {
		title = string(r[:43]) + "…"
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 360" font-family="Arial, sans-serif">` +
		`<rect width="640" height="360" fill="#ffffff"/>` +
		`<text x="32" y="50" font-size="22" font-weight="bold" fill="#1f2937">` + title + `</text>` +
		`<g font-size="17" fill="#1f2937" text-anchor="middle">` +
		`<rect x="32" y="130" width="160" height="90" rx="14" fill="#e0e7ff" stroke="#6366f1" stroke-width="2"/><text x="112" y="181">Collect</text>` +
		`<rect x="240" y="130" width="160" height="90" rx="14" fill="#e0e7ff" stroke="#6366f1" stroke-width="2"/><text x="320" y="181">Check</text>` +
		`<rect x="448" y="130" width="160" height="90" rx="14" fill="#dcfce7" stroke="#16a34a" stroke-width="2"/><text x="528" y="181">Report</text>` +
		`</g>` +
		`<g stroke="#6b7280" stroke-width="3" fill="none"><path d="M196 175h36"/><path d="M224 167l10 8-10 8"/><path d="M404 175h36"/><path d="M432 167l10 8-10 8"/></g>` +
		`<text x="32" y="300" font-size="16" fill="#6b7280">Drawn by the mock model as an example diagram.</text>` +
		`</svg>`
}
