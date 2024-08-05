package sentencepiece

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
)

const debugEncode = true

type Encoder struct {
	model *ModelProto

	pieces   map[string]int
	reserved map[string]int

	// unknownId is the token identifier of the UNKNOWN piece
	unknownId int

	// userDefined is a set of symbols that are of "user-defined" type in the
	// model proto.
	userDefined map[string]struct{}

	// byteTokens is a cache of byte values and the tokens they represent
	byteTokens map[byte]Token
}

func NewEncoder(protoFile string) (*Encoder, error) {
	b, err := os.ReadFile(protoFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read %q: %v", protoFile, err)
	}

	var mp ModelProto
	err = proto.Unmarshal(b, &mp)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal %q: %v", protoFile, err)
	}

	tspec := mp.GetTrainerSpec()
	if tspec.GetModelType() != TrainerSpec_BPE {
		return nil, fmt.Errorf("model type %s not supported", tspec.GetModelType())
	}

	userDefined := make(map[string]struct{})
	pieces := make(map[string]int)
	reserved := make(map[string]int)
	byteTokens := make(map[byte]Token)
	unkId := -1

	for i, piece := range mp.GetPieces() {
		isNormalPiece := (piece.GetType() == ModelProto_SentencePiece_NORMAL ||
			piece.GetType() == ModelProto_SentencePiece_USER_DEFINED ||
			piece.GetType() == ModelProto_SentencePiece_UNUSED)

		if isNormalPiece {
			pieces[piece.GetPiece()] = i
		} else {
			reserved[piece.GetPiece()] = i
		}

		if piece.GetType() == ModelProto_SentencePiece_USER_DEFINED {
			userDefined[piece.GetPiece()] = struct{}{}
		} else if piece.GetType() == ModelProto_SentencePiece_UNKNOWN {
			if unkId > 0 {
				return nil, fmt.Errorf("unk redefined")
			}
			unkId = i
		} else if piece.GetType() == ModelProto_SentencePiece_BYTE {
			if !tspec.GetByteFallback() {
				return nil, fmt.Errorf("byte piece %q is found although `byte_fallback=false`", piece.GetPiece())
			}
			bv := convertHexValue(piece.GetPiece())
			if bv >= 0 && bv < 256 {
				byteTokens[byte(bv)] = Token{ID: i, Text: piece.GetPiece()}
			}
		}
	}

	if unkId < 0 {
		return nil, fmt.Errorf("unk symbol is not defined")
	}

	// In case byte_fallback is specified, make sure that all 256 possible byte
	// values were found.
	if tspec.GetByteFallback() {
		for i := 0; i < 256; i++ {
			if _, found := byteTokens[byte(i)]; !found {
				return nil, fmt.Errorf("byte value 0x%02X not found", i)
			}
		}
	}

	return &Encoder{
		model:       &mp,
		userDefined: userDefined,
		byteTokens:  byteTokens,
		unknownId:   unkId,
		pieces:      pieces,
		reserved:    reserved,
	}, nil
}

func (enc *Encoder) Encode(text string) []Token {
	// Theoretically SentencePiece performs unicode normalization on the input
	// text and has some options for adding dummpy whitespace prefixes or
	// trimming whitespace. However, the model we're working with has a very
	// simple normalizer that does none of this. These options can be added
	// in the future if needed.
	text = replaceSeparator(text)

	var symbols []string

	for {
		slen, _ := enc.symbolMatch(text)
		symbols = append(symbols, text[:slen])
		text = text[slen:]

		if len(text) == 0 {
			break
		}
	}

	debugShowSymbols := func(prefix string) {
		if debugEncode {
			fmt.Printf("%s: [", prefix)
			for _, s := range symbols {
				fmt.Printf("%q ", s)
			}
			fmt.Printf("]\n")
		}
	}

	debugShowSymbols("initial")

	// TODO: the performance here is quadratic because of the reshuffling of
	// the (potentially large) symbols slice.
	// Needs a more sophisticated algorithm.
	for {
		var bestScore float32 = -math.MaxFloat32
		bestMergeIndex := -1

		for i := 0; i < len(symbols)-1; i++ {
			pair := symbols[i] + symbols[i+1]
			if pairId, found := enc.pieces[pair]; found {
				pairScore := enc.model.GetPieces()[pairId].GetScore()
				if pairScore > bestScore {
					bestScore = pairScore
					bestMergeIndex = i
				}
			}
		}

		if bestMergeIndex >= 0 {
			// Found a pair to merge
			pair := symbols[bestMergeIndex] + symbols[bestMergeIndex+1]
			symbols = slices.Replace(symbols, bestMergeIndex, bestMergeIndex+2, pair)

			debugShowSymbols("merge")
		} else {
			// No more pairs to merge; we're done!
			break
		}
	}

	// Here symbols is a list with all possible merges done. Create a list of
	// tokens, and convert unknown symbols to their byte-by-byte tokens.
	tokens := make([]Token, 0, len(symbols))

	for _, symb := range symbols {
		id := enc.symbolToID(symb)

		if id == enc.unknownId && enc.model.GetTrainerSpec().GetByteFallback() {
			// Decompose this symbol into bytes, and report each byte as a separate
			// token.
			for i := 0; i < len(symb); i++ {
				tokens = append(tokens, enc.byteTokens[symb[i]])
			}
		} else {
			tokens = append(tokens, Token{ID: id, Text: symb})
		}
	}

	return tokens
}

// TODO: unknown unicode rules are marked as UNKNOWN id, and then
// SentencePieceProcessor::PopulateSentencePieceText decomposes them
// to bytes if byte fallback is enabled
// PieceToId: if found in reserved retunrs that, otherwise pieces, otherwise
// unknown id

// replaceSeparator replaces spaces by the whitespace separator used by
// the model.
func replaceSeparator(text string) string {
	return strings.ReplaceAll(text, " ", "▁")
}

// symbolMatch finds the length of the first symbol in text. A symbol is either
// a user-defined symbol from the proto or a single rune. The second return
// value is true iff a user-defined symbol was matched.
func (enc *Encoder) symbolMatch(text string) (int, bool) {
	maxLen := 0

	// TODO: optimize this using a trie
	for us := range enc.userDefined {
		if strings.HasPrefix(text, us) {
			if len(us) > maxLen {
				maxLen = len(us)
			}
		}
	}

	if maxLen > 0 {
		return maxLen, true
	} else {
		_, rlen := utf8.DecodeRuneInString(text)
		return rlen, false
	}
}

// symbolToID finds the right ID for the given textual symbol, or returns
// enc.unknownId if the symbol is unknown.
func (enc *Encoder) symbolToID(symbol string) int {
	if id, found := enc.reserved[symbol]; found {
		return id
	}
	if id, found := enc.pieces[symbol]; found {
		return id
	}
	return enc.unknownId
}

// convertHexValue converts strings of the form "<0xXY>" to the (unsigned)
// integer value of the hexadecimal number XY. -1 is returned for bad input.
func convertHexValue(bv string) int {
	bv = strings.TrimPrefix(bv, "<0x")
	bv = strings.TrimSuffix(bv, ">")
	n, err := strconv.ParseInt(bv, 16, 32)
	if err != nil {
		return -1
	}
	return int(n)
}
