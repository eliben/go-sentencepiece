package sentencepiece

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/eliben/go-sentencepiece/internal/model"
	"github.com/eliben/go-sentencepiece/internal/prefixmatcher"
	"github.com/eliben/go-sentencepiece/internal/priorityqueue"
	"google.golang.org/protobuf/proto"
)

const debugEncode = false

// Processor represents a SentencePiece processor (tokenizer).
// A Processor converts input text into a sequence of tokens LLMs use, and back.
// The mapping between token IDs and the text they represent is read from the
// model proto (provided to the constructor); it's the same between all calls
// to the Encode method.
//
// The term "processor" comes from the original C++ SentencePiece library and
// its Python bindings.
type Processor struct {
	model *model.ModelProto

	pieces   map[string]int
	reserved map[string]int

	// unknownID is the token identifier of the UNKNOWN piece
	unknownID int

	// userDefinedMatcher is a prefix matcher for symbols that are of
	// "user-defined" type in the model proto.
	userDefinedMatcher *prefixmatcher.PrefixMatcher

	// byte2Token is a cache of byte values and the tokens they represent
	byte2Token map[byte]Token

	// idToByte maps IDs to byte values they represent
	idToByte map[int]byte

	// maxPieceLength is the maximum length of a piece in the model.
	// This is used to preallocate a buffer for merging symbols.
	maxPieceLength int
}

// NewProcessorFromPath creates a new Processor from a file path to the protobuf
// data.
func NewProcessorFromPath(protoFile string) (*Processor, error) {
	f, err := os.Open(protoFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read %q: %v", protoFile, err)
	}
	defer f.Close()
	return NewProcessor(f)
}

// NewProcessor creates a new Processor from a reader with the protobuf data.
func NewProcessor(protoReader io.Reader) (*Processor, error) {
	b, err := io.ReadAll(protoReader)
	if err != nil {
		return nil, fmt.Errorf("unable to read protobuf data: %v", err)
	}

	var mp model.ModelProto
	err = proto.Unmarshal(b, &mp)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal protobuf: %v", err)
	}

	tspec := mp.GetTrainerSpec()
	if tspec.GetModelType() != model.TrainerSpec_BPE {
		return nil, fmt.Errorf("model type %s not supported", tspec.GetModelType())
	}

	nspec := mp.GetNormalizerSpec()
	if *nspec.AddDummyPrefix || *nspec.RemoveExtraWhitespaces {
		return nil, fmt.Errorf("normalizer spec options not supported: %s", nspec)
	}

	userDefined := make(map[string]bool)
	pieces := make(map[string]int)
	reserved := make(map[string]int)
	byte2Token := make(map[byte]Token)
	idToByte := make(map[int]byte)
	unkID := -1
	maxPieceLength := 0

	for i, piece := range mp.GetPieces() {
		isNormalPiece := (piece.GetType() == model.ModelProto_SentencePiece_NORMAL ||
			piece.GetType() == model.ModelProto_SentencePiece_USER_DEFINED ||
			piece.GetType() == model.ModelProto_SentencePiece_UNUSED)

		if isNormalPiece {
			pieces[piece.GetPiece()] = i
			maxPieceLength = max(maxPieceLength, len(piece.GetPiece()))
		} else {
			reserved[piece.GetPiece()] = i
		}

		if piece.GetType() == model.ModelProto_SentencePiece_USER_DEFINED {
			userDefined[piece.GetPiece()] = true
		} else if piece.GetType() == model.ModelProto_SentencePiece_UNKNOWN {
			if unkID > 0 {
				return nil, fmt.Errorf("unk redefined")
			}
			unkID = i
		} else if piece.GetType() == model.ModelProto_SentencePiece_BYTE {
			if !tspec.GetByteFallback() {
				return nil, fmt.Errorf("byte piece %q is found although `byte_fallback=false`", piece.GetPiece())
			}
			bv := convertHexValue(piece.GetPiece())
			if bv >= 0 && bv < 256 {
				byte2Token[byte(bv)] = Token{ID: i, Text: piece.GetPiece()}
				idToByte[i] = byte(bv)
			}
		}
	}

	if unkID < 0 {
		return nil, fmt.Errorf("unk symbol is not defined")
	}

	// In case byte_fallback is specified, make sure that all 256 possible byte
	// values were found.
	if tspec.GetByteFallback() {
		for i := 0; i < 256; i++ {
			if _, found := byte2Token[byte(i)]; !found {
				return nil, fmt.Errorf("byte value 0x%02X not found", i)
			}
		}
	}

	return &Processor{
		model:              &mp,
		userDefinedMatcher: prefixmatcher.NewFromSet(userDefined),
		byte2Token:         byte2Token,
		idToByte:           idToByte,
		unknownID:          unkID,
		pieces:             pieces,
		reserved:           reserved,
		maxPieceLength:     maxPieceLength,
	}, nil
}

