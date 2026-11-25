// Package tokens counts tokens of text mcpDeck returns to agents with an
// open, offline encoder (SES-3). Counts are for comparing before and after,
// not a vendor's own tokenizer.
package tokens

import (
	"sync"

	"github.com/pkoukk/tiktoken-go"
	loader "github.com/pkoukk/tiktoken-go-loader"
)

// Encoder is the name shown next to every count.
const Encoder = "cl100k_base"

var (
	once sync.Once
	enc  *tiktoken.Tiktoken
)

func get() *tiktoken.Tiktoken {
	once.Do(func() {
		tiktoken.SetBpeLoader(loader.NewOfflineLoader())
		e, err := tiktoken.GetEncoding(Encoder)
		if err == nil {
			enc = e
		}
	})
	return enc
}

// Count returns the number of tokens in s.
func Count(s string) int {
	if s == "" {
		return 0
	}
	e := get()
	if e == nil {
		return (len(s) + 3) / 4
	}
	return len(e.Encode(s, nil, nil))
}
