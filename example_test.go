package sentencepiece_test

import (
	"fmt"
	"log"
	"os"

	"github.com/eliben/go-sentencepiece"
)

func ExampleEncode() {
	protoFile := os.Getenv("MODELPATH")
	if protoFile == "" {
		log.Println("Need MODELPATH env var to run example")
		return
	}

	proc, err := sentencepiece.NewProcessorFromPath(protoFile)
	if err != nil {
		log.Fatal(err)
	}

	text := "Encoding produces tokens that LLMs can learn and understand"
	tokens := proc.Encode(text)

	for _, token := range tokens {
		fmt.Println(token)
	}
}

// TODO: example for decode