// Encode tokenizes the input text and returns a list of Tokens.
func (proc *Processor) Encode(text string) []Token {
	text = normalize(text)

	// We begin by having each symbol a single Unicode character (or a
	// user-defined string), and will iteratively merge them into larger and
	// larger symbols until we have the final list of tokens.
	// Since this list of symbols changes a lot, we represent it as a
	// doubly-linked list in the symList slice. Each element in this slice has
	// prev/next links to the next "live" symbol in the list; noMerge means this
	// is a user-defined symbol we're not allowed to merge with neighbors.
	// After the algorithm is finished, many elements in symList will be "dead"
	// (unreachable by next/prev links from the first element).
	// This representation is inspired by the implementation of bpe::Model
	// in the SentencePiece C++ library.

	type symListElem struct {
		prev, next int
		noMerge    bool
		symbol     string
	}
	symList := make([]symListElem, 0, len(text))

	for {
		// Match the next symbol in text
		slen, found := proc.symbolMatch(text)

		// Append a list element for this symbol; note that this element will be
		// at index len(symList), so prev/next are set up accordingly.
		sym := symListElem{
			noMerge: found,
			symbol:  text[:slen],
			prev:    len(symList) - 1,
			next:    len(symList) + 1,
		}
		symList = append(symList, sym)

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
	nTokens := len(symList)

	debugShowSymList := func(prefix string) {
		if debugEncode {
			fmt.Println(prefix)
			for i, elem := range symList {
				fmt.Printf("[%3d]: [prev: %3v, next: %3d, noMerge: %v] %q\n", i, elem.prev, elem.next, elem.noMerge, elem.symbol)
			}
		}
	}
	debugShowSymList("initial")

	// To avoid repeating work, we manage a priority queue of "merge candidates".
	// Each candidate has pointers to the symList list for the left and right
	// symbol in the pair, as well as the combined symbol's score.
	// The priority of merging is determined by this score, with position as
	// the tie-breaker (earlier pairs are preferred).
	type mergeCandidate struct {
		left, right int
		length      int
		score       float32
	}

	mergeQueue := priorityqueue.New(len(symList), func(a, b mergeCandidate) int {
		if a.score > b.score || (a.score == b.score && a.left < b.left) {
			return 1
		}
		return -1
	})

	buf := make([]byte, proc.maxPieceLength) // reusable buffer for merging symbols
	mergeSymbols := func(x, y symListElem) (string, int, bool) {
		buf = buf[:len(x.symbol)+len(y.symbol)]
		copy(buf, x.symbol)
		copy(buf[len(x.symbol):], y.symbol)
		id, found := proc.pieces[string(buf)]
		if found {
			return proc.model.GetPieces()[id].GetPiece(), id, true
		}
		return "", 0, false
	}

	// suggestNewMergePair is called to potentially add a new mergeCandidate to
	// mergeQueue. The candidate is added if it's valid, both its parts are
	// allowed to merge, and it appears in the vocabulary.
	suggestNewMergePair := func(left, right int) {
		if left == -1 || right == -1 || symList[left].noMerge || symList[right].noMerge {
			return
		}

		mergedSymbol, id, ok := mergeSymbols(symList[left], symList[right])
		if !ok {
			return
		}
		mergeQueue.Insert(mergeCandidate{
			left:   left,
			right:  right,
			length: len(mergedSymbol),
			score:  proc.model.GetPieces()[id].GetScore(),
		})
	}

	// Seed the merge queue with all pairs of symbols from symList
	for i := 1; i < len(symList); i++ {
		suggestNewMergePair(i-1, i)
	}

	candidateIsDead := func(candidate mergeCandidate) bool {
		leftSymbol := symList[candidate.left].symbol
		rightSymbol := symList[candidate.right].symbol
		return leftSymbol == "" || rightSymbol == "" || len(leftSymbol)+len(rightSymbol) != candidate.length
	}

	// Main loop
	mergeQueueDead := 0
	for mergeQueue.Len() > 0 {
		candidate := mergeQueue.PopMax()
		leftSymbol := symList[candidate.left]
		rightSymbol := symList[candidate.right]

		// Make sure this candidate is not out of date. If one of its parts was
		// already merged with another symbol, just skip this candidate.
		if candidateIsDead(candidate) {
			mergeQueueDead--
			continue
		}

		// If there are lots more dead merge candidates than live ones, remove the dead.
		// The factor of 3 was determined empirically.
		if mergeQueueDead*3 > mergeQueue.Len() {
			mergeQueue.RemoveFunc(candidateIsDead)
			mergeQueueDead = 0
		}

		// Do the merge:
		// 1. Merge the concatenation of leftSymbol and rightSymbol into leftSymbol
		mergedSymbol, _, ok := mergeSymbols(leftSymbol, rightSymbol)
		if !ok {
			panic("failed to merge symbols")
		}
		symList[candidate.left].symbol = mergedSymbol
		nTokens--

		// 2. Update prev/next pointers
		symList[candidate.left].next = rightSymbol.next
		if rightSymbol.next >= 0 {
			symList[rightSymbol.next].prev = candidate.left
		}

		// 3. Mark the right element in the pair as outdated (it's been merged
		//    into the left one).
		symList[candidate.right].symbol = ""
		mergeQueueDead++

		// 4. Add merge suggestions for the newly merged symbol with its neighbors
		suggestNewMergePair(leftSymbol.prev, candidate.left)
		suggestNewMergePair(candidate.left, rightSymbol.next)
	}

	// Collect the final list of tokens from the remaining elements of symList.
	tokens := make([]Token, 0, nTokens)
	for i := 0; i >= 0; i = symList[i].next {
		symbol := symList[i].symbol
		id := proc.symbolToID(symbol)

		if id == proc.unknownID && proc.model.GetTrainerSpec().GetByteFallback() {
			// Decompose this symbol into bytes, and report each byte as a separate
			// token.
			for i := 0; i < len(symbol); i++ {
				tokens = append(tokens, proc.byte2Token[symbol[i]])
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
func (proc *Processor) symbolMatch(text string) (int, bool) {
	prefixLen := proc.userDefinedMatcher.FindPrefixLen(text)
	if prefixLen > 0 {
		return prefixLen, true
	}
	// Not found a user-defined prefix; get the length of next rune.
	_, rlen := utf8.DecodeRuneInString(text)
	return rlen, false
}

const (
	symbolBOS = "<bos>"
	symbolEOS = "<eos>"
	symbolUNK = "<unk>"
	symbolPAD = "<pad>"
)

// symbolToID finds the right ID for the given textual symbol, or returns
// proc.unknownID if the symbol is unknown.
func (proc *Processor) symbolToID(symbol string) int {
	if id, found := proc.reserved[symbol]; found {
		return id
	}
	if id, found := proc.pieces[symbol]; found {
		return id
	}
	return proc.unknownID
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

// Decode translates a list of IDs produced by [Encode] back into the string
// it represents.
func (proc *Processor) Decode(ids []int) string {
	var sb strings.Builder

	for i := 0; i < len(ids); {
		// Find a run of IDs that represent single bytes starting at i.
		nextNonByte := i
		for nextNonByte < len(ids) && proc.isByteID(ids[nextNonByte]) {
			nextNonByte++
		}
		numBytes := nextNonByte - i

		// Handle a run of numBytes IDs, by decoding them into utf8 runes.
		if numBytes > 0 {
			buf := make([]byte, 0, numBytes)
			for bi := i; bi < nextNonByte; bi++ {
				buf = append(buf, proc.idToByte[ids[bi]])
			}

			for len(buf) > 0 {
				// DecodeRune returns utf8.RuneError ('\uFFFD') for bad UTF8 encodings,
				// and this is exactly what SentencePiece is supposed to emit for them.
				// So we don't do any special handling for UTF8 decode errors here.
				r, size := utf8.DecodeRune(buf)
				sb.WriteRune(r)
				buf = buf[size:]
			}
		}

		if nextNonByte >= len(ids) {
			break
		}
		// Here nextNonByte is the index of an ID that's not a single byte.
		id := ids[nextNonByte]
		if proc.isControlID(id) {
			// Don't emit anything for control IDs
		} else if id == proc.unknownID {
			// Special "unk_surface" string for unknown IDs
			sb.WriteString(proc.model.GetTrainerSpec().GetUnkSurface())
		} else {
			piece := proc.model.GetPieces()[id].GetPiece()
			sb.WriteString(replaceSeparatorsBySpace(piece))
		}
		i = nextNonByte + 1
	}

	return sb.String()
}

// DecodeTokens is a convenience wrapper around [Decode], accepting a list of
// tokens as returned by [Encode]. It only uses the ID fields of tokens to
// decode the text.
func (proc *Processor) DecodeTokens(tokens []Token) string {
	ids := make([]int, len(tokens))
	for i, t := range tokens {
		ids[i] = t.ID
	}
	return proc.Decode(ids)
}

func (proc *Processor) isByteID(id int) bool {
	return proc.model.GetPieces()[id].GetType() == model.ModelProto_SentencePiece_BYTE
}

func (proc *Processor) isControlID(id int) bool {
	return proc.model.GetPieces()[id].GetType() == model.ModelProto_SentencePiece_CONTROL
}

// ModelInfo stores information about the model proto loaded by the processor.
type ModelInfo struct {
	VocabularySize        int
	BeginningOfSentenceID int
	EndOfSentenceID       int
	UnknownID             int
	PadID                 int
}

// ModelInfo returns information about the loaded proto model file.
func (proc *Processor) ModelInfo() *ModelInfo {
	getControlID := func(symbol string) int {
		if id := proc.symbolToID(symbol); proc.isControlID(id) {
			return id
		}
		return -1
	}

	return &ModelInfo{
		VocabularySize:        len(proc.model.GetPieces()),
		BeginningOfSentenceID: getControlID(symbolBOS),
		EndOfSentenceID:       getControlID(symbolEOS),
		PadID:                 getControlID(symbolPAD),
		UnknownID:             proc.unknownID,
	}
}
