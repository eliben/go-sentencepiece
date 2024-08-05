package sentencepiece

import (
	"fmt"
	"os"
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

func TestNew(t *testing.T) {
	enc := createEncoder(t)
	// TODO: verify 245 is the right len
	wantLen := 245
	if len(enc.userDefined) != wantLen {
		t.Errorf("got len(userDefined)=%v, want %v", len(enc.userDefined), wantLen)
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

func TestEncode(t *testing.T) {
	enc := createEncoder(t)
	//tk := enc.Encode("hi <td> bye")
	//fmt.Println(tk)

	tk := enc.Encode("hiƻ <td>🤨there ⇲bob, สวัสดี")
	fmt.Println(tk)

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
