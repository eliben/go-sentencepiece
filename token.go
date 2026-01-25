package sentencepiece

import "fmt"

// Token represents a single token from the input text. ID is a unique token
// identifier that the model uses in its internal representation. Text is
// the piece of text this token represents.
type Token struct {
	ID   int
	Text string
}

func (t Token) String() string {
	return fmt.Sprintf("Token{ID: %v, Text: %q}", t.ID, t.Text)
}

// TokenSpan represents the byte span of a token in the original text.
// Start and End are byte offsets (not rune offsets), suitable for slicing
// Go strings directly: originalText[span.Start:span.End].
type TokenSpan struct {
	Start int // start byte position (inclusive)
	End   int // end byte position (exclusive)
}

// TokenWithSpan represents a token with its byte span in the original text.
// This is useful for token classification tasks (NER, chunking) where you need
// to map token predictions back to positions in the original text.
type TokenWithSpan struct {
	Token
	Span TokenSpan
}

func (t TokenWithSpan) String() string {
	return fmt.Sprintf("TokenWithSpan{ID: %v, Text: %q, Span: [%d:%d]}", t.ID, t.Text, t.Span.Start, t.Span.End)
}
