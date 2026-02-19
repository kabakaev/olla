package ports

import "context"

// Tokenizer calculates the number of tokens in a text string.
type Tokenizer interface {
	// CountTokens returns the number of tokens in the given text.
	CountTokens(ctx context.Context, text string) (int, error)
}
