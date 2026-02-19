package tokenizer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/thushan/olla/internal/core/ports"
)

// RemoteTokenizer uses an upstream service (like llama.cpp) to count tokens.
type RemoteTokenizer struct {
	client     *http.Client
	url        string
	fallback   ports.Tokenizer
}

type tokenizeRequest struct {
	Content string `json:"content"`
}

type tokenizeResponse struct {
	Tokens []int `json:"tokens"`
}

func NewRemoteTokenizer(url string, timeout time.Duration) *RemoteTokenizer {
	return &RemoteTokenizer{
		client: &http.Client{
			Timeout: timeout,
		},
		url:      url,
		fallback: NewApproxTokenizer(),
	}
}

func (t *RemoteTokenizer) CountTokens(ctx context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}

	reqBody, err := json.Marshal(tokenizeRequest{Content: text})
	if err != nil {
		return t.fallback.CountTokens(ctx, text)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewBuffer(reqBody))
	if err != nil {
		return t.fallback.CountTokens(ctx, text)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		// Log error? For now, fallback silently or maybe return error if strict?
		// Plan said "fallback", so we fallback.
		return t.fallback.CountTokens(ctx, text)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return t.fallback.CountTokens(ctx, text)
	}

	var parsed tokenizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return t.fallback.CountTokens(ctx, text)
	}

	return len(parsed.Tokens), nil
}
