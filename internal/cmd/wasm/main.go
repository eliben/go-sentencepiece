//go:build js && wasm

// Main binary for exposing the go-sentencepiece functionality in the browser
// via WASM. The required functionality is exposed via the syscall/js interface.
// This module should only be built in js && wasm mode.
package main

import (
	_ "embed"
	"fmt"
	"log"
	"strings"
	"sync"
	"syscall/js"

	"github.com/eliben/go-sentencepiece"
)

//go:embed embed_data/tokenizer.model
var modelFileData string
var spm *sentencepiece.Processor

func main() {
	var once sync.Once
	once.Do(func() {
		var err error
		spm, err = sentencepiece.NewProcessor(strings.NewReader(modelFileData))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("processor loaded, vocab len=%v\n", spm.VocabularySize())
	})

	js.Global().Set("textToIDs", jsTextToIDs)
	js.Global().Set("textToPieces", jsTextToPieces)

	// For the Go code to be usable from JS, the main function has to run forever.
	<-make(chan bool)
}

var jsTextToIDs = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 {
		return "expected 1 argument: text to tokenize"
	}
	txt := args[0].String()
	tokens := spm.Encode(txt)

	jsTokens := js.Global().Get("Array").New()
	for _, t := range tokens {
		jsTokens.Call("push", js.ValueOf(t.ID))
	}
	return jsTokens
})

var jsTextToPieces = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 {
		return "expected 1 argument: text to tokenize"
	}
	txt := args[0].String()
	tokens := spm.Encode(txt)

	jsTokens := js.Global().Get("Array").New()
	for _, t := range tokens {
		jsTokens.Call("push", js.ValueOf(t.Text))
	}
	return jsTokens
})
