package sentencepiece

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

func createProcessor(t testing.TB) *Processor {
	t.Helper()
	protoFile := os.Getenv("MODELPATH")
	if protoFile == "" {
		t.Fatal("Need MODELPATH env var to run tests")
	}

	proc, err := NewProcessorFromPath(protoFile)
	if err != nil {
		t.Error(err)
	}
	return proc
}

func TestEncodeIDs(t *testing.T) {
	proc := createProcessor(t)

	var tests = []struct {
		text    string
		wantIDs []int
	}{
		{"hello world", []int{17534, 2134}},
		{"12345", []int{235274, 235284, 235304, 235310, 235308}},
		{"  ", []int{139}},
		{"   ", []int{140}},
		{"        ", []int{145}},
		{"ҔӌԐڎ", []int{427, 365, 428, 357, 429, 361, 435, 359}},
		{" <mask>  <pad>", []int{235248, 4, 139, 235322, 8939, 235313}},
		{"<table><th></th></table>", []int{169, 175, 183, 177}},
		{"one line\nand another line", []int{785, 2017, 108, 639, 2550, 2017}},
		{"Language: English\r\n\r\nCredits: Produced by David Widger\r\n", []int{14357, 235292, 4645, 235316, 108, 235316, 108, 34711, 235292, 99662, 731, 6046, 37303, 1197, 235316, 108}},
		{"Bienvenido a este proyecto", []int{176831, 476, 4004, 25431}},
		{"अस्मिन् परियोजनायां स्वागतम्", []int{236088, 22740, 212361, 18029, 14480, 19900, 146166, 6751, 235563, 56545, 44071, 235550, 26989}},
		{"if allow == true { return x;} else {return x+y;}", []int{648, 2765, 1159, 1382, 612, 2203, 1141, 22505, 1354, 612, 773, 1141, 235340, 235267, 22505}},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := proc.Encode(tt.text)

			var gotIDs []int
			for _, t := range got {
				gotIDs = append(gotIDs, t.ID)
			}

			if !slices.Equal(gotIDs, tt.wantIDs) {
				t.Errorf("got  %v\nwant: %v\n", gotIDs, tt.wantIDs)
			}
		})
	}
}

func TestProcessorWithText(t *testing.T) {
	proc := createProcessor(t)

	var tests = []struct {
		text       string
		wantTokens []Token
	}{
		{"hi <td> bye",
			[]Token{
				{544, "hi"},
				{235248, "▁"},
				{176, "<td>"},
				{44788, "▁bye"},
			}},
		{"hiƻ <td>🤨there ⇲bob, สวัสดี",
			[]Token{
				{544, "hi"},
				{415, "<0xC6>"},
				{404, "<0xBB>"},
				{235248, "▁"},
				{176, "<td>"},
				{241847, "🤨"},
				{11048, "there"},
				{235248, "▁"},
				{248372, "⇲"},
				{26242, "bob"},
				{235269, ","},
				{12515, "▁ส"},
				{151622, "วัส"},
				{28890, "ดี"},
			}},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := proc.Encode(tt.text)
			if !slices.Equal(got, tt.wantTokens) {
				t.Errorf("got  %v\nwant: %v\n", got, tt.wantTokens)
			}
		})
	}
}

func TestSymbolMatch(t *testing.T) {
	proc := createProcessor(t)

	var tests = []struct {
		text      string
		wantLen   int
		wantFound bool
	}{
		{"<td>", 4, true},
		{"<s>", 3, true},
		{"</s>", 4, true},
		{"<start_of_turn>", 15, true},
		{"<start_of_turn!", 1, false},
		{"▁▁", 6, true},
		{"▁▁▁▁▁▁", 18, true},
		{"bob", 1, false},
		{"🤨", 4, false},
		{"สวัสดี", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			gotLen, gotFound := proc.symbolMatch(tt.text)
			if gotLen != tt.wantLen || gotFound != tt.wantFound {
				t.Errorf("got (%v, %v), want (%v, %v)", gotLen, gotFound, tt.wantLen, tt.wantFound)
			}
		})
	}
}

func TestConvertHexValue(t *testing.T) {
	var tests = []struct {
		in    string
		wantN int
	}{
		{"<0x40>", 64},
		{"<0x00>", 0},
		{"<0x1a>", 26},
		{"<0xF3>", 243},

		{"0x12>", -1},
		{"<x12>", -1},
		{"<012>", -1},
		{"<0xTA>", -1},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			gotN := convertHexValue(tt.in)
			if gotN != tt.wantN {
				t.Errorf("got %v, want %v", gotN, tt.wantN)
			}
		})
	}
}

