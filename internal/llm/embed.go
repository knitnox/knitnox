package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// EmbedRequest asks an OpenAI-compatible /embeddings endpoint to turn texts
// into vectors (OpenAI, DeepInfra, Together, Ollama, vLLM, LiteLLM, ...).
type EmbedRequest struct {
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int // 0 = the model's own size; only some models can shrink
	Input      []string
	UserURL    bool // a user's own provider: guarded addresses are refused
}

// Embed returns one vector per input, in input order.
func Embed(ctx context.Context, req EmbedRequest) ([][]float32, error) {
	if len(req.Input) == 0 {
		return nil, nil
	}
	body := map[string]any{"model": req.Model, "input": req.Input, "encoding_format": "float"}
	if req.Dimensions > 0 {
		body["dimensions"] = req.Dimensions
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.BaseURL+"/embeddings", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		hreq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	resp, err := httpClient(req.UserURL).Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return nil, &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(b)),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("the embedding endpoint did not answer with JSON: %w", err)
	}
	if out.Error != nil {
		return nil, errors.New("provider error: " + out.Error.Message)
	}
	if len(out.Data) != len(req.Input) {
		return nil, fmt.Errorf("asked for %d embeddings, got %d", len(req.Input), len(out.Data))
	}
	vecs := make([][]float32, len(req.Input))
	for i, d := range out.Data {
		idx := d.Index
		if idx < 0 || idx >= len(vecs) || vecs[idx] != nil {
			idx = i // providers that omit or repeat the index answer in order
		}
		if len(d.Embedding) == 0 {
			return nil, errors.New("the embedding endpoint returned an empty vector")
		}
		vecs[idx] = d.Embedding
	}
	for _, v := range vecs {
		if v == nil {
			return nil, errors.New("the embedding endpoint skipped an input")
		}
	}
	return vecs, nil
}
