package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Pictures, voice and video from OpenAI-compatible endpoints:
//
//	POST {base}/images/generations   -> a picture (base64 or a link to it)
//	POST {base}/audio/speech         -> a voice recording
//	POST {base}/videos, then GET {base}/videos/{id} until it is done,
//	     then GET {base}/videos/{id}/content -> a video
//
// Pictures and voice follow the format most providers share. Video has no
// shared format yet; this is the one OpenAI uses.

// MediaRequest asks a model for one piece of media.
type MediaRequest struct {
	BaseURL string
	APIKey  string
	Model   string
	Prompt  string // what to draw or film, or the text to speak
	Voice   string // voice name for speech; "" = "alloy"
	UserURL bool   // a user's own provider: guarded addresses are refused
}

// Media is a finished file.
type Media struct {
	Data []byte
	Mime string // e.g. image/png, audio/mpeg, video/mp4
}

const mediaMaxBytes = 100 << 20

func mediaCall(ctx context.Context, userURL bool, method, url, key string, body any) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := httpClient(userURL).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return nil, &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(b)),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	return resp, nil
}

func readMedia(resp *http.Response, fallbackMime string) (*Media, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, mediaMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("the provider sent an empty file")
	}
	if len(data) > mediaMaxBytes {
		return nil, errors.New("the provider sent a file that is too large")
	}
	mime := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if mime == "" || mime == "application/octet-stream" || strings.HasPrefix(mime, "text/") || mime == "application/json" {
		mime = http.DetectContentType(data)
		if mime == "application/octet-stream" || strings.HasPrefix(mime, "text/") {
			mime = fallbackMime
		}
	}
	return &Media{Data: data, Mime: mime}, nil
}

// GenerateImage asks an image model for one picture.
func GenerateImage(ctx context.Context, req MediaRequest) (*Media, error) {
	resp, err := mediaCall(ctx, req.UserURL, http.MethodPost, req.BaseURL+"/images/generations", req.APIKey,
		map[string]any{"model": req.Model, "prompt": req.Prompt, "n": 1})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Data []struct {
			B64 string `json:"b64_json"`
			URL string `json:"url"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*mediaMaxBytes)).Decode(&out); err != nil {
		return nil, fmt.Errorf("the image endpoint did not answer with JSON: %w", err)
	}
	if out.Error != nil {
		return nil, errors.New("provider error: " + out.Error.Message)
	}
	if len(out.Data) == 0 {
		return nil, errors.New("the image endpoint returned no picture")
	}
	if b64 := out.Data[0].B64; b64 != "" {
		if i := strings.Index(b64, ","); strings.HasPrefix(b64, "data:") && i > 0 {
			b64 = b64[i+1:]
		}
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(data) == 0 {
			return nil, errors.New("the image endpoint returned a picture that could not be read")
		}
		return &Media{Data: data, Mime: http.DetectContentType(data)}, nil
	}
	link := out.Data[0].URL
	if !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "http://") {
		return nil, errors.New("the image endpoint returned neither a picture nor a link to one")
	}
	file, err := mediaCall(ctx, req.UserURL, http.MethodGet, link, "", nil) // a signed link: no key is sent to it
	if err != nil {
		return nil, fmt.Errorf("could not fetch the picture: %w", err)
	}
	return readMedia(file, "image/png")
}

// Speak asks a voice model to read a text aloud.
func Speak(ctx context.Context, req MediaRequest) (*Media, error) {
	voice := req.Voice
	if voice == "" {
		voice = "alloy"
	}
	resp, err := mediaCall(ctx, req.UserURL, http.MethodPost, req.BaseURL+"/audio/speech", req.APIKey,
		map[string]any{"model": req.Model, "input": req.Prompt, "voice": voice})
	if err != nil {
		return nil, err
	}
	return readMedia(resp, "audio/mpeg")
}

// GenerateVideo asks a video model for a short film and waits until it is
// ready. poll is how long to wait between looks (0 = 5 seconds); the context
// sets the overall limit.
func GenerateVideo(ctx context.Context, req MediaRequest, poll time.Duration) (*Media, error) {
	if poll <= 0 {
		poll = 5 * time.Second
	}
	type job struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	read := func(resp *http.Response) (*job, error) {
		defer resp.Body.Close()
		j := &job{}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(j); err != nil {
			return nil, fmt.Errorf("the video endpoint did not answer with JSON: %w", err)
		}
		return j, nil
	}
	resp, err := mediaCall(ctx, req.UserURL, http.MethodPost, req.BaseURL+"/videos", req.APIKey, map[string]any{"model": req.Model, "prompt": req.Prompt})
	if err != nil {
		return nil, err
	}
	j, err := read(resp)
	if err != nil {
		return nil, err
	}
	if j.ID == "" {
		return nil, errors.New("the video endpoint did not start a job")
	}
	for {
		switch strings.ToLower(j.Status) {
		case "completed", "succeeded", "done":
			file, err := mediaCall(ctx, req.UserURL, http.MethodGet, req.BaseURL+"/videos/"+j.ID+"/content", req.APIKey, nil)
			if err != nil {
				return nil, fmt.Errorf("could not fetch the video: %w", err)
			}
			return readMedia(file, "video/mp4")
		case "failed", "cancelled", "canceled", "error":
			if j.Error != nil && j.Error.Message != "" {
				return nil, errors.New("the provider could not make the video: " + j.Error.Message)
			}
			return nil, errors.New("the provider could not make the video")
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("the video took too long to make")
		case <-time.After(poll):
		}
		resp, err := mediaCall(ctx, req.UserURL, http.MethodGet, req.BaseURL+"/videos/"+j.ID, req.APIKey, nil)
		if err != nil {
			return nil, err
		}
		id := j.ID
		if j, err = read(resp); err != nil {
			return nil, err
		}
		if j.ID == "" {
			j.ID = id
		}
	}
}