func TestDecoder(t *testing.T) {
	proc := createProcessor(t)

	var tests = []struct {
		IDs      []int
		wantText string
	}{
		{[]int{17534, 2134}, "hello world"},
		{[]int{427, 365, 428, 357, 29422, 1653, 427, 365, 428, 357}, "Ҕӌnever againҔӌ"},
		{[]int{785, 2017, 108, 639, 2550, 2017}, "one line\nand another line"},
		{[]int{1001, 1002, 1003, 1004}, "buark}) res"},
		{[]int{111001, 111002, 111003, 111004}, " Wichita EducaçãoVocabulary天堂"},
		{[]int{139}, "  "},
		{[]int{140}, "   "},
		{[]int{145}, "        "},
		{[]int{441, 401, 387}, "ส"},
		{[]int{411, 380}, "£"},

		// control IDs (0, 1, 2)
		{[]int{2, 411, 380}, "£"},
		{[]int{1, 2, 411, 380}, "£"},
		{[]int{2, 411, 380, 0, 1, 2, 0}, "£"},

		// unknown (id=3)
		{[]int{3, 411, 380}, " ⁇ £"},
		{[]int{3, 3, 1000, 3}, " ⁇  ⁇ ew ⁇ "},

		// invalid bytes for UTF-8, produce "invalid unicode" runes
		{[]int{349, 349, 349}, "���"},
		{[]int{800, 348, 500, 348}, "sed�it�"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.IDs), func(t *testing.T) {
			got := proc.Decode(tt.IDs)
			if got != tt.wantText {
				t.Errorf("got %q\nwant %q\n", got, tt.wantText)
			}
		})
	}
}

func TestDecodeTokens(t *testing.T) {
	proc := createProcessor(t)
	wantText := "hello   world"
	tokens := []Token{
		Token{17534, "xxx"},
		Token{139, "xxx"},
		Token{2134, "xxx"}}

	text := proc.DecodeTokens(tokens)
	if text != wantText {
		t.Errorf("got %q, want %q", text, wantText)
	}
}

func TestInfo(t *testing.T) {
	proc := createProcessor(t)
	info := proc.ModelInfo()

	// Assumes we use the known model file
	wantVocabSize := 256000
	wantBOS := 2
	wantEOS := 1
	wantPAD := 0
	wantUNK := 3

	if info.VocabularySize != wantVocabSize {
		t.Errorf("got %v, want %v", info.VocabularySize, wantVocabSize)
	}
	if info.BeginningOfSentenceID != wantBOS {
		t.Errorf("got %v, want %v", info.BeginningOfSentenceID, wantBOS)
	}
	if info.EndOfSentenceID != wantEOS {
		t.Errorf("got %v, want %v", info.EndOfSentenceID, wantEOS)
	}
	if info.PadID != wantPAD {
		t.Errorf("got %v, want %v", info.PadID, wantPAD)
	}
	if info.UnknownID != wantUNK {
		t.Errorf("got %v, want %v", info.UnknownID, wantUNK)
	}
}

// TestMergedSymbolExceedsMaxPieceLength tests that encoding doesn't panic
// when BPE attempts to merge two symbols whose combined length exceeds
// maxPieceLength. See https://github.com/eliben/go-sentencepiece/pull/8
func TestMergedSymbolExceedsMaxPieceLength(t *testing.T) {
	proc := createProcessor(t)

	// These test cases previously caused a panic:
	// panic: runtime error: slice bounds out of range [:96] with capacity 93
	testCases := []string{
		strings.Repeat("—", 32), // 32 em dashes (U+2014, 3 bytes each = 96 bytes)
		strings.Repeat("…", 32), // 32 ellipses (U+2026, 3 bytes each = 96 bytes)
		strings.Repeat("—", 64), // More em dashes
		strings.Repeat("…", 64), // More ellipses
	}

	for _, text := range testCases {
		t.Run(fmt.Sprintf("len=%d", len(text)), func(t *testing.T) {
			// Should not panic
			tokens := proc.Encode(text)
			if len(tokens) == 0 {
				t.Errorf("expected at least one token for input of length %d", len(text))
			}

			// Verify round-trip works
			decoded := proc.DecodeTokens(tokens)
			if decoded != text {
				t.Errorf("round-trip failed: got %q, want %q", decoded, text)
			}
		})
	}
}

