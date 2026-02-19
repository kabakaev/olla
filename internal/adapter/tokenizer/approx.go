package tokenizer

import (
	"context"
	"math"
)

// ApproxTokenizer estimates token count based on character length.
// It assumes roughly 4 characters per token, which is a common heuristic for English text.
type ApproxTokenizer struct{}

func NewApproxTokenizer() *ApproxTokenizer {
	return &ApproxTokenizer{}
}

func (t *ApproxTokenizer) CountTokens(ctx context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	// Average English word is 4.7 chars. Tokens are parts of words.
	// 4 chars/token is a safe upper bound estimate for many use cases,
	// but for context limits we might want to be slightly conservative.
	// Let's use 3.5 chars/token to be safer (over-estimate count).
	count := float64(len(text)) / 3.5
	return int(math.Ceil(count)), nil
}
