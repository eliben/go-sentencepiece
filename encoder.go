package sentencepiece

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/eliben/go-sentencepiece/internal/priorityqueue"
	"google.golang.org/protobuf/proto"
)

const debugEncode = false

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

// Encode tokenizes the input text and returns a list of Tokens.
func (enc *Encoder) Encode(text string) []Token {
	text = normalize(text)

	// We begin by having each token a single Unicode character (or a user-defined
	// string), and will iteratively merge them into larger and larger symbols
	// until we have the final list of tokens.
	// Since this list of symbols changes a lot, we represent it as a
	// doubly-linked list in the symList slice. Each element in this slice has
	// prev/next links to the next "live" symbol in the list; noMerge means this
	// is a user-defined symbol we're not allowed to merge with neighbors.
	// After the algorithm is finished, many elements in symList will be "dead"
	// (unreachable by next/prev links from the first element).

	type symListElem struct {
		prev, next int
		noMerge    bool
		symbol     string
	}

	symList := make([]symListElem, 0, len(text))

	for {
		// Match the next symbol in text
		slen, found := enc.symbolMatch(text)

		// Append a list element for this symbol; note that this element will be
		// at index len(symList), so prev/next are set up accordingly.
		elem := symListElem{
			noMerge: found,
			symbol:  text[:slen],
			prev:    len(symList) - 1,
			next:    len(symList) + 1,
		}
		symList = append(symList, elem)

		// Advance the text slice to the next symbol; if no more text, we're done.
		text = text[slen:]
		if len(text) == 0 {
			break
		}
	}

	if len(symList) == 0 {
		return nil
	}
	symList[len(symList)-1].next = -1

	debugShowSymList := func(prefix string) {
		if debugEncode {
			fmt.Println(prefix)
			for i, elem := range symList {
				fmt.Printf("[%3d]: [prev: %3v, next: %3d, noMerge: %v] %q\n", i, elem.prev, elem.next, elem.noMerge, elem.symbol)
			}
		}
	}
	debugShowSymList("initial")

	type mergeCandidate struct {
		left, right int
		length      int
		score       float32
	}

	mergeQueue := priorityqueue.New(func(a, b mergeCandidate) int {
		if a.score > b.score || (a.score == b.score && a.left < b.left) {
			return 1
		} else {
			return -1
		}
	})

	suggestNewMergePair := func(left, right int) {
		if left == -1 || right == -1 || symList[left].noMerge || symList[right].noMerge {
			return
		}

		mergedSymbol := symList[left].symbol + symList[right].symbol
		if id, found := enc.pieces[mergedSymbol]; found {
			mergeQueue.Insert(mergeCandidate{
				left:   left,
				right:  right,
				length: len(mergedSymbol),
				score:  enc.model.GetPieces()[id].GetScore(),
			})
		}
	}

	// Seed the merge queue with all pairs of symbols from symList
	for i := 1; i < len(symList); i++ {
		suggestNewMergePair(i-1, i)
	}

	// Main loop
	for mergeQueue.Len() > 0 {
		candidate := mergeQueue.PopMax()
		leftSymbol := symList[candidate.left]
		rightSymbol := symList[candidate.right]

		// Make sure this candidate is not out of date. If one of its parts was
		// already merged with another symbol, just skip this candidate.
		// TODO: do I really need the len == 0 here?
		if len(leftSymbol.symbol) == 0 || len(rightSymbol.symbol) == 0 ||
			len(leftSymbol.symbol)+len(rightSymbol.symbol) != candidate.length {
			continue
		}

		// Do the merge:
		// 1. Merge the concatenation of leftSymbol and rightSymbol into leftSymbol
		// 2. Update prev/next pointers
		// 3. Add merge suggestions for the newly merged symbol with its neighbors
		symList[candidate.left].symbol = leftSymbol.symbol + rightSymbol.symbol

		symList[candidate.left].next = rightSymbol.next
		if rightSymbol.next >= 0 {
			symList[rightSymbol.next].prev = candidate.left
		}
		symList[candidate.right].symbol = ""

		debugShowSymList(fmt.Sprintf("merged %d and %d", candidate.left, candidate.right))

		suggestNewMergePair(leftSymbol.prev, candidate.left)
		suggestNewMergePair(candidate.left, rightSymbol.next)
	}

	// TODO: document
	tokens := make([]Token, 0, len(symList))
	for i := 0; i >= 0; i = symList[i].next {
		symbol := symList[i].symbol
		id := enc.symbolToID(symbol)

		if id == enc.unknownId && enc.model.GetTrainerSpec().GetByteFallback() {
			// Decompose this symbol into bytes, and report each byte as a separate
			// token.
			for i := 0; i < len(symbol); i++ {
				tokens = append(tokens, enc.byteTokens[symbol[i]])
			}
		} else {
			tokens = append(tokens, Token{ID: id, Text: symbol})
		}
	}

	return tokens
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
