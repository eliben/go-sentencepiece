package sentencepiece

import (
	"os"
	"slices"
	"testing"
)

func createEncoder(t *testing.T) *Encoder {
	t.Helper()
	protoFile := os.Getenv("MODELPATH")
	if protoFile == "" {
		t.Skip("Need MODELPATH set to run tests")
	}

	encoder, err := NewEncoder(protoFile)
	if err != nil {
		t.Error(err)
	}
	return encoder
}

func TestEncodeWithText(t *testing.T) {
	enc := createEncoder(t)

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
			got := enc.Encode(tt.text)
			if !slices.Equal(got, tt.wantTokens) {
				t.Errorf("got  %v\nwant: %v\n", got, tt.wantTokens)
			}
		})
	}
}

func TestSymbolMatch(t *testing.T) {
	enc := createEncoder(t)

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
			gotLen, gotFound := enc.symbolMatch(tt.text)
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
