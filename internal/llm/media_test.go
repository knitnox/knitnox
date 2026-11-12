package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mediaBody(r *http.Request) map[string]any {
	var m map[string]any
	json.NewDecoder(r.Body).Decode(&m)
	return m
}

// OpenRouter makes pictures at /images, not /images/generations.
func TestImageAtShortAddress(t *testing.T) {
	var pic bytes.Buffer
	png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /images", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{
			"b64_json": base64.StdEncoding.EncodeToString(pic.Bytes()), "media_type": "image/png"}}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	m, err := GenerateImage(context.Background(), MediaRequest{BaseURL: srv.URL, Model: "img", Prompt: "a tree"})
	if err != nil || m.Mime != "image/png" {
		t.Fatalf("picture = %+v, %v", m, err)
	}
}

// A provider without any picture endpoint still gives its own 404.
func TestImageNoEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()
	_, err := GenerateImage(context.Background(), MediaRequest{BaseURL: srv.URL, Model: "img", Prompt: "a tree"})
	if statusOf(err) != http.StatusNotFound {
		t.Fatalf("err = %v", err)
	}
}

// Raw PCM (OpenRouter's and Gemini's usual answer) is wrapped so it can be played.
func TestSpeakWrapsRawSound(t *testing.T) {
	pcm := bytes.Repeat([]byte{1, 2}, 100)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /audio/speech", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/L16;rate=16000")
		w.Write(pcm)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	m, err := Speak(context.Background(), MediaRequest{BaseURL: srv.URL, Model: "tts", Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Mime != "audio/wav" || len(m.Data) != 44+len(pcm) || string(m.Data[:4]) != "RIFF" || string(m.Data[8:16]) != "WAVEfmt " ||
		binary.LittleEndian.Uint32(m.Data[24:28]) != 16000 || binary.LittleEndian.Uint32(m.Data[40:44]) != uint32(len(pcm)) ||
		!bytes.Equal(m.Data[44:], pcm) {
		t.Fatalf("wav = %q (%s)", m.Data[:44], m.Mime)
	}
	if pcmRate("audio/pcm") != 24000 || pcmRate("audio/mpeg") != 0 || pcmRate("audio/wav") != 0 {
		t.Error("pcmRate is wrong")
	}
}

// "alloy" is only a first try: a provider that does not know it is asked
// again without a voice. A voice the user chose is never dropped.
func TestSpeakVoiceFallback(t *testing.T) {
	var voices []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /audio/speech", func(w http.ResponseWriter, r *http.Request) {
		in := mediaBody(r)
		v, has := in["voice"]
		voices = append(voices, fmt.Sprint(v))
		if has {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"unknown voice"}}`)
			return
		}
		if _, asked := in["response_format"]; asked {
			t.Error("a format was asked for from a provider that is not OpenRouter")
		}
		w.Header().Set("Content-Type", "audio/wav")
		fmt.Fprint(w, "RIFF....WAVE")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	m, err := Speak(context.Background(), MediaRequest{BaseURL: srv.URL, Model: "tts", Prompt: "hello"})
	if err != nil || m.Mime != "audio/wav" || strings.Join(voices, ",") != "alloy,<nil>" {
		t.Fatalf("speech = %+v, %v; voices asked: %v", m, err, voices)
	}
	voices = nil
	_, err = Speak(context.Background(), MediaRequest{BaseURL: srv.URL, Model: "tts", Prompt: "hello", Voice: "nova"})
	if statusOf(err) != http.StatusBadRequest || strings.Join(voices, ",") != "nova" {
		t.Fatalf("a chosen voice: err = %v, asked: %v", err, voices)
	}
}

// DeepInfra: speech is beside the /openai path, not under it.
func TestSpeakBesideOpenAIPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/audio/speech", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		fmt.Fprint(w, "ID3 sound")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	m, err := Speak(context.Background(), MediaRequest{BaseURL: srv.URL + "/v1/openai", Model: "tts", Prompt: "hello"})
	if err != nil || m.Mime != "audio/mpeg" {
		t.Fatalf("speech = %+v, %v", m, err)
	}
}

// A video job that expired or failed ends at once, with the provider's
// reason whether it is plain text or an object.
func TestVideoFailureWords(t *testing.T) {
	for status, body := range map[string]string{
		"expired": `{"id":"v1","status":"expired","error":"the job ran out of time"}`,
		"failed":  `{"id":"v1","status":"failed","error":{"message":"the job ran out of time"}}`,
	} {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /videos", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"id":"v1","status":"pending","polling_url":"/videos/v1"}`)
		})
		mux.HandleFunc("GET /videos/v1", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
		srv := httptest.NewServer(mux)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := GenerateVideo(ctx, MediaRequest{BaseURL: srv.URL, Model: "vid", Prompt: "a pan"}, time.Millisecond)
		cancel()
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), "the job ran out of time") {
			t.Errorf("%s: err = %v", status, err)
		}
	}
}