func TestBuildPositionMap(t *testing.T) {
	// This test doesn't require a model file
	tests := []struct {
		input    string
		wantMap  []int
	}{
		// No spaces: positions map 1:1
		{"hello", []int{0, 1, 2, 3, 4, 5}},

		// Single space at start: " " (1 byte) -> "▁" (3 bytes)
		// Original: " hi" = positions 0, 1, 2
		// Normalized: "▁hi" = positions 0,0,0, 1, 2, 3 (normalized), maps to 0,0,0, 1, 2, 3 (original)
		{" hi", []int{0, 0, 0, 1, 2, 3}},

		// Space in middle: "a b"
		// Original: 'a'=0, ' '=1, 'b'=2, end=3
		// Normalized: "a▁b" = 'a'=0, '▁'=[1,2,3], 'b'=4, end=5
		// Maps: norm[0]=0, norm[1]=1, norm[2]=1, norm[3]=1, norm[4]=2, norm[5]=3
		{"a b", []int{0, 1, 1, 1, 2, 3}},

		// Multiple spaces
		{"a b c", []int{0, 1, 1, 1, 2, 3, 3, 3, 4, 5}},

		// Empty string
		{"", []int{0}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := buildPositionMap(tt.input)
			if !slices.Equal(got, tt.wantMap) {
				t.Errorf("buildPositionMap(%q):\n  got  %v\n  want %v", tt.input, got, tt.wantMap)
			}
		})
	}
}

func TestEncodeWithSpans(t *testing.T) {
	proc := createProcessor(t)

	tests := []struct {
		text      string
		wantSpans []TokenSpan
	}{
		// Single word - spans should cover the whole word
		{"hello", []TokenSpan{{0, 5}}},

		// Two words - each token should have correct span
		{"hello world", []TokenSpan{{0, 5}, {5, 11}}},

		// Verify spans can extract original text
		{"one line", nil}, // Just verify extraction works
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			tokens := proc.EncodeWithSpans(tt.text)

			if len(tokens) == 0 {
				t.Fatalf("expected at least one token")
			}

			// Verify IDs match regular Encode
			regularTokens := proc.Encode(tt.text)
			if len(tokens) != len(regularTokens) {
				t.Errorf("token count mismatch: EncodeWithSpans=%d, Encode=%d", len(tokens), len(regularTokens))
			}
			for i := range tokens {
				if i < len(regularTokens) && tokens[i].ID != regularTokens[i].ID {
					t.Errorf("token %d ID mismatch: EncodeWithSpans=%d, Encode=%d", i, tokens[i].ID, regularTokens[i].ID)
				}
			}

			// If we have expected spans, verify them
			if tt.wantSpans != nil {
				for i, want := range tt.wantSpans {
					if i >= len(tokens) {
						break
					}
					got := tokens[i].Span
					if got.Start != want.Start || got.End != want.End {
						t.Errorf("token %d span: got [%d:%d], want [%d:%d]", i, got.Start, got.End, want.Start, want.End)
					}
				}
			}

			// Verify spans are valid and don't overlap incorrectly
			for i, tok := range tokens {
				if tok.Span.Start < 0 || tok.Span.End > len(tt.text) {
					t.Errorf("token %d: span [%d:%d] out of bounds for text len %d", i, tok.Span.Start, tok.Span.End, len(tt.text))
				}
				if tok.Span.Start > tok.Span.End {
					t.Errorf("token %d: invalid span [%d:%d]", i, tok.Span.Start, tok.Span.End)
				}
			}

			// Verify spans are monotonically non-decreasing
			for i := 1; i < len(tokens); i++ {
				if tokens[i].Span.Start < tokens[i-1].Span.Start {
					t.Errorf("token %d start (%d) < token %d start (%d)", i, tokens[i].Span.Start, i-1, tokens[i-1].Span.Start)
				}
			}
		})
	}
}

func TestEncodeWithSpansExtraction(t *testing.T) {
	proc := createProcessor(t)

	// Test that we can extract meaningful substrings from the original text
	text := "Hello world, this is a test!"
	tokens := proc.EncodeWithSpans(text)

	// Collect all extracted pieces and verify they cover the text
	var extracted []string
	for _, tok := range tokens {
		piece := text[tok.Span.Start:tok.Span.End]
		extracted = append(extracted, piece)
	}

	// The extracted pieces should join to form something close to the original
	// (modulo how BPE splits things)
	joined := strings.Join(extracted, "")

	// Verify we extracted real substrings (not empty or out of bounds)
	for i, tok := range tokens {
		if tok.Span.End < tok.Span.Start {
			t.Errorf("token %d has invalid span: [%d:%d]", i, tok.Span.Start, tok.Span.End)
		}
	}

	t.Logf("Original: %q", text)
	t.Logf("Joined:   %q", joined)
	t.Logf("Tokens:   %v", tokens)
}
